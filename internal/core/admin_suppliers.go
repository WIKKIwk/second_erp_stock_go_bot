package core

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"erpnext_stock_telegram/internal/erpnext"
	"erpnext_stock_telegram/internal/suplier"
)

const supplierCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

var ErrAdminSupplierNotFound = errors.New("admin supplier not found")

func (a *ERPAuthenticator) AdminSupplierSummary(ctx context.Context, limit int) (AdminSupplierSummary, error) {
	items, err := a.AdminSuppliers(ctx, limit)
	if err != nil {
		return AdminSupplierSummary{}, err
	}

	summary := AdminSupplierSummary{
		TotalSuppliers: len(items),
	}
	for _, item := range items {
		if item.Blocked {
			summary.BlockedSuppliers++
			continue
		}
		summary.ActiveSuppliers++
	}
	return summary, nil
}

func (a *ERPAuthenticator) AdminSuppliers(ctx context.Context, limit int) ([]AdminSupplier, error) {
	items, err := a.erp.SearchSuppliers(ctx, a.baseURL, a.apiKey, a.apiSecret, "", limit)
	if err != nil {
		return nil, err
	}

	result := make([]AdminSupplier, 0, len(items))
	for _, item := range items {
		state, err := a.adminSupplierState(item.ID)
		if err != nil {
			return nil, err
		}
		if state.Removed {
			continue
		}

		adminItem, err := a.buildAdminSupplier(item, state)
		if err != nil {
			continue
		}
		result = append(result, adminItem)
	}
	return result, nil
}

func (a *ERPAuthenticator) AdminSupplierDetail(ctx context.Context, ref string) (AdminSupplierDetail, error) {
	item, state, err := a.findSupplierForAdmin(ctx, ref)
	if err != nil {
		return AdminSupplierDetail{}, err
	}

	assignedItems, err := a.adminAssignedItems(ctx, state.AssignedItemCodes)
	if err != nil {
		return AdminSupplierDetail{}, err
	}

	code, err := a.supplierAccessCode(item, state)
	if err != nil {
		return AdminSupplierDetail{}, err
	}

	return AdminSupplierDetail{
		Ref:           item.ID,
		Name:          item.Name,
		Phone:         item.Phone,
		Code:          code,
		Blocked:       state.Blocked,
		AssignedItems: assignedItems,
	}, nil
}

func (a *ERPAuthenticator) AdminSearchItems(ctx context.Context, query string, limit int) ([]SupplierItem, error) {
	items, err := a.erp.SearchItems(ctx, a.baseURL, a.apiKey, a.apiSecret, query, limit)
	if err != nil {
		return nil, err
	}
	return a.mapSupplierItems(ctx, items)
}

func (a *ERPAuthenticator) AdminUpdateSupplierItems(ctx context.Context, ref string, itemCodes []string) (AdminSupplierDetail, error) {
	item, state, err := a.findSupplierForAdmin(ctx, ref)
	if err != nil {
		return AdminSupplierDetail{}, err
	}

	normalizedCodes := normalizeItemCodes(itemCodes)
	if len(normalizedCodes) > 0 {
		items, err := a.erp.GetItemsByCodes(ctx, a.baseURL, a.apiKey, a.apiSecret, normalizedCodes)
		if err != nil {
			return AdminSupplierDetail{}, err
		}
		found := make(map[string]struct{}, len(items))
		for _, item := range items {
			found[strings.TrimSpace(item.Code)] = struct{}{}
		}
		for _, code := range normalizedCodes {
			if _, ok := found[code]; !ok {
				return AdminSupplierDetail{}, fmt.Errorf("item topilmadi: %s", code)
			}
		}
	}

	state.AssignmentsConfigured = true
	state.AssignedItemCodes = normalizedCodes
	state.UpdatedAt = time.Now().UTC()
	if err := a.saveAdminSupplierState(item.ID, state); err != nil {
		return AdminSupplierDetail{}, err
	}
	return a.AdminSupplierDetail(ctx, item.ID)
}

func (a *ERPAuthenticator) AdminSetSupplierBlocked(ctx context.Context, ref string, blocked bool) (AdminSupplierDetail, error) {
	item, state, err := a.findSupplierForAdmin(ctx, ref)
	if err != nil {
		return AdminSupplierDetail{}, err
	}
	state.Blocked = blocked
	state.UpdatedAt = time.Now().UTC()
	if err := a.saveAdminSupplierState(item.ID, state); err != nil {
		return AdminSupplierDetail{}, err
	}
	return a.AdminSupplierDetail(ctx, item.ID)
}

