package bot

import (
	"testing"

	"erpnext_stock_telegram/internal/store"
)

func TestMatchPrivilegedContactIgnoresPhoneFormatting(t *testing.T) {
	service := NewService(
		NewSessionManager(),
		store.NewMemoryCredentialStore(),
		&fakeERP{},
		nil,
		nil,
		"",
		"",
		"",
		"Kg",
		"",
		"",
		"",
		"+998 88 817 01 31",
		"Admin",
		"998(71)200-00-00",
		"Werka",
		nil,
	)

	role, name, ok := service.MatchPrivilegedContact("+998888170131")
	if !ok || role != UserRoleAdmin || name != "Admin" {
		t.Fatalf("expected admin match, got role=%q name=%q ok=%v", role, name, ok)
	}

	role, name, ok = service.MatchPrivilegedContact("+998 71 200 00 00")
	if !ok || role != UserRoleWerka || name != "Werka" {
		t.Fatalf("expected werka match, got role=%q name=%q ok=%v", role, name, ok)
	}
}
