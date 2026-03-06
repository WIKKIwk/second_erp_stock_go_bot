package erpnext

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestValidateCredentialsSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "token key:secret" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		switch r.URL.Path {
		case "/api/method/frappe.auth.get_logged_user":
			_, _ = w.Write([]byte(`{"message":"user@example.com"}`))
		case "/api/method/frappe.core.doctype.user.user.get_roles":
			_, _ = w.Write([]byte(`{"message":["Stock User","Material Manager"]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(&http.Client{Timeout: 3 * time.Second})
	result, err := client.ValidateCredentials(context.Background(), server.URL, "key", "secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Username != "user@example.com" {
		t.Fatalf("unexpected username: %q", result.Username)
	}
	if len(result.Roles) != 2 {
		t.Fatalf("expected 2 roles, got: %v", result.Roles)
	}
}

func TestValidateCredentialsFallbackRoles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/method/frappe.auth.get_logged_user":
			_, _ = w.Write([]byte(`{"message":"user@example.com"}`))
		case "/api/method/frappe.core.doctype.user.user.get_roles":
			http.NotFound(w, r)
		case "/api/resource/User/user@example.com":
			_, _ = w.Write([]byte(`{"data":{"roles":[{"role":"Stock User"}]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(&http.Client{Timeout: 3 * time.Second})
	result, err := client.ValidateCredentials(context.Background(), server.URL, "key", "secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Roles) != 1 || result.Roles[0] != "Stock User" {
		t.Fatalf("unexpected roles: %v", result.Roles)
	}
}

func TestValidateCredentialsInvalidAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "invalid token", http.StatusUnauthorized)
	}))
	defer server.Close()

	client := NewClient(&http.Client{Timeout: 3 * time.Second})
	_, err := client.ValidateCredentials(context.Background(), server.URL, "bad", "bad")
	if err == nil {
		t.Fatal("expected error for invalid auth")
	}
}

func TestSearchItems(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/resource/Item" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"data":[{"name":"ITEM-001","item_name":"Rice","stock_uom":"Kg"}]}`))
	}))
	defer server.Close()

	client := NewClient(&http.Client{Timeout: 3 * time.Second})
	items, err := client.SearchItems(context.Background(), server.URL, "key", "secret", "ri", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Code != "ITEM-001" || items[0].UOM != "Kg" {
		t.Fatalf("unexpected item: %+v", items[0])
	}
}

func TestCreateAndSubmitStockEntry(t *testing.T) {
	var createPayload map[string]interface{}
	var submitPayload map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/resource/Stock Entry/") || strings.HasPrefix(r.URL.EscapedPath(), "/api/resource/Stock%20Entry/") {
			if r.Method != http.MethodGet {
				http.Error(w, "bad method", http.StatusMethodNotAllowed)
				return
			}
			_, _ = w.Write([]byte(`{"data":{"doctype":"Stock Entry","name":"STE-0001","modified":"2026-03-05 10:00:00.000000"}}`))
			return
		}

		switch r.URL.Path {
		case "/api/resource/Stock Entry":
			if r.Method != http.MethodPost {
				http.Error(w, "bad method", http.StatusMethodNotAllowed)
				return
			}
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &createPayload)
			_, _ = w.Write([]byte(`{"data":{"name":"STE-0001"}}`))
		case "/api/method/frappe.client.submit":
			if r.Method != http.MethodPost {
				http.Error(w, "bad method", http.StatusMethodNotAllowed)
				return
			}
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &submitPayload)
			_, _ = w.Write([]byte(`{"message":{"name":"STE-0001","docstatus":1}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(&http.Client{Timeout: 3 * time.Second})
	result, err := client.CreateAndSubmitStockEntry(
		context.Background(),
		server.URL,
		"key",
		"secret",
		CreateStockEntryInput{
			EntryType:       "Material Receipt",
			ItemCode:        "ITEM-001",
			Qty:             5,
			UOM:             "Kg",
			TargetWarehouse: "Stores - CH",
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Name != "STE-0001" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if createPayload["stock_entry_type"] != "Material Receipt" {
		t.Fatalf("unexpected create payload: %+v", createPayload)
	}
	if submitPayload["doc"] == nil {
		t.Fatalf("unexpected submit payload: %+v", submitPayload)
	}
}

func TestSearchWarehousesAndUOMs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/method/frappe.desk.search.search_link":
			doctype := r.URL.Query().Get("doctype")
			if doctype == "Warehouse" {
				_, _ = w.Write([]byte(`{"message":[{"value":"Stores - CH"}]}`))
				return
			}
			if doctype == "UOM" {
				_, _ = w.Write([]byte(`{"message":[{"value":"Kg"},{"value":"Nos"}]}`))
				return
			}
			http.NotFound(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(&http.Client{Timeout: 3 * time.Second})

	warehouses, err := client.SearchWarehouses(context.Background(), server.URL, "key", "secret", "store", 10)
	if err != nil {
		t.Fatalf("unexpected warehouse error: %v", err)
	}
	if len(warehouses) != 1 || warehouses[0].Name != "Stores - CH" {
		t.Fatalf("unexpected warehouses: %+v", warehouses)
	}

	uoms, err := client.SearchUOMs(context.Background(), server.URL, "key", "secret", "k", 10)
	if err != nil {
		t.Fatalf("unexpected uom error: %v", err)
	}
	if len(uoms) != 2 || uoms[0].Name == "" {
		t.Fatalf("unexpected uoms: %+v", uoms)
	}
}

func TestBuildSearchQueryVariantsAddsLatinFallbackForCyrillic(t *testing.T) {
	variants := buildSearchQueryVariants("омбор")
	if len(variants) != 2 {
		t.Fatalf("expected 2 variants, got %v", variants)
	}
	if variants[0] != "омбор" {
		t.Fatalf("unexpected first variant: %q", variants[0])
	}
	if variants[1] != "ombor" {
		t.Fatalf("unexpected latin fallback: %q", variants[1])
	}
}

func TestSearchWarehousesUsesCyrillicFallbackVariant(t *testing.T) {
	var queries []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/method/frappe.desk.search.search_link" {
			http.NotFound(w, r)
			return
		}

		query, err := url.QueryUnescape(r.URL.Query().Get("txt"))
		if err != nil {
			t.Fatalf("failed to decode query: %v", err)
		}
		queries = append(queries, query)

		if query == "ombor" {
			_, _ = w.Write([]byte(`{"message":[{"value":"Stores - CH"}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"message":[]}`))
	}))
	defer server.Close()

	client := NewClient(&http.Client{Timeout: 3 * time.Second})
	warehouses, err := client.SearchWarehouses(context.Background(), server.URL, "key", "secret", "омбор", 10)
	if err != nil {
		t.Fatalf("unexpected warehouse error: %v", err)
	}
	if len(warehouses) != 1 || warehouses[0].Name != "Stores - CH" {
		t.Fatalf("unexpected warehouses: %+v", warehouses)
	}
	if len(queries) < 2 {
		t.Fatalf("expected fallback queries, got %v", queries)
	}
	if queries[0] != "омбор" || queries[1] != "ombor" {
		t.Fatalf("unexpected queries order: %v", queries)
	}
}
