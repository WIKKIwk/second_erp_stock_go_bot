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
	authInfo          erpnext.AuthInfo
	err               error
	supplierItems     []erpnext.Item
	suppliers         []erpnext.Supplier
	ensureSupplierIn  erpnext.CreateSupplierInput
	createDraftInput  erpnext.CreatePurchaseReceiptInput
	createDraftResult erpnext.PurchaseReceiptDraft
	createDraftErr    error
	pendingReceipts   []erpnext.PurchaseReceiptDraft
	pendingErr        error
	receipt           erpnext.PurchaseReceiptDraft
	receiptErr        error
	submitResult      erpnext.PurchaseReceiptSubmissionResult
	submitErr         error
	confirmName       string
	confirmQty        float64
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

func (f *fakeERP) SearchSupplierItems(_ context.Context, _, _, _, _, _ string, _ int) ([]erpnext.Item, error) {
	return f.supplierItems, nil
}

func (f *fakeERP) SearchSuppliers(_ context.Context, _, _, _, _ string, _ int) ([]erpnext.Supplier, error) {
	return f.suppliers, nil
}

func (f *fakeERP) EnsureSupplier(_ context.Context, _, _, _ string, input erpnext.CreateSupplierInput) (erpnext.Supplier, error) {
	f.ensureSupplierIn = input
	return erpnext.Supplier{
		ID:    "SUP-001",
		Name:  input.Name,
		Phone: input.Phone,
	}, nil
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

func (f *fakeERP) CreateDraftPurchaseReceipt(_ context.Context, _, _, _ string, input erpnext.CreatePurchaseReceiptInput) (erpnext.PurchaseReceiptDraft, error) {
	f.createDraftInput = input
	if f.createDraftErr != nil {
		return erpnext.PurchaseReceiptDraft{}, f.createDraftErr
	}
	return f.createDraftResult, nil
}

func (f *fakeERP) ListPendingPurchaseReceipts(_ context.Context, _, _, _ string, _ int) ([]erpnext.PurchaseReceiptDraft, error) {
	if f.pendingErr != nil {
		return nil, f.pendingErr
	}
	return f.pendingReceipts, nil
}

func (f *fakeERP) GetPurchaseReceipt(_ context.Context, _, _, _, _ string) (erpnext.PurchaseReceiptDraft, error) {
	if f.receiptErr != nil {
		return erpnext.PurchaseReceiptDraft{}, f.receiptErr
	}
	return f.receipt, nil
}

func (f *fakeERP) ConfirmAndSubmitPurchaseReceipt(_ context.Context, _, _, _, name string, qty, _ float64, _ string) (erpnext.PurchaseReceiptSubmissionResult, error) {
	f.confirmName = name
	f.confirmQty = qty
	if f.submitErr != nil {
		return erpnext.PurchaseReceiptSubmissionResult{}, f.submitErr
	}
	return f.submitResult, nil
}

func TestServiceLoginFlowSuccess(t *testing.T) {
	sessions := NewSessionManager()
	creds := store.NewMemoryCredentialStore()
	erp := &fakeERP{authInfo: erpnext.AuthInfo{Username: "test@example.com", Roles: []string{"Stock User"}}}
	svc := NewService(sessions, creds, erp, nil, nil, nil, "", "", "", "", "", "", "", "", "", "", "", 0, nil)

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
	svc := NewService(sessions, creds, erp, nil, nil, nil, "", "", "", "", "", "", "", "", "", "", "", 0, nil)

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
	svc := NewService(sessions, creds, erp, nil, nil, nil, "", "", "", "", "", "", "", "", "", "", "", 0, nil)

	msg := svc.HandleText(context.Background(), 123, "https://erp.example.com")
	if msg != "Iltimos, avval /login buyrug'ini yuboring." {
		t.Fatalf("unexpected message: %q", msg)
	}
}

func TestServiceFindSupplierByPhoneFallsBackToERP(t *testing.T) {
	sessions := NewSessionManager()
	creds := store.NewMemoryCredentialStore()
	creds.Save(55, store.Credentials{
		BaseURL:   "http://localhost:8000",
		APIKey:    "key",
		APISecret: "secret",
	})
	erp := &fakeERP{
		suppliers: []erpnext.Supplier{
			{ID: "SUP-001", Name: "Stocker", Phone: "+998901390311"},
		},
	}
	svc := NewService(sessions, creds, erp, nil, nil, nil, "", "", "", "", "", "", "", "", "", "", "", 0, nil)

	supplier, ok, err := svc.FindSupplierByPhone(context.Background(), 55, "+998901390311")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected supplier to be found")
	}
	if supplier.Name != "Stocker" || supplier.Phone != "+998901390311" || supplier.Ref != "SUP-001" {
		t.Fatalf("unexpected supplier: %+v", supplier)
	}
}
