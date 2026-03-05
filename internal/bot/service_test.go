package bot

import (
	"context"
	"errors"
	"strings"
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

func (f *fakeERP) SearchItems(_ context.Context, _, _, _, _ string, _ int) ([]erpnext.Item, error) {
	return nil, nil
}

func (f *fakeERP) SearchWarehouses(_ context.Context, _, _, _, _ string, _ int) ([]erpnext.Warehouse, error) {
	return nil, nil
}

func (f *fakeERP) SearchUOMs(_ context.Context, _, _, _, _ string, _ int) ([]erpnext.UOM, error) {
	return nil, nil
}

func (f *fakeERP) CreateAndSubmitStockEntry(_ context.Context, _, _, _ string, _ erpnext.CreateStockEntryInput) (erpnext.StockEntryResult, error) {
	return erpnext.StockEntryResult{}, nil
}

func TestServiceLoginFlowSuccess(t *testing.T) {
	sessions := NewSessionManager()
	creds := store.NewMemoryCredentialStore()
	erp := &fakeERP{authInfo: erpnext.AuthInfo{Username: "test@example.com", Roles: []string{"Stock User"}}}
	svc := NewService(sessions, creds, erp, "", "", "", "")

	chatID := int64(99)

	msg := svc.HandleLoginCommand(chatID)
	if msg == "" {
		t.Fatal("expected login command response")
	}

	msg = svc.HandleText(context.Background(), chatID, "not-url")
	if !strings.HasPrefix(msg, "Noto'g'ri") {
		t.Fatalf("expected invalid URL message, got: %q", msg)
	}

	msg = svc.HandleText(context.Background(), chatID, "https://erp.example.com/")
	if msg != "2/3: API Key kiriting." {
		t.Fatalf("unexpected response after URL: %q", msg)
	}

	msg = svc.HandleText(context.Background(), chatID, "my-key")
	if msg != "3/3: API Secret kiriting." {
		t.Fatalf("unexpected response after API key: %q", msg)
	}

	msg = svc.HandleText(context.Background(), chatID, "my-secret")
	if msg == "" {
		t.Fatalf("unexpected response after API secret: %q", msg)
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
	svc := NewService(sessions, creds, erp, "", "", "", "")

	chatID := int64(7)
	svc.HandleLoginCommand(chatID)
	svc.HandleText(context.Background(), chatID, "https://erp.example.com")
	svc.HandleText(context.Background(), chatID, "my-key")
	msg := svc.HandleText(context.Background(), chatID, "bad-secret")

	wantPrefix := "Kirish muvaffaqiyatsiz. URL/API Key/API Secret noto'g'ri bo'lishi mumkin."
	if len(msg) < len(wantPrefix) || msg[:len(wantPrefix)] != wantPrefix {
		t.Fatalf("unexpected failure message: %q", msg)
	}
	if _, ok := creds.Get(chatID); ok {
		t.Fatal("credentials must not be saved on failed auth")
	}
}

func TestServiceHandleTextRequiresLogin(t *testing.T) {
	sessions := NewSessionManager()
	creds := store.NewMemoryCredentialStore()
	erp := &fakeERP{}
	svc := NewService(sessions, creds, erp, "", "", "", "")

	msg := svc.HandleText(context.Background(), 123, "https://erp.example.com")
	if msg != "Iltimos, avval /login buyrug'ini yuboring." {
		t.Fatalf("unexpected message: %q", msg)
	}
}
