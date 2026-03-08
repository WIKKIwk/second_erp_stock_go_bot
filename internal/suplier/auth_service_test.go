package suplier

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeAuthRepository struct {
	items map[string]SupplierAuth
}

func (f *fakeAuthRepository) FindByPhone(_ context.Context, phone string) (SupplierAuth, bool, error) {
	if f.items == nil {
		return SupplierAuth{}, false, nil
	}
	item, ok := f.items[phone]
	return item, ok, nil
}

func (f *fakeAuthRepository) List(_ context.Context) ([]SupplierAuth, error) {
	items := make([]SupplierAuth, 0, len(f.items))
	for _, item := range f.items {
		items = append(items, item)
	}
	return items, nil
}

func (f *fakeAuthRepository) Upsert(_ context.Context, auth SupplierAuth) error {
	if f.items == nil {
		f.items = map[string]SupplierAuth{}
	}
	f.items[auth.Phone] = auth
	return nil
}

func TestAuthServiceRegisterAndAuthenticate(t *testing.T) {
	repository := &fakeAuthRepository{}
	service := NewAuthService(repository)
	service.now = func() time.Time {
		return time.Unix(1_700_000_000, 0).UTC()
	}

	auth, err := service.Register(context.Background(), "998901234567", 123, "abc12345")
	if err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	if auth.Phone != "+998901234567" {
		t.Fatalf("unexpected auth record: %+v", auth)
	}
	if auth.PasswordHash == "" || auth.PasswordHash == "abc12345" {
		t.Fatalf("expected hashed password, got %+v", auth)
	}

	auth, err = service.Authenticate(context.Background(), "+998901234567", 123, "abc12345")
	if err != nil {
		t.Fatalf("Authenticate returned error: %v", err)
	}
	if auth.TelegramUserID != 123 {
		t.Fatalf("expected telegram user ID to be updated, got %+v", auth)
	}
}

func TestAuthServiceAuthenticateLocksAfterTooManyFailures(t *testing.T) {
	repository := &fakeAuthRepository{}
	service := NewAuthService(repository)
	now := time.Unix(1_700_000_000, 0).UTC()
	service.now = func() time.Time { return now }

	if _, err := service.Register(context.Background(), "+998901234567", 0, "abc12345"); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	for i := 0; i < supplierAuthMaxFailedAttempts-1; i++ {
		_, err := service.Authenticate(context.Background(), "+998901234567", 0, "wrong")
		if !errors.Is(err, ErrSupplierAuthInvalidPassword) {
			t.Fatalf("expected invalid password error on attempt %d, got %v", i+1, err)
		}
	}

	_, err := service.Authenticate(context.Background(), "+998901234567", 0, "wrong")
	if !errors.Is(err, ErrSupplierAuthLocked) {
		t.Fatalf("expected locked error, got %v", err)
	}

	record, ok, err := repository.FindByPhone(context.Background(), "+998901234567")
	if err != nil {
		t.Fatalf("FindByPhone returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected auth record to exist")
	}
	if record.LockedUntil.IsZero() {
		t.Fatalf("expected locked_until to be set, got %+v", record)
	}
}
