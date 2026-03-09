package suplier

import (
	"context"
	"testing"
)

type fakeRepository struct {
	items []Supplier
}

func (f *fakeRepository) Add(_ context.Context, supplier Supplier) error {
	for i := range f.items {
		if f.items[i].Phone == supplier.Phone {
			f.items[i] = supplier
			return nil
		}
	}
	f.items = append(f.items, supplier)
	return nil
}

func (f *fakeRepository) FindByPhone(_ context.Context, phone string) (Supplier, bool, error) {
	for _, item := range f.items {
		if item.Phone == phone {
			return item, true, nil
		}
	}
	return Supplier{}, false, nil
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

func TestServiceFindByPhone(t *testing.T) {
	repository := &fakeRepository{
		items: []Supplier{{Name: "Ali", Phone: "+998901234567"}},
	}
	service := NewService(repository)

	supplier, ok, err := service.FindByPhone(context.Background(), "998901234567")
	if err != nil {
		t.Fatalf("FindByPhone returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected supplier to be found")
	}
	if supplier.Name != "Ali" {
		t.Fatalf("unexpected supplier: %+v", supplier)
	}
}

func TestServiceAddRejectsDuplicateName(t *testing.T) {
	repository := &fakeRepository{
		items: []Supplier{{Name: "Ali", Phone: "+998901234567"}},
	}
	service := NewService(repository)

	_, err := service.Add(context.Background(), "Ali", "998901111111")
	if err == nil {
		t.Fatal("expected duplicate name error")
	}
}

func TestServiceAddUpdatesExistingPhoneRecord(t *testing.T) {
	repository := &fakeRepository{
		items: []Supplier{{Name: "Stock", Phone: "+998901234567"}},
	}
	service := NewService(repository)

	supplier, err := service.Add(context.Background(), "Stocker", "998901234567")
	if err != nil {
		t.Fatalf("Add returned error: %v", err)
	}
	if supplier.Name != "Stocker" {
		t.Fatalf("unexpected supplier: %+v", supplier)
	}
	if len(repository.items) != 1 || repository.items[0].Name != "Stocker" {
		t.Fatalf("expected phone match to update existing record, got %+v", repository.items)
	}
}
