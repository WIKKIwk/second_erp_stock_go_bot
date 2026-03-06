package suplier

import "context"

type Repository interface {
	Search(ctx context.Context, options SearchOptions) ([]Supplier, error)
}
