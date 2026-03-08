package suplier

import "time"

type SupplierAuth struct {
	Phone          string
	PasswordHash   string
	RegisteredAt   time.Time
	LastLoginAt    time.Time
	FailedAttempts int
	LockedUntil    time.Time
	TelegramUserID int64
}
