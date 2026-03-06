package suplier

import (
	"context"
	"testing"
)

type fakeRepository struct {
	items []Supplier
}

func (f *fakeRepository) Add(_ context.Context, supplier Supplier) error {
	f.items = append(f.items, supplier)
	return nil
}

func (f *fakeRepository) List(_ context.Context) ([]Supplier, error) {
	return append([]Supplier(nil), f.items...), nil
}

func TestServiceAdd(t *testing.T) {
	repository := &fakeRepository{}
	service := NewService(repository)

	supplier, err := service.Add(context.Background(), "Ali", "998901234567")
	if err != nil {
		t.Fatalf("Add returned error: %v", err)
	}
	if supplier.Phone != "+998901234567" {
		t.Fatalf("unexpected supplier: %+v", supplier)
	}
	if len(repository.items) != 1 {
		t.Fatalf("expected 1 saved supplier, got %d", len(repository.items))
	}
}
