package suplier

import "context"

type AuthRepository interface {
	FindByPhone(ctx context.Context, phone string) (SupplierAuth, bool, error)
	List(ctx context.Context) ([]SupplierAuth, error)
	Upsert(ctx context.Context, auth SupplierAuth) error
}
