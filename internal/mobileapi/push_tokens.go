package mobileapi

import "erpnext_stock_telegram/internal/core"

type PushTokenStore = core.PushTokenStore

func NewPushTokenStore(path string) *PushTokenStore {
	return core.NewPushTokenStore(path)
}
