package mobileapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"erpnext_stock_telegram/internal/erpnext"
	"erpnext_stock_telegram/internal/suplier"
)

type fakeERPClient struct {
	suppliers []erpnext.Supplier
}

func (f *fakeERPClient) SearchSuppliers(_ context.Context, _, _, _, _ string, _ int) ([]erpnext.Supplier, error) {
	return f.suppliers, nil
}

func (f *fakeERPClient) SearchSupplierItems(_ context.Context, _, _, _, _, _ string, _ int) ([]erpnext.Item, error) {
	return nil, nil
}

func (f *fakeERPClient) ListPendingPurchaseReceipts(_ context.Context, _, _, _ string, _ int) ([]erpnext.PurchaseReceiptDraft, error) {
	return nil, nil
}

func (f *fakeERPClient) ListSupplierPurchaseReceipts(_ context.Context, _, _, _, _ string, _ int) ([]erpnext.PurchaseReceiptDraft, error) {
	return nil, nil
}

func (f *fakeERPClient) CreateDraftPurchaseReceipt(_ context.Context, _, _, _ string, _ erpnext.CreatePurchaseReceiptInput) (erpnext.PurchaseReceiptDraft, error) {
	return erpnext.PurchaseReceiptDraft{}, nil
}

func (f *fakeERPClient) ConfirmAndSubmitPurchaseReceipt(_ context.Context, _, _, _, _ string, _ float64) (erpnext.PurchaseReceiptSubmissionResult, error) {
	return erpnext.PurchaseReceiptSubmissionResult{}, nil
}

func TestServerLoginAndMeFlow(t *testing.T) {
	creds, err := suplier.GenerateAccessCredentials(suplier.Supplier{
		Ref:   "SUP-001",
		Name:  "Abdulloh",
		Phone: "+998901234567",
	})
	if err != nil {
		t.Fatalf("failed to generate access credentials: %v", err)
	}

	server := NewServer(NewERPAuthenticator(
		&fakeERPClient{
			suppliers: []erpnext.Supplier{
				{ID: "SUP-001", Name: "Abdulloh", Phone: "+998901234567"},
			},
		},
		"http://localhost:8000",
		"key",
		"secret",
		"Stores - CH",
		"10",
		"20",
		"20WERKA0001",
		"+998901111111",
		"Werka",
	))
	ts := httptest.NewServer(server.Handler())
	defer ts.Close()

	body, _ := json.Marshal(LoginRequest{
		Phone:  "+998901234567",
		Code:   creds.Code,
	})
	resp, err := http.Post(ts.URL+"/v1/mobile/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected login status: %d", resp.StatusCode)
	}

	var loginResp LoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		t.Fatalf("failed to decode login response: %v", err)
	}
	if loginResp.Token == "" {
		t.Fatal("expected token")
	}
	if loginResp.Profile.Role != RoleSupplier || loginResp.Profile.DisplayName != "Abdulloh" {
		t.Fatalf("unexpected profile: %+v", loginResp.Profile)
	}

	meReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/v1/mobile/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+loginResp.Token)
	meResp, err := http.DefaultClient.Do(meReq)
	if err != nil {
		t.Fatalf("me request failed: %v", err)
	}
	defer meResp.Body.Close()

	if meResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected me status: %d", meResp.StatusCode)
	}
}

func TestServerLogoutInvalidatesSession(t *testing.T) {
	server := NewServer(NewERPAuthenticator(
		&fakeERPClient{},
		"http://localhost:8000",
		"key",
		"secret",
		"Stores - CH",
		"10",
		"20",
		"20WERKA0001",
		"+998901111111",
		"Werka",
	))
	token, err := server.sessions.Create(Principal{Role: RoleSupplier, DisplayName: "Abdulloh"})
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	logoutReq := httptest.NewRequest(http.MethodPost, "/v1/mobile/auth/logout", nil)
	logoutReq.Header.Set("Authorization", "Bearer "+token)
	logoutResp := httptest.NewRecorder()
	server.Handler().ServeHTTP(logoutResp, logoutReq)
	if logoutResp.Code != http.StatusOK {
		t.Fatalf("unexpected logout status: %d", logoutResp.Code)
	}

	meReq := httptest.NewRequest(http.MethodGet, "/v1/mobile/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+token)
	meResp := httptest.NewRecorder()
	server.Handler().ServeHTTP(meResp, meReq)
	if meResp.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized after logout, got %d", meResp.Code)
	}
}
