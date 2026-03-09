package mobileapi

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"

	"erpnext_stock_telegram/internal/erpnext"
	"erpnext_stock_telegram/internal/suplier"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidRole        = errors.New("invalid role")
	ErrUnauthorized       = errors.New("unauthorized")
)

type ERPClient interface {
	SearchSuppliers(ctx context.Context, baseURL, apiKey, apiSecret, query string, limit int) ([]erpnext.Supplier, error)
	SearchSupplierItems(ctx context.Context, baseURL, apiKey, apiSecret, supplier, query string, limit int) ([]erpnext.Item, error)
	ListPendingPurchaseReceipts(ctx context.Context, baseURL, apiKey, apiSecret string, limit int) ([]erpnext.PurchaseReceiptDraft, error)
	ListSupplierPurchaseReceipts(ctx context.Context, baseURL, apiKey, apiSecret, supplier string, limit int) ([]erpnext.PurchaseReceiptDraft, error)
	CreateDraftPurchaseReceipt(ctx context.Context, baseURL, apiKey, apiSecret string, input erpnext.CreatePurchaseReceiptInput) (erpnext.PurchaseReceiptDraft, error)
	ConfirmAndSubmitPurchaseReceipt(ctx context.Context, baseURL, apiKey, apiSecret, name string, acceptedQty float64) (erpnext.PurchaseReceiptSubmissionResult, error)
}

type ERPAuthenticator struct {
	erp              ERPClient
	baseURL          string
	apiKey           string
	apiSecret        string
	defaultWarehouse string
	supplierPrefix   string
	werkaPrefix      string
	werkaCode        string
	werkaPhone       string
	werkaName        string
}

func NewERPAuthenticator(
	erp ERPClient,
	baseURL string,
	apiKey string,
	apiSecret string,
	defaultWarehouse string,
	supplierPrefix string,
	werkaPrefix string,
	werkaCode string,
	werkaPhone string,
	werkaName string,
) *ERPAuthenticator {
	if strings.TrimSpace(supplierPrefix) == "" {
		supplierPrefix = "10"
	}
	if strings.TrimSpace(werkaPrefix) == "" {
		werkaPrefix = "20"
	}
	if strings.TrimSpace(werkaName) == "" {
		werkaName = "Werka"
	}

	return &ERPAuthenticator{
		erp:              erp,
		baseURL:          strings.TrimSpace(baseURL),
		apiKey:           strings.TrimSpace(apiKey),
		apiSecret:        strings.TrimSpace(apiSecret),
		defaultWarehouse: strings.TrimSpace(defaultWarehouse),
		supplierPrefix:   strings.TrimSpace(supplierPrefix),
		werkaPrefix:      strings.TrimSpace(werkaPrefix),
		werkaCode:        strings.TrimSpace(werkaCode),
		werkaPhone:       strings.TrimSpace(werkaPhone),
		werkaName:        strings.TrimSpace(werkaName),
	}
}

func (a *ERPAuthenticator) Login(ctx context.Context, phone, code string) (Principal, error) {
	role, err := a.inferRole(code)
	if err != nil {
		return Principal{}, err
	}

	normalizedPhone, err := suplier.NormalizePhone(phone)
	if err != nil {
		return Principal{}, ErrInvalidCredentials
	}

	switch role {
	case RoleSupplier:
		suppliers, err := a.erp.SearchSuppliers(ctx, a.baseURL, a.apiKey, a.apiSecret, "", 500)
		if err != nil {
			return Principal{}, err
		}
		for _, item := range suppliers {
			creds, err := suplier.GenerateAccessCredentials(suplier.Supplier{
				Ref:   item.ID,
				Name:  item.Name,
				Phone: item.Phone,
			})
			if err != nil {
				continue
			}
			if strings.TrimSpace(code) == creds.Code &&
				strings.TrimSpace(item.Phone) != "" &&
				strings.EqualFold(strings.TrimSpace(item.Phone), normalizedPhone) {
				return Principal{
					Role:        RoleSupplier,
					DisplayName: item.Name,
					Ref:         item.ID,
					Phone:       item.Phone,
				}, nil
			}
		}
		return Principal{}, ErrInvalidCredentials

	case RoleWerka:
		if code == a.werkaCode && code != "" {
			if a.werkaPhone != "" {
				expectedWerkaPhone, err := suplier.NormalizePhone(a.werkaPhone)
				if err != nil {
					return Principal{}, ErrInvalidCredentials
				}
				if expectedWerkaPhone != normalizedPhone {
					return Principal{}, ErrInvalidCredentials
				}
			}
			return Principal{
				Role:        RoleWerka,
				DisplayName: a.werkaName,
				Ref:         "werka",
			}, nil
		}
		return Principal{}, ErrInvalidCredentials

	default:
		return Principal{}, ErrInvalidRole
	}
}

func (a *ERPAuthenticator) inferRole(code string) (PrincipalRole, error) {
	trimmed := strings.TrimSpace(code)
	switch {
	case strings.HasPrefix(trimmed, a.supplierPrefix):
		return RoleSupplier, nil
	case strings.HasPrefix(trimmed, a.werkaPrefix):
		return RoleWerka, nil
	default:
		return "", ErrInvalidRole
	}
}