func (a *ERPAuthenticator) AdminRegenerateSupplierCode(ctx context.Context, ref string) (AdminSupplierDetail, error) {
	item, state, err := a.findSupplierForAdmin(ctx, ref)
	if err != nil {
		return AdminSupplierDetail{}, err
	}

	items, err := a.erp.SearchSuppliers(ctx, a.baseURL, a.apiKey, a.apiSecret, "", 500)
	if err != nil {
		return AdminSupplierDetail{}, err
	}

	existingCodes := make(map[string]struct{}, len(items))
	for _, candidate := range items {
		candidateState, err := a.adminSupplierState(candidate.ID)
		if err != nil {
			return AdminSupplierDetail{}, err
		}
		if candidateState.Removed {
			continue
		}
		code, err := a.supplierAccessCode(candidate, candidateState)
		if err != nil {
			continue
		}
		existingCodes[code] = struct{}{}
	}

	state.CustomCode, err = randomSupplierCode(a.supplierPrefix, existingCodes)
	if err != nil {
		return AdminSupplierDetail{}, err
	}
	state.UpdatedAt = time.Now().UTC()
	if err := a.saveAdminSupplierState(item.ID, state); err != nil {
		return AdminSupplierDetail{}, err
	}
	return a.AdminSupplierDetail(ctx, item.ID)
}

func (a *ERPAuthenticator) AdminRemoveSupplier(ctx context.Context, ref string) error {
	item, state, err := a.findSupplierForAdmin(ctx, ref)
	if err != nil {
		return err
	}
	state.Removed = true
	state.Blocked = true
	state.UpdatedAt = time.Now().UTC()
	return a.saveAdminSupplierState(item.ID, state)
}

func (a *ERPAuthenticator) AdminCreateSupplier(ctx context.Context, name, phone string) (AdminSupplier, error) {
	item, err := a.erp.EnsureSupplier(ctx, a.baseURL, a.apiKey, a.apiSecret, erpnext.CreateSupplierInput{
		Name:  strings.TrimSpace(name),
		Phone: strings.TrimSpace(phone),
	})
	if err != nil {
		return AdminSupplier{}, err
	}

	state, err := a.adminSupplierState(item.ID)
	if err != nil {
		return AdminSupplier{}, err
	}
	if state.Removed {
		state.Removed = false
		state.Blocked = false
		state.UpdatedAt = time.Now().UTC()
		if err := a.saveAdminSupplierState(item.ID, state); err != nil {
			return AdminSupplier{}, err
		}
	}

	return a.buildAdminSupplier(item, state)
}

func (a *ERPAuthenticator) supplierAllowedItems(ctx context.Context, principal Principal, query string, limit int) ([]SupplierItem, error) {
	state, err := a.adminSupplierState(principal.Ref)
	if err != nil {
		return nil, err
	}
	if state.Removed || state.Blocked {
		return []SupplierItem{}, nil
	}
	if !state.AssignmentsConfigured {
		items, err := a.erp.SearchSupplierItems(ctx, a.baseURL, a.apiKey, a.apiSecret, principal.Ref, query, limit)
		if err != nil {
			return nil, err
		}
		return a.mapSupplierItems(ctx, items)
	}
	if len(state.AssignedItemCodes) == 0 {
		return []SupplierItem{}, nil
	}

	items, err := a.erp.GetItemsByCodes(ctx, a.baseURL, a.apiKey, a.apiSecret, state.AssignedItemCodes)
	if err != nil {
		return nil, err
	}
	if trimmed := strings.TrimSpace(query); trimmed != "" {
		items = filterItemsByQuery(items, trimmed)
	}
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return a.mapSupplierItems(ctx, items)
}

func (a *ERPAuthenticator) validateSupplierItemAllowed(ctx context.Context, supplierRef, itemCode string) error {
	state, err := a.adminSupplierState(supplierRef)
	if err != nil {
		return err
	}
	if state.Removed || state.Blocked {
		return ErrInvalidCredentials
	}
	if !state.AssignmentsConfigured {
		return nil
	}
	if stateIncludesItem(state, itemCode) {
		return nil
	}
	return fmt.Errorf("item supplierga biriktirilmagan")
}

func (a *ERPAuthenticator) adminSupplierState(ref string) (AdminSupplierState, error) {
	if a.supplierAdmin == nil {
		return AdminSupplierState{}, nil
	}
	return a.supplierAdmin.Get(strings.TrimSpace(ref))
}

func (a *ERPAuthenticator) saveAdminSupplierState(ref string, state AdminSupplierState) error {
	if a.supplierAdmin == nil {
		return nil
	}
	state.CustomCode = strings.TrimSpace(state.CustomCode)
	state.AssignedItemCodes = normalizeItemCodes(state.AssignedItemCodes)
	return a.supplierAdmin.Put(strings.TrimSpace(ref), state)
}

