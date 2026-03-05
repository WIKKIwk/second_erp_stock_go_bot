package bot

import (
	"context"
	"errors"
	"testing"

	"erpnext_stock_telegram/internal/erpnext"
	"erpnext_stock_telegram/internal/store"
)

type fakeERP struct {
	authInfo erpnext.AuthInfo
	err      error
}

func (f *fakeERP) ValidateCredentials(_ context.Context, _, _, _ string) (erpnext.AuthInfo, error) {
	if f.err != nil {
		return erpnext.AuthInfo{}, f.err
	}
	return f.authInfo, nil
}

func TestServiceLoginFlowSuccess(t *testing.T) {
	sessions := NewSessionManager()
	creds := store.NewMemoryCredentialStore()
	erp := &fakeERP{authInfo: erpnext.AuthInfo{Username: "test@example.com", Roles: []string{"Stock User"}}}
	svc := NewService(sessions, creds, erp)

	chatID := int64(99)

	msgs := svc.HandleLoginCommand(chatID)
	if len(msgs) == 0 {
		t.Fatal("expected login command response")
	}

	msgs = svc.HandleText(context.Background(), chatID, "not-url")
	if msgs[0] == "" || msgs[0][:9] != "Noto'g'ri" {
		t.Fatalf("expected invalid URL message, got: %v", msgs)
	}

	msgs = svc.HandleText(context.Background(), chatID, "https://erp.example.com/")
	if len(msgs) != 1 || msgs[0] != "2/3: API Key kiriting." {
		t.Fatalf("unexpected response after URL: %v", msgs)
	}

	msgs = svc.HandleText(context.Background(), chatID, "my-key")
	if len(msgs) != 1 || msgs[0] != "3/3: API Secret kiriting." {
		t.Fatalf("unexpected response after API key: %v", msgs)
	}

	msgs = svc.HandleText(context.Background(), chatID, "my-secret")
	if len(msgs) != 1 || msgs[0] == "" {
		t.Fatalf("unexpected response after API secret: %v", msgs)
	}

	stored, ok := creds.Get(chatID)
	if !ok {
		t.Fatal("expected credentials to be saved")
	}
	if stored.BaseURL != "https://erp.example.com" {
		t.Fatalf("expected normalized URL, got: %q", stored.BaseURL)
	}
	if stored.Username != "test@example.com" {
		t.Fatalf("unexpected stored username: %q", stored.Username)
	}
}

func TestServiceLoginFlowFailure(t *testing.T) {
	sessions := NewSessionManager()
	creds := store.NewMemoryCredentialStore()
	erp := &fakeERP{err: errors.New("401 unauthorized")}
	svc := NewService(sessions, creds, erp)

	chatID := int64(7)
	svc.HandleLoginCommand(chatID)
	svc.HandleText(context.Background(), chatID, "https://erp.example.com")
	svc.HandleText(context.Background(), chatID, "my-key")
	msgs := svc.HandleText(context.Background(), chatID, "bad-secret")

	if len(msgs) < 1 || msgs[0] != "Kirish muvaffaqiyatsiz. URL/API Key/API Secret noto'g'ri bo'lishi mumkin." {
		t.Fatalf("unexpected failure message: %v", msgs)
	}
	if _, ok := creds.Get(chatID); ok {
		t.Fatal("credentials must not be saved on failed auth")
	}
}
