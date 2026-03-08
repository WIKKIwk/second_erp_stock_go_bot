package suplier

import (
	"context"
	"testing"
	"time"
)

func TestAuthFileRepositoryUpsertAndFindByPhone(t *testing.T) {
	repository := NewAuthFileRepository(t.TempDir() + "/supplier_auth.fb")

	record := SupplierAuth{
		Phone:          "+998901234567",
		PasswordHash:   "hash",
		RegisteredAt:   time.Unix(1_700_000_000, 0).UTC(),
		LastLoginAt:    time.Unix(1_700_000_100, 0).UTC(),
		FailedAttempts: 2,
		LockedUntil:    time.Unix(1_700_000_200, 0).UTC(),
		TelegramUserID: 123,
	}
	if err := repository.Upsert(context.Background(), record); err != nil {
		t.Fatalf("Upsert returned error: %v", err)
	}

	got, ok, err := repository.FindByPhone(context.Background(), "+998901234567")
	if err != nil {
		t.Fatalf("FindByPhone returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected auth record to exist")
	}
	if got.Phone != record.Phone || got.PasswordHash != record.PasswordHash || got.TelegramUserID != record.TelegramUserID {
		t.Fatalf("unexpected auth record: %+v", got)
	}
	if got.FailedAttempts != record.FailedAttempts {
		t.Fatalf("unexpected failed attempts: %+v", got)
	}
	if got.RegisteredAt.Unix() != record.RegisteredAt.Unix() || got.LastLoginAt.Unix() != record.LastLoginAt.Unix() || got.LockedUntil.Unix() != record.LockedUntil.Unix() {
		t.Fatalf("unexpected timestamps: %+v", got)
	}
}