func (a *ERPAuthenticator) buildAdminSupplier(item erpnext.Supplier, state AdminSupplierState) (AdminSupplier, error) {
	code, err := a.supplierAccessCode(item, state)
	if err != nil {
		return AdminSupplier{}, err
	}
	return AdminSupplier{
		Ref:               item.ID,
		Name:              item.Name,
		Phone:             item.Phone,
		Code:              code,
		Blocked:           state.Blocked,
		AssignedItemCodes: append([]string(nil), state.AssignedItemCodes...),
		AssignedItemCount: len(state.AssignedItemCodes),
	}, nil
}

func (a *ERPAuthenticator) supplierAccessCode(item erpnext.Supplier, state AdminSupplierState) (string, error) {
	if trimmed := strings.TrimSpace(state.CustomCode); trimmed != "" {
		return trimmed, nil
	}
	creds, err := suplier.GenerateAccessCredentials(suplier.Supplier{
		Ref:   item.ID,
		Name:  item.Name,
		Phone: item.Phone,
	})
	if err != nil {
		return "", err
	}
	return creds.Code, nil
}

func (a *ERPAuthenticator) findSupplierForAdmin(ctx context.Context, ref string) (erpnext.Supplier, AdminSupplierState, error) {
	doc, err := a.erp.GetSupplier(ctx, a.baseURL, a.apiKey, a.apiSecret, strings.TrimSpace(ref))
	if err != nil {
		return erpnext.Supplier{}, AdminSupplierState{}, err
	}
	if strings.TrimSpace(doc.ID) == "" {
		return erpnext.Supplier{}, AdminSupplierState{}, ErrAdminSupplierNotFound
	}

	state, err := a.adminSupplierState(doc.ID)
	if err != nil {
		return erpnext.Supplier{}, AdminSupplierState{}, err
	}
	if state.Removed {
		return erpnext.Supplier{}, AdminSupplierState{}, ErrAdminSupplierNotFound
	}
	return doc, state, nil
}

func (a *ERPAuthenticator) adminAssignedItems(ctx context.Context, itemCodes []string) ([]SupplierItem, error) {
	normalizedCodes := normalizeItemCodes(itemCodes)
	if len(normalizedCodes) == 0 {
		return []SupplierItem{}, nil
	}
	items, err := a.erp.GetItemsByCodes(ctx, a.baseURL, a.apiKey, a.apiSecret, normalizedCodes)
	if err != nil {
		return nil, err
	}
	return a.mapSupplierItems(ctx, items)
}

func (a *ERPAuthenticator) mapSupplierItems(ctx context.Context, items []erpnext.Item) ([]SupplierItem, error) {
	warehouse, err := a.resolveWarehouse(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]SupplierItem, 0, len(items))
	for _, item := range items {
		result = append(result, SupplierItem{
			Code:      item.Code,
			Name:      item.Name,
			UOM:       item.UOM,
			Warehouse: warehouse,
		})
	}
	return result, nil
}

func normalizeItemCodes(itemCodes []string) []string {
	normalized := make([]string, 0, len(itemCodes))
	seen := make(map[string]struct{}, len(itemCodes))
	for _, code := range itemCodes {
		trimmed := strings.TrimSpace(code)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	return normalized
}

func filterItemsByQuery(items []erpnext.Item, query string) []erpnext.Item {
	lowerQuery := strings.ToLower(strings.TrimSpace(query))
	if lowerQuery == "" {
		return items
	}

	filtered := make([]erpnext.Item, 0, len(items))
	for _, item := range items {
		if strings.Contains(strings.ToLower(item.Code), lowerQuery) ||
			strings.Contains(strings.ToLower(item.Name), lowerQuery) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func randomSupplierCode(prefix string, existing map[string]struct{}) (string, error) {
	if strings.TrimSpace(prefix) == "" {
		prefix = "10"
	}
	for attempts := 0; attempts < 64; attempts++ {
		buf := make([]byte, 10)
		if _, err := rand.Read(buf); err != nil {
			return "", err
		}
		builder := strings.Builder{}
		builder.Grow(len(prefix) + len(buf))
		builder.WriteString(prefix)
		for _, value := range buf {
			builder.WriteByte(supplierCodeAlphabet[int(value)%len(supplierCodeAlphabet)])
		}
		code := builder.String()
		if _, ok := existing[code]; ok {
			continue
		}
		return code, nil
	}
	return "", fmt.Errorf("supplier code generation failed")
}

func stateIncludesItem(state AdminSupplierState, itemCode string) bool {
	return slices.ContainsFunc(state.AssignedItemCodes, func(candidate string) bool {
		return strings.EqualFold(strings.TrimSpace(candidate), strings.TrimSpace(itemCode))
	})
}
