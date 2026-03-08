package suplier

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrSupplierAuthNotFound          = errors.New("supplier auth not found")
	ErrSupplierAuthAlreadyRegistered = errors.New("supplier auth already registered")
	ErrSupplierAuthInvalidPassword   = errors.New("supplier auth invalid password")
	ErrSupplierAuthLocked            = errors.New("supplier auth locked")
)

const (
	supplierAuthMaxFailedAttempts = 5
	supplierAuthLockDuration      = 15 * time.Minute
)

type AuthService struct {
	repository AuthRepository
	now        func() time.Time
}

func NewAuthService(repository AuthRepository) *AuthService {
	return &AuthService{
		repository: repository,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func (s *AuthService) FindByPhone(ctx context.Context, phone string) (SupplierAuth, bool, error) {
	if s.repository == nil {
		return SupplierAuth{}, false, fmt.Errorf("supplier auth repository is not configured")
	}

	normalizedPhone, err := NormalizePhone(phone)
	if err != nil {
		return SupplierAuth{}, false, err
	}
	return s.repository.FindByPhone(ctx, normalizedPhone)
}

func (s *AuthService) Register(ctx context.Context, phone string, telegramUserID int64, password string) (SupplierAuth, error) {
	if s.repository == nil {
		return SupplierAuth{}, fmt.Errorf("supplier auth repository is not configured")
	}

	normalizedPhone, err := NormalizePhone(phone)
	if err != nil {
		return SupplierAuth{}, err
	}
	if strings.TrimSpace(password) == "" {
		return SupplierAuth{}, fmt.Errorf("password is required")
	}

	existing, found, err := s.repository.FindByPhone(ctx, normalizedPhone)
	if err != nil {
		return SupplierAuth{}, err
	}
	if found && strings.TrimSpace(existing.PasswordHash) != "" {
		return SupplierAuth{}, ErrSupplierAuthAlreadyRegistered
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return SupplierAuth{}, err
	}

	now := s.now()
	auth := SupplierAuth{
		Phone:          normalizedPhone,
		PasswordHash:   string(hash),
		RegisteredAt:   now,
		LastLoginAt:    now,
		FailedAttempts: 0,
		LockedUntil:    time.Time{},
		TelegramUserID: telegramUserID,
	}
	if found && !existing.RegisteredAt.IsZero() {
		auth.RegisteredAt = existing.RegisteredAt
	}

	if err := s.repository.Upsert(ctx, auth); err != nil {
		return SupplierAuth{}, err
	}
	return auth, nil
}

func (s *AuthService) Authenticate(ctx context.Context, phone string, telegramUserID int64, password string) (SupplierAuth, error) {
	if s.repository == nil {
		return SupplierAuth{}, fmt.Errorf("supplier auth repository is not configured")
	}

	normalizedPhone, err := NormalizePhone(phone)
	if err != nil {
		return SupplierAuth{}, err
	}

	auth, found, err := s.repository.FindByPhone(ctx, normalizedPhone)
	if err != nil {
		return SupplierAuth{}, err
	}
	if !found || strings.TrimSpace(auth.PasswordHash) == "" {
		return SupplierAuth{}, ErrSupplierAuthNotFound
	}

	now := s.now()
	if !auth.LockedUntil.IsZero() && now.Before(auth.LockedUntil) {
		return SupplierAuth{}, ErrSupplierAuthLocked
	}

	if err := bcrypt.CompareHashAndPassword([]byte(auth.PasswordHash), []byte(password)); err != nil {
		auth.FailedAttempts++
		if auth.FailedAttempts >= supplierAuthMaxFailedAttempts {
			auth.LockedUntil = now.Add(supplierAuthLockDuration)
		}
		if upsertErr := s.repository.Upsert(ctx, auth); upsertErr != nil {
			return SupplierAuth{}, upsertErr
		}
		if !auth.LockedUntil.IsZero() && now.Before(auth.LockedUntil) {
			return SupplierAuth{}, ErrSupplierAuthLocked
		}
		return SupplierAuth{}, ErrSupplierAuthInvalidPassword
	}

	auth.LastLoginAt = now
	auth.FailedAttempts = 0
	auth.LockedUntil = time.Time{}
	if telegramUserID != 0 {
		auth.TelegramUserID = telegramUserID
	}
	if err := s.repository.Upsert(ctx, auth); err != nil {
		return SupplierAuth{}, err
	}
	return auth, nil
}
