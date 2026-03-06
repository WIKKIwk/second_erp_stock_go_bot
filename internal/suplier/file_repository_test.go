package suplier

import (
	"context"
	"path/filepath"
	"testing"
)

func TestFileRepositoryAddAndList(t *testing.T) {
	repository := NewFileRepository(filepath.Join(t.TempDir(), "suppliers.fb"))

	err := repository.Add(context.Background(), Supplier{Name: "Ali", Phone: "+998901234567"})
	if err != nil {
		t.Fatalf("Add returned error: %v", err)
	}
	err = repository.Add(context.Background(), Supplier{Name: "Vali", Phone: "+998901111111"})
	if err != nil {
		t.Fatalf("Add returned error: %v", err)
	}

	items, err := repository.List(context.Background())
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 suppliers, got %d", len(items))
	}
	if items[0].Name != "Ali" || items[1].Phone != "+998901111111" {
		t.Fatalf("unexpected suppliers: %+v", items)
	}
}
