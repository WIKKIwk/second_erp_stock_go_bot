package suplier

import "context"

type Repository interface {
	Add(ctx context.Context, supplier Supplier) error
	FindByPhone(ctx context.Context, phone string) (Supplier, bool, error)
	List(ctx context.Context) ([]Supplier, error)
}
