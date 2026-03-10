package mobileapi

import "erpnext_stock_telegram/internal/core"

type ProfilePrefs = core.ProfilePrefs
type ProfileStore = core.ProfileStore

func NewProfileStore(path string) *ProfileStore {
	return core.NewProfileStore(path)
}
