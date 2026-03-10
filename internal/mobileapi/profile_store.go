package mobileapi

import "erpnext_stock_telegram/internal/core"

type ProfilePrefs = core.ProfilePrefs
type ProfileStore = core.ProfileStore
type AdminSupplierStore = core.AdminSupplierStore

func NewProfileStore(path string) *ProfileStore {
	return core.NewProfileStore(path)
}

func NewAdminSupplierStore(path string) *AdminSupplierStore {
	return core.NewAdminSupplierStore(path)
}
