package suplier

import "context"

type Repository interface {
	Add(ctx context.Context, supplier Supplier) error
	List(ctx context.Context) ([]Supplier, error)
}
