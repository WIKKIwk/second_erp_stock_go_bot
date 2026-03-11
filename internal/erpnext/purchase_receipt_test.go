package erpnext

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestListAssignedSupplierItemsIncludesParentDoctype(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/method/frappe.desk.search.search_link":
			_, _ = w.Write([]byte(`{"message":[{"value":"SUP-001"}]}`))
		case "/api/resource/Item Supplier":
			if got := r.URL.Query().Get("parent"); got != "Item" {
				http.Error(w, "missing parent doctype", http.StatusForbidden)
				return
			}
			_, _ = w.Write([]byte(`{"data":[{"parent":"ITEM-001"}]}`))
		case "/api/resource/Item":
			_, _ = w.Write([]byte(`{"data":[{"name":"ITEM-001","item_name":"Bolt","stock_uom":"Nos"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(&http.Client{Timeout: 3 * time.Second})
	items, err := client.ListAssignedSupplierItems(context.Background(), server.URL, "key", "secret", "SUP-001", 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Code != "ITEM-001" {
		t.Fatalf("expected ITEM-001, got %q", items[0].Code)
	}
}