func (a *ERPAuthenticator) SupplierHistory(ctx context.Context, principal Principal, limit int) ([]DispatchRecord, error) {
	items, err := a.erp.ListSupplierPurchaseReceipts(ctx, a.baseURL, a.apiKey, a.apiSecret, principal.Ref, limit)
	if err != nil {
		return nil, err
	}

	result := make([]DispatchRecord, 0, len(items))
	for _, item := range items {
		sentQty := item.Qty
		if markerQty, ok := erpnext.ParseTelegramReceiptMarkerQty(item.SupplierDeliveryNote); ok && markerQty > sentQty {
			sentQty = markerQty
		}
		status, acceptedQty := mapDispatchStatus(item, sentQty)
		result = append(result, DispatchRecord{
			ID:           item.Name,
			SupplierName: principal.DisplayName,
			ItemCode:     item.ItemCode,
			ItemName:     item.ItemName,
			UOM:          item.UOM,
			SentQty:      sentQty,
			AcceptedQty:  acceptedQty,
			Status:       status,
			CreatedLabel: item.PostingDate,
		})
	}
	return result, nil
}

func (a *ERPAuthenticator) WerkaPending(ctx context.Context, limit int) ([]DispatchRecord, error) {
	items, err := a.erp.ListPendingPurchaseReceipts(ctx, a.baseURL, a.apiKey, a.apiSecret, limit)
	if err != nil {
		return nil, err
	}

	result := make([]DispatchRecord, 0, len(items))
	for _, item := range items {
		sentQty := item.Qty
		if markerQty, ok := erpnext.ParseTelegramReceiptMarkerQty(item.SupplierDeliveryNote); ok && markerQty > sentQty {
			sentQty = markerQty
		}
		result = append(result, DispatchRecord{
			ID:           item.Name,
			SupplierName: item.SupplierName,
			ItemCode:     item.ItemCode,
			ItemName:     item.ItemName,
			UOM:          item.UOM,
			SentQty:      sentQty,
			AcceptedQty:  0,
			Status:       "pending",
			CreatedLabel: item.PostingDate,
		})
	}
	return result, nil
}

func (a *ERPAuthenticator) SupplierItems(ctx context.Context, principal Principal, query string, limit int) ([]SupplierItem, error) {
	items, err := a.erp.SearchSupplierItems(ctx, a.baseURL, a.apiKey, a.apiSecret, principal.Ref, query, limit)
	if err != nil {
		return nil, err
	}

	result := make([]SupplierItem, 0, len(items))
	for _, item := range items {
		result = append(result, SupplierItem{
			Code:      item.Code,
			Name:      item.Name,
			UOM:       item.UOM,
			Warehouse: a.defaultWarehouse,
		})
	}
	return result, nil
}

func (a *ERPAuthenticator) CreateDispatch(ctx context.Context, principal Principal, itemCode string, qty float64) (DispatchRecord, error) {
	draft, err := a.erp.CreateDraftPurchaseReceipt(ctx, a.baseURL, a.apiKey, a.apiSecret, erpnext.CreatePurchaseReceiptInput{
		Supplier:      principal.Ref,
		SupplierPhone: principal.Phone,
		ItemCode:      strings.TrimSpace(itemCode),
		Qty:           qty,
		Warehouse:     a.defaultWarehouse,
	})
	if err != nil {
		return DispatchRecord{}, err
	}

	return DispatchRecord{
		ID:           draft.Name,
		SupplierName: principal.DisplayName,
		ItemCode:     draft.ItemCode,
		ItemName:     draft.ItemName,
		UOM:          draft.UOM,
		SentQty:      draft.Qty,
		AcceptedQty:  0,
		Status:       "pending",
		CreatedLabel: draft.PostingDate,
	}, nil
}

func (a *ERPAuthenticator) ConfirmReceipt(ctx context.Context, receiptID string, acceptedQty float64) (DispatchRecord, error) {
	result, err := a.erp.ConfirmAndSubmitPurchaseReceipt(ctx, a.baseURL, a.apiKey, a.apiSecret, strings.TrimSpace(receiptID), acceptedQty)
	if err != nil {
		return DispatchRecord{}, err
	}

	return DispatchRecord{
		ID:           result.Name,
		SupplierName: result.Supplier,
		ItemCode:     result.ItemCode,
		ItemName:     result.ItemCode,
		UOM:          result.UOM,
		SentQty:      result.SentQty,
		AcceptedQty:  result.AcceptedQty,
		Status:       dispatchStatusFromQuantities(result.SentQty, result.AcceptedQty),
		CreatedLabel: result.Name,
	}, nil
}

type SessionManager struct {
	mu       sync.RWMutex
	sessions map[string]Principal
}

func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[string]Principal),
	}
}

func (m *SessionManager) Create(principal Principal) (string, error) {
	tokenBytes := make([]byte, 24)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(tokenBytes)

	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[token] = principal
	return token, nil
}

func (m *SessionManager) Get(token string) (Principal, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	principal, ok := m.sessions[token]
	return principal, ok
}

func (m *SessionManager) Delete(token string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, token)
}

func requireRole(principal Principal, role PrincipalRole) error {
	if principal.Role != role {
		return fmt.Errorf("role %s required", role)
	}
	return nil
}

func mapDispatchStatus(item erpnext.PurchaseReceiptDraft, sentQty float64) (string, float64) {
	if item.DocStatus == 2 || strings.EqualFold(strings.TrimSpace(item.Status), "Cancelled") {
		return "cancelled", 0
	}
	if item.DocStatus == 1 {
		return dispatchStatusFromQuantities(sentQty, item.Qty), item.Qty
	}
	if strings.EqualFold(strings.TrimSpace(item.Status), "Draft") {
		return "draft", 0
	}
	return "pending", 0
}

func dispatchStatusFromQuantities(sentQty, acceptedQty float64) string {
	switch {
	case acceptedQty <= 0:
		return "rejected"
	case sentQty > 0 && acceptedQty < sentQty:
		return "partial"
	default:
		return "accepted"
	}
}
