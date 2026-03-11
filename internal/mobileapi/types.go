package mobileapi

import "erpnext_stock_telegram/internal/core"

type PrincipalRole = core.PrincipalRole

const (
	RoleSupplier = core.RoleSupplier
	RoleWerka    = core.RoleWerka
	RoleAdmin    = core.RoleAdmin
)

type Principal = core.Principal
type Authenticator = core.Authenticator
type LoginRequest = core.LoginRequest
type LoginResponse = core.LoginResponse
type DispatchRecord = core.DispatchRecord
type NotificationComment = core.NotificationComment
type NotificationDetail = core.NotificationDetail
type SupplierItem = core.SupplierItem
type CreateDispatchRequest = core.CreateDispatchRequest
type ConfirmReceiptRequest = core.ConfirmReceiptRequest
type NotificationCommentCreateRequest = core.NotificationCommentCreateRequest
type ProfileUpdateRequest = core.ProfileUpdateRequest
type AdminSettings = core.AdminSettings
type AdminSupplier = core.AdminSupplier
type AdminCreateSupplierRequest = core.AdminCreateSupplierRequest
type AdminSupplierSummary = core.AdminSupplierSummary
type AdminSupplierDetail = core.AdminSupplierDetail
type AdminSupplierStatusUpdateRequest = core.AdminSupplierStatusUpdateRequest
type AdminSupplierPhoneUpdateRequest = core.AdminSupplierPhoneUpdateRequest
type AdminSupplierItemsUpdateRequest = core.AdminSupplierItemsUpdateRequest
type AdminSupplierItemMutationRequest = core.AdminSupplierItemMutationRequest
type AdminCreateItemRequest = core.AdminCreateItemRequest
