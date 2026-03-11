package erpnext

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const telegramReceiptMarkerPrefix = "TG:"

type CreatePurchaseReceiptInput struct {
	Supplier      string
	SupplierPhone string
	ItemCode      string
	Qty           float64
	UOM           string
	Warehouse     string
}

type PurchaseReceiptDraft struct {
	Name                 string
	DocStatus            int
	Status               string
	Supplier             string
	SupplierName         string
	PostingDate          string
	SupplierDeliveryNote string
	ItemCode             string
	ItemName             string
	Qty                  float64
	UOM                  string
	Warehouse            string
	Remarks              string
}

type PurchaseReceiptSubmissionResult struct {
	Name                 string
	Supplier             string
	ItemCode             string
	UOM                  string
	SentQty              float64
	AcceptedQty          float64
	SupplierDeliveryNote string
	Note                 string
}

type Comment struct {
	ID        string
	Content   string
	CreatedAt string
}

func (c *Client) SearchSupplierItems(ctx context.Context, baseURL, apiKey, apiSecret, supplier, query string, limit int) ([]Item, error) {
	normalized, err := normalizeBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	supplierLink, err := c.resolveSupplierLink(ctx, normalized, apiKey, apiSecret, supplier)
	if err != nil {
		return nil, err
	}

	searchLimit := limit * 10
	if searchLimit < 50 {
		searchLimit = 50
	}
	if searchLimit > 100 {
		searchLimit = 100
	}

	candidates, err := c.searchItemsByQuery(ctx, normalized, apiKey, apiSecret, query, searchLimit)
	if err != nil {
		return nil, err
	}

	filtered := make([]Item, 0, limit)
	for _, item := range candidates {
		match, err := c.itemHasSupplier(ctx, normalized, apiKey, apiSecret, item.Code, supplierLink)
		if err != nil {
			return nil, err
		}
		if !match {
			continue
		}
		filtered = append(filtered, item)
		if len(filtered) >= limit {
			break
		}
	}

	return filtered, nil
}

func (c *Client) ListAssignedSupplierItems(ctx context.Context, baseURL, apiKey, apiSecret, supplier string, limit int) ([]Item, error) {
	normalized, err := normalizeBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	supplierLink, err := c.resolveSupplierLink(ctx, normalized, apiKey, apiSecret, supplier)
	if err != nil {
		return nil, err
	}
	itemCodes, err := c.fetchSupplierItemCodes(ctx, normalized, apiKey, apiSecret, supplierLink, limit)
	if err != nil {
		return nil, err
	}
	return c.searchItemsByCodes(ctx, normalized, apiKey, apiSecret, itemCodes, "", limit)
}

func (c *Client) AssignSupplierToItem(ctx context.Context, baseURL, apiKey, apiSecret, itemCode, supplier string) error {
	normalized, err := normalizeBaseURL(baseURL)
	if err != nil {
		return err
	}
	supplierLink, err := c.resolveSupplierLink(ctx, normalized, apiKey, apiSecret, supplier)
	if err != nil {
		return err
	}
	match, err := c.itemHasSupplier(ctx, normalized, apiKey, apiSecret, itemCode, supplierLink)
	if err != nil {
		return err
	}
	if match {
		return nil
	}
	endpoint := normalized + "/api/resource/Item%20Supplier"
	return c.doJSONRequest(ctx, http.MethodPost, endpoint, apiKey, apiSecret, map[string]interface{}{
		"parent":      strings.TrimSpace(itemCode),
		"parenttype":  "Item",
		"parentfield": "supplier_items",
		"supplier":    supplierLink,
	}, nil)
}

func (c *Client) RemoveSupplierFromItem(ctx context.Context, baseURL, apiKey, apiSecret, itemCode, supplier string) error {
	normalized, err := normalizeBaseURL(baseURL)
	if err != nil {
		return err
	}
	supplierLink, err := c.resolveSupplierLink(ctx, normalized, apiKey, apiSecret, supplier)
	if err != nil {
		return err
	}

	var payload struct {
		Data struct {
			DefaultSupplier string `json:"default_supplier"`
			SupplierItems   []struct {
				Name     string `json:"name"`
				Supplier string `json:"supplier"`
			} `json:"supplier_items"`
		} `json:"data"`
	}
	endpoint := normalized + "/api/resource/Item/" + url.PathEscape(strings.TrimSpace(itemCode))
	if err := c.doJSON(ctx, endpoint, apiKey, apiSecret, &payload); err != nil {
		return err
	}

	for _, row := range payload.Data.SupplierItems {
		if !strings.EqualFold(strings.TrimSpace(row.Supplier), strings.TrimSpace(supplierLink)) {
			continue
		}
		if strings.TrimSpace(row.Name) == "" {
			continue
		}
		deleteEndpoint := normalized + "/api/resource/Item%20Supplier/" + url.PathEscape(strings.TrimSpace(row.Name))
		if err := c.doJSONRequest(ctx, http.MethodDelete, deleteEndpoint, apiKey, apiSecret, nil, nil); err != nil {
			return err
		}
	}

	if strings.EqualFold(strings.TrimSpace(payload.Data.DefaultSupplier), strings.TrimSpace(supplierLink)) {
		if err := c.doJSONRequest(ctx, http.MethodPut, endpoint, apiKey, apiSecret, map[string]string{
			"default_supplier": "",
		}, nil); err != nil {
			return err
		}
	}

	return nil
}

func (c *Client) GetItemsByCodes(ctx context.Context, baseURL, apiKey, apiSecret string, itemCodes []string) ([]Item, error) {
	normalized, err := normalizeBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	return c.searchItemsByCodes(ctx, normalized, apiKey, apiSecret, itemCodes, "", len(itemCodes))
}

func (c *Client) CreateDraftPurchaseReceipt(ctx context.Context, baseURL, apiKey, apiSecret string, input CreatePurchaseReceiptInput) (PurchaseReceiptDraft, error) {
	normalized, err := normalizeBaseURL(baseURL)
	if err != nil {
		return PurchaseReceiptDraft{}, err
	}
	if input.Qty <= 0 {
		return PurchaseReceiptDraft{}, fmt.Errorf("qty must be greater than 0")
	}
	if strings.TrimSpace(input.ItemCode) == "" {
		return PurchaseReceiptDraft{}, fmt.Errorf("item code is required")
	}
	if strings.TrimSpace(input.Supplier) == "" {
		return PurchaseReceiptDraft{}, fmt.Errorf("supplier is required")
	}
	if strings.TrimSpace(input.Warehouse) == "" {
		return PurchaseReceiptDraft{}, fmt.Errorf("warehouse is required")
	}

	supplierLink, err := c.resolveSupplierLink(ctx, normalized, apiKey, apiSecret, input.Supplier)
	if err != nil {
		return PurchaseReceiptDraft{}, err
	}

	company, err := c.fetchWarehouseCompany(ctx, normalized, apiKey, apiSecret, input.Warehouse)
	if err != nil {
		return PurchaseReceiptDraft{}, err
	}

	uom := strings.TrimSpace(input.UOM)
	if uom == "" {
		items, err := c.searchItemsByCodes(ctx, normalized, apiKey, apiSecret, []string{input.ItemCode}, input.ItemCode, 1)
		if err != nil {
			return PurchaseReceiptDraft{}, err
		}
		if len(items) > 0 && strings.TrimSpace(items[0].UOM) != "" {
			uom = strings.TrimSpace(items[0].UOM)
		}
	}
	if uom == "" {
		uom = "Nos"
	}

	payload := map[string]interface{}{
		"supplier":               supplierLink,
		"company":                company,
		"posting_date":           time.Now().Format("2006-01-02"),
		"set_warehouse":          strings.TrimSpace(input.Warehouse),
		"supplier_delivery_note": buildTelegramReceiptMarker(input.SupplierPhone, input.Qty, time.Now().UTC()),
		"items": []map[string]interface{}{
			{
				"item_code":                 strings.TrimSpace(input.ItemCode),
				"warehouse":                 strings.TrimSpace(input.Warehouse),
				"qty":                       input.Qty,
				"received_qty":              input.Qty,
				"uom":                       uom,
				"stock_uom":                 uom,
				"conversion_factor":         1,
				"stock_qty":                 input.Qty,
				"received_stock_qty":        input.Qty,
				"rate":                      0,
				"allow_zero_valuation_rate": 1,
			},
		},
	}

	var createResp struct {
		Data struct {
			Name string `json:"name"`
		} `json:"data"`
	}
	endpoint := normalized + "/api/resource/Purchase Receipt"
	if err := c.doJSONRequest(ctx, http.MethodPost, endpoint, apiKey, apiSecret, payload, &createResp); err != nil {
		return PurchaseReceiptDraft{}, err
	}
	if createResp.Data.Name == "" {
		return PurchaseReceiptDraft{}, fmt.Errorf("purchase receipt create response did not return name")
	}

	return c.GetPurchaseReceipt(ctx, normalized, apiKey, apiSecret, createResp.Data.Name)
}

func (c *Client) ListPendingPurchaseReceipts(ctx context.Context, baseURL, apiKey, apiSecret string, limit int) ([]PurchaseReceiptDraft, error) {
	normalized, err := normalizeBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 50 {
		limit = 10
	}

	filtersJSON, _ := json.Marshal([][]interface{}{
		{"docstatus", "=", 0},
	})
	params := url.Values{}
	params.Set("fields", `["name"]`)
	params.Set("filters", string(filtersJSON))
	params.Set("limit_page_length", fmt.Sprintf("%d", limit))
	params.Set("order_by", "modified desc")

	var payload struct {
		Data []struct {
			Name string `json:"name"`
		} `json:"data"`
	}
	endpoint := normalized + "/api/resource/Purchase Receipt?" + params.Encode()
	if err := c.doJSON(ctx, endpoint, apiKey, apiSecret, &payload); err != nil {
		return nil, err
	}

	items := make([]PurchaseReceiptDraft, 0, len(payload.Data))
	for _, row := range payload.Data {
		if strings.TrimSpace(row.Name) == "" {
			continue
		}
		doc, err := c.GetPurchaseReceipt(ctx, normalized, apiKey, apiSecret, row.Name)
		if err != nil {
			return nil, err
		}
		items = append(items, doc)
	}
	return items, nil
}

func (c *Client) ListSupplierPurchaseReceipts(ctx context.Context, baseURL, apiKey, apiSecret, supplier string, limit int) ([]PurchaseReceiptDraft, error) {
	normalized, err := normalizeBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	filtersJSON, _ := json.Marshal([][]interface{}{
		{"supplier", "=", strings.TrimSpace(supplier)},
		{"supplier_delivery_note", "like", telegramReceiptMarkerPrefix + "%"},
	})
	params := url.Values{}
	params.Set("fields", `["name"]`)
	params.Set("filters", string(filtersJSON))
	params.Set("limit_page_length", fmt.Sprintf("%d", limit))
	params.Set("order_by", "modified desc")

	var payload struct {
		Data []struct {
			Name string `json:"name"`
		} `json:"data"`
	}
	endpoint := normalized + "/api/resource/Purchase Receipt?" + params.Encode()
	if err := c.doJSON(ctx, endpoint, apiKey, apiSecret, &payload); err != nil {
		return nil, err
	}

	items := make([]PurchaseReceiptDraft, 0, len(payload.Data))
	for _, row := range payload.Data {
		if strings.TrimSpace(row.Name) == "" {
			continue
		}
		doc, err := c.GetPurchaseReceipt(ctx, normalized, apiKey, apiSecret, row.Name)
		if err != nil {
			return nil, err
		}
		items = append(items, doc)
	}
	return items, nil
}

func (c *Client) ListTelegramPurchaseReceipts(ctx context.Context, baseURL, apiKey, apiSecret string, limit int) ([]PurchaseReceiptDraft, error) {
	normalized, err := normalizeBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	filtersJSON, _ := json.Marshal([][]interface{}{
		{"supplier_delivery_note", "like", telegramReceiptMarkerPrefix + "%"},
	})
	params := url.Values{}
	params.Set("fields", `["name"]`)
	params.Set("filters", string(filtersJSON))
	params.Set("limit_page_length", fmt.Sprintf("%d", limit))
	params.Set("order_by", "modified desc")

	var payload struct {
		Data []struct {
			Name string `json:"name"`
		} `json:"data"`
	}
	endpoint := normalized + "/api/resource/Purchase Receipt?" + params.Encode()
	if err := c.doJSON(ctx, endpoint, apiKey, apiSecret, &payload); err != nil {
		return nil, err
	}

	items := make([]PurchaseReceiptDraft, 0, len(payload.Data))
	for _, row := range payload.Data {
		if strings.TrimSpace(row.Name) == "" {
			continue
		}
		doc, err := c.GetPurchaseReceipt(ctx, normalized, apiKey, apiSecret, row.Name)
		if err != nil {
			return nil, err
		}
		items = append(items, doc)
	}
	return items, nil
}

func (c *Client) GetPurchaseReceipt(ctx context.Context, baseURL, apiKey, apiSecret, name string) (PurchaseReceiptDraft, error) {
	normalized, err := normalizeBaseURL(baseURL)
	if err != nil {
		return PurchaseReceiptDraft{}, err
	}

	doc, err := c.fetchPurchaseReceiptDoc(ctx, normalized, apiKey, apiSecret, name)
	if err != nil {
		return PurchaseReceiptDraft{}, err
	}

	return mapPurchaseReceiptDraft(doc)
}

func (c *Client) ListPurchaseReceiptComments(ctx context.Context, baseURL, apiKey, apiSecret, name string, limit int) ([]Comment, error) {
	normalized, err := normalizeBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	filtersJSON, _ := json.Marshal([][]interface{}{
		{"reference_doctype", "=", "Purchase Receipt"},
		{"reference_name", "=", strings.TrimSpace(name)},
		{"comment_type", "=", "Comment"},
	})
	params := url.Values{}
	params.Set("fields", `["name","content","creation"]`)
	params.Set("filters", string(filtersJSON))
	params.Set("order_by", "creation asc")
	params.Set("limit_page_length", fmt.Sprintf("%d", limit))

	var payload struct {
		Data []struct {
			Name     string `json:"name"`
			Content  string `json:"content"`
			Creation string `json:"creation"`
		} `json:"data"`
	}
	endpoint := normalized + "/api/resource/Comment?" + params.Encode()
	if err := c.doJSON(ctx, endpoint, apiKey, apiSecret, &payload); err != nil {
		return nil, err
	}

	items := make([]Comment, 0, len(payload.Data))
	for _, row := range payload.Data {
		items = append(items, Comment{
			ID:        strings.TrimSpace(row.Name),
			Content:   strings.TrimSpace(row.Content),
			CreatedAt: strings.TrimSpace(row.Creation),
		})
	}
	return items, nil
}

func (c *Client) AddPurchaseReceiptComment(ctx context.Context, baseURL, apiKey, apiSecret, name, content string) error {
	normalized, err := normalizeBaseURL(baseURL)
	if err != nil {
		return err
	}
	return c.addComment(ctx, normalized, apiKey, apiSecret, "Purchase Receipt", name, content)
}

func (c *Client) ConfirmAndSubmitPurchaseReceipt(ctx context.Context, baseURL, apiKey, apiSecret, name string, acceptedQty, returnedQty float64, returnReason, returnComment string) (PurchaseReceiptSubmissionResult, error) {
	normalized, err := normalizeBaseURL(baseURL)
	if err != nil {
		return PurchaseReceiptSubmissionResult{}, err
	}
	if acceptedQty < 0 {
		return PurchaseReceiptSubmissionResult{}, fmt.Errorf("accepted qty cannot be negative")
	}

	doc, err := c.fetchPurchaseReceiptDoc(ctx, normalized, apiKey, apiSecret, name)
	if err != nil {
		return PurchaseReceiptSubmissionResult{}, err
	}

	draft, err := mapPurchaseReceiptDraft(doc)
	if err != nil {
		return PurchaseReceiptSubmissionResult{}, err
	}
	if acceptedQty > draft.Qty {
		return PurchaseReceiptSubmissionResult{}, fmt.Errorf("accepted qty cannot exceed sent qty")
	}
	decisionNote, err := buildAccordDecisionNote(draft, acceptedQty, returnedQty, returnReason, returnComment)
	if err != nil {
		return PurchaseReceiptSubmissionResult{}, err
	}
	fullReturn := acceptedQty == 0 && returnedQty >= draft.Qty && draft.Qty > 0

	if fullReturn {
		if strings.TrimSpace(decisionNote) != "" {
			updateEndpoint := normalized + "/api/resource/Purchase%20Receipt/" + url.PathEscape(name)
			if err := c.doJSONRequest(ctx, http.MethodPut, updateEndpoint, apiKey, apiSecret, map[string]string{
				"remarks": upsertAccordDecisionInRemarks(strings.TrimSpace(draft.Remarks), decisionNote),
			}, nil); err != nil {
				return PurchaseReceiptSubmissionResult{}, err
			}
			_ = c.addComment(ctx, normalized, apiKey, apiSecret, "Purchase Receipt", name, decisionNote)
		}
		return PurchaseReceiptSubmissionResult{
			Name:                 name,
			Supplier:             draft.Supplier,
			ItemCode:             draft.ItemCode,
			UOM:                  draft.UOM,
			SentQty:              draft.Qty,
			AcceptedQty:          0,
			SupplierDeliveryNote: draft.SupplierDeliveryNote,
			Note:                 ExtractAccordDecisionNote(decisionNote),
		}, nil
	}

	items, ok := doc["items"].([]interface{})
	if !ok || len(items) == 0 {
		return PurchaseReceiptSubmissionResult{}, fmt.Errorf("purchase receipt %s has no items", name)
	}
	firstItem, ok := items[0].(map[string]interface{})
	if !ok {
		return PurchaseReceiptSubmissionResult{}, fmt.Errorf("purchase receipt %s item payload is invalid", name)
	}

	conversionFactor := getFloatValue(firstItem["conversion_factor"])
	if conversionFactor <= 0 {
		conversionFactor = 1
	}
	stockQty := acceptedQty * conversionFactor

	firstItem["qty"] = acceptedQty
	firstItem["received_qty"] = acceptedQty
	firstItem["stock_qty"] = stockQty
	firstItem["received_stock_qty"] = stockQty
	firstItem["rejected_qty"] = returnedQty
	if returnedQty > 0 {
		firstItem["rejected_warehouse"] = getStringValue(firstItem["warehouse"])
	} else {
		firstItem["rejected_warehouse"] = ""
	}
	firstItem["allow_zero_valuation_rate"] = 1
	if _, ok := firstItem["rate"]; !ok {
		firstItem["rate"] = 0
	}
	if strings.TrimSpace(decisionNote) != "" {
		doc["remarks"] = upsertAccordDecisionInRemarks(getStringValue(doc["remarks"]), decisionNote)
	}

	updateEndpoint := normalized + "/api/resource/Purchase%20Receipt/" + url.PathEscape(name)
	if err := c.doJSONRequest(ctx, http.MethodPut, updateEndpoint, apiKey, apiSecret, doc, nil); err != nil {
		return PurchaseReceiptSubmissionResult{}, err
	}

	if err := c.submitDoc(ctx, normalized, apiKey, apiSecret, "Purchase Receipt", name); err != nil {
		return PurchaseReceiptSubmissionResult{}, err
	}
	if strings.TrimSpace(decisionNote) != "" {
		_ = c.addComment(ctx, normalized, apiKey, apiSecret, "Purchase Receipt", name, decisionNote)
	}

	return PurchaseReceiptSubmissionResult{
		Name:                 name,
		Supplier:             draft.Supplier,
		ItemCode:             draft.ItemCode,
		UOM:                  draft.UOM,
		SentQty:              draft.Qty,
		AcceptedQty:          acceptedQty,
		SupplierDeliveryNote: draft.SupplierDeliveryNote,
		Note:                 ExtractAccordDecisionNote(decisionNote),
	}, nil
}

func (c *Client) addComment(ctx context.Context, normalized, apiKey, apiSecret, doctype, name, content string) error {
	if strings.TrimSpace(content) == "" {
		return nil
	}
	endpoint := normalized + "/api/resource/Comment"
	return c.doJSONRequest(ctx, http.MethodPost, endpoint, apiKey, apiSecret, map[string]string{
		"comment_type":      "Comment",
		"reference_doctype": strings.TrimSpace(doctype),
		"reference_name":    strings.TrimSpace(name),
		"content":           strings.TrimSpace(content),
	}, nil)
}

func (c *Client) resolveSupplierLink(ctx context.Context, normalized, apiKey, apiSecret, supplier string) (string, error) {
	links, err := c.searchLink(ctx, normalized, apiKey, apiSecret, "Supplier", supplier, 5)
	if err != nil {
		return "", err
	}
	if len(links) == 0 {
		return "", fmt.Errorf("supplier not found: %s", supplier)
	}

	needle := strings.TrimSpace(strings.ToLower(supplier))
	for _, item := range links {
		if strings.TrimSpace(strings.ToLower(item)) == needle {
			return item, nil
		}
	}
	return links[0], nil
}

func (c *Client) fetchSupplierItemCodes(ctx context.Context, normalized, apiKey, apiSecret, supplier string, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 200
	}
	if limit > 500 {
		limit = 500
	}

	filtersJSON, _ := json.Marshal([][]interface{}{
		{"supplier", "=", supplier},
	})
	params := url.Values{}
	params.Set("parent", "Item")
	params.Set("fields", `["parent"]`)
	params.Set("filters", string(filtersJSON))
	params.Set("limit_page_length", fmt.Sprintf("%d", limit))

	var payload struct {
		Data []struct {
			Parent string `json:"parent"`
		} `json:"data"`
	}
	endpoint := normalized + "/api/resource/Item%20Supplier?" + params.Encode()
	if err := c.doJSON(ctx, endpoint, apiKey, apiSecret, &payload); err != nil {
		return nil, err
	}

	seen := map[string]struct{}{}
	result := make([]string, 0, len(payload.Data))
	for _, row := range payload.Data {
		code := strings.TrimSpace(row.Parent)
		if code == "" {
			continue
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		result = append(result, code)
	}
	return result, nil
}

func (c *Client) itemHasSupplier(ctx context.Context, normalized, apiKey, apiSecret, itemCode, supplier string) (bool, error) {
	var payload struct {
		Data struct {
			DefaultSupplier string `json:"default_supplier"`
			SupplierItems   []struct {
				Supplier string `json:"supplier"`
			} `json:"supplier_items"`
		} `json:"data"`
	}

	endpoint := normalized + "/api/resource/Item/" + url.PathEscape(strings.TrimSpace(itemCode))
	if err := c.doJSON(ctx, endpoint, apiKey, apiSecret, &payload); err != nil {
		return false, err
	}

	if strings.EqualFold(strings.TrimSpace(payload.Data.DefaultSupplier), strings.TrimSpace(supplier)) {
		return true, nil
	}
	for _, row := range payload.Data.SupplierItems {
		if strings.EqualFold(strings.TrimSpace(row.Supplier), strings.TrimSpace(supplier)) {
			return true, nil
		}
	}
	return false, nil
}

func (c *Client) searchItemsByCodes(ctx context.Context, normalized, apiKey, apiSecret string, itemCodes []string, query string, limit int) ([]Item, error) {
	if len(itemCodes) == 0 {
		return []Item{}, nil
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	if len(itemCodes) > 200 {
		itemCodes = itemCodes[:200]
	}

	filtersJSON, _ := json.Marshal([][]interface{}{
		{"disabled", "=", 0},
		{"is_stock_item", "=", 1},
		{"name", "in", itemCodes},
	})

	params := url.Values{}
	params.Set("fields", `["name","item_name","stock_uom"]`)
	params.Set("filters", string(filtersJSON))
	params.Set("limit_page_length", fmt.Sprintf("%d", limit))

	if trimmed := strings.TrimSpace(query); trimmed != "" {
		like := "%" + strings.ReplaceAll(trimmed, "\"", "") + "%"
		orFiltersJSON, _ := json.Marshal([][]interface{}{
			{"name", "like", like},
			{"item_name", "like", like},
		})
		params.Set("or_filters", string(orFiltersJSON))
	}

	var payload struct {
		Data []struct {
			Name     string `json:"name"`
			ItemName string `json:"item_name"`
			StockUOM string `json:"stock_uom"`
		} `json:"data"`
	}
	endpoint := normalized + "/api/resource/Item?" + params.Encode()
	if err := c.doJSON(ctx, endpoint, apiKey, apiSecret, &payload); err != nil {
		return nil, err
	}

	items := make([]Item, 0, len(payload.Data))
	for _, row := range payload.Data {
		displayName := row.ItemName
		if displayName == "" {
			displayName = row.Name
		}
		items = append(items, Item{
			Code: row.Name,
			Name: displayName,
			UOM:  row.StockUOM,
		})
	}
	return items, nil
}

func (c *Client) fetchWarehouseCompany(ctx context.Context, normalized, apiKey, apiSecret, warehouse string) (string, error) {
	var payload struct {
		Data struct {
			Company string `json:"company"`
		} `json:"data"`
	}
	endpoint := normalized + "/api/resource/Warehouse/" + url.PathEscape(strings.TrimSpace(warehouse))
	if err := c.doJSON(ctx, endpoint, apiKey, apiSecret, &payload); err != nil {
		return "", err
	}
	if strings.TrimSpace(payload.Data.Company) == "" {
		return "", fmt.Errorf("company not found for warehouse %s", warehouse)
	}
	return payload.Data.Company, nil
}

func (c *Client) fetchPurchaseReceiptDoc(ctx context.Context, normalized, apiKey, apiSecret, name string) (map[string]interface{}, error) {
	var payload struct {
		Data map[string]interface{} `json:"data"`
	}
	endpoint := normalized + "/api/resource/Purchase%20Receipt/" + url.PathEscape(strings.TrimSpace(name))
	if err := c.doJSON(ctx, endpoint, apiKey, apiSecret, &payload); err != nil {
		return nil, err
	}
	if len(payload.Data) == 0 {
		return nil, fmt.Errorf("purchase receipt %s not found", name)
	}
	return payload.Data, nil
}

func (c *Client) submitDoc(ctx context.Context, normalized, apiKey, apiSecret, doctype, name string) error {
	submitPayload := map[string]interface{}{
		"doc": map[string]interface{}{},
	}
	submitEndpoint := normalized + "/api/method/frappe.client.submit"
	docEndpoint := normalized + "/api/resource/" + url.PathEscape(doctype) + "/" + url.PathEscape(name)

	for attempt := 0; attempt < 2; attempt++ {
		var latest struct {
			Data map[string]interface{} `json:"data"`
		}
		if err := c.doJSON(ctx, docEndpoint, apiKey, apiSecret, &latest); err != nil {
			return err
		}
		if len(latest.Data) == 0 {
			return fmt.Errorf("%s %s not found before submit", doctype, name)
		}
		submitPayload["doc"] = latest.Data

		if err := c.doJSONRequest(ctx, http.MethodPost, submitEndpoint, apiKey, apiSecret, submitPayload, nil); err != nil {
			if attempt == 0 && strings.Contains(err.Error(), "TimestampMismatchError") {
				continue
			}
			return err
		}
		return nil
	}
	return nil
}

func mapPurchaseReceiptDraft(doc map[string]interface{}) (PurchaseReceiptDraft, error) {
	items, ok := doc["items"].([]interface{})
	if !ok || len(items) == 0 {
		return PurchaseReceiptDraft{}, fmt.Errorf("purchase receipt has no items")
	}
	firstItem, ok := items[0].(map[string]interface{})
	if !ok {
		return PurchaseReceiptDraft{}, fmt.Errorf("purchase receipt item payload is invalid")
	}

	itemCode := getStringValue(firstItem["item_code"])
	itemName := getStringValue(firstItem["item_name"])
	if itemName == "" {
		itemName = itemCode
	}

	uom := getStringValue(firstItem["uom"])
	if uom == "" {
		uom = getStringValue(firstItem["stock_uom"])
	}

	return PurchaseReceiptDraft{
		Name:                 getStringValue(doc["name"]),
		DocStatus:            int(getFloatValue(doc["docstatus"])),
		Status:               getStringValue(doc["status"]),
		Supplier:             getStringValue(doc["supplier"]),
		SupplierName:         getStringValue(doc["supplier_name"]),
		PostingDate:          getStringValue(doc["posting_date"]),
		SupplierDeliveryNote: getStringValue(doc["supplier_delivery_note"]),
		ItemCode:             itemCode,
		ItemName:             itemName,
		Qty:                  getFloatValue(firstItem["qty"]),
		UOM:                  uom,
		Warehouse:            getStringValue(firstItem["warehouse"]),
		Remarks:              getStringValue(doc["remarks"]),
	}, nil
}

const (
	accordAcceptedLinePrefix = "Accord Qabul:"
	accordReturnedLinePrefix = "Accord Qaytarildi:"
	accordReasonLinePrefix   = "Accord Sabab:"
	accordCommentLinePrefix  = "Accord Izoh:"
	accordSupplierAckPrefix  = "Accord Supplier Tasdiq:"
)

func buildAccordDecisionNote(draft PurchaseReceiptDraft, acceptedQty, returnedQty float64, returnReason, returnComment string) (string, error) {
	impliedReturnedQty := draft.Qty - acceptedQty
	if impliedReturnedQty < 0 {
		impliedReturnedQty = 0
	}
	trimmedComment := strings.TrimSpace(returnComment)
	if impliedReturnedQty <= 0 {
		return "", nil
	}

	if returnedQty < 0 {
		return "", fmt.Errorf("returned qty cannot be negative")
	}
	if returnedQty == 0 {
		returnedQty = impliedReturnedQty
	}
	if returnedQty-impliedReturnedQty > 0.0001 {
		return "", fmt.Errorf("returned qty cannot exceed sent minus accepted qty")
	}

	lines := []string{
		fmt.Sprintf("%s %.4f %s", accordAcceptedLinePrefix, acceptedQty, draft.UOM),
		fmt.Sprintf("%s %.4f %s", accordReturnedLinePrefix, returnedQty, draft.UOM),
	}
	if strings.TrimSpace(returnReason) != "" {
		lines = append(lines, accordReasonLinePrefix+" "+strings.TrimSpace(returnReason))
	}
	if trimmedComment != "" {
		lines = append(lines, accordCommentLinePrefix+" "+trimmedComment)
	}
	return strings.Join(lines, "\n"), nil
}

func upsertAccordDecisionInRemarks(existing, decision string) string {
	lines := strings.Split(strings.ReplaceAll(existing, "\r\n", "\n"), "\n")
	filtered := make([]string, 0, len(lines)+3)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, accordAcceptedLinePrefix) ||
			strings.HasPrefix(trimmed, accordReturnedLinePrefix) ||
			strings.HasPrefix(trimmed, accordReasonLinePrefix) ||
			strings.HasPrefix(trimmed, accordCommentLinePrefix) ||
			strings.HasPrefix(trimmed, accordSupplierAckPrefix) {
			continue
		}
		filtered = append(filtered, trimmed)
	}
	if strings.TrimSpace(decision) != "" {
		filtered = append(filtered, strings.Split(strings.TrimSpace(decision), "\n")...)
	}
	return strings.Join(filtered, "\n")
}

func ExtractAccordDecisionNote(remarks string) string {
	lines := strings.Split(strings.ReplaceAll(remarks, "\r\n", "\n"), "\n")
	result := make([]string, 0, 3)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, accordAcceptedLinePrefix):
			result = append(result, "Qabul: "+strings.TrimSpace(strings.TrimPrefix(trimmed, accordAcceptedLinePrefix)))
		case strings.HasPrefix(trimmed, accordReturnedLinePrefix):
			result = append(result, "Qaytarildi: "+strings.TrimSpace(strings.TrimPrefix(trimmed, accordReturnedLinePrefix)))
		case strings.HasPrefix(trimmed, accordReasonLinePrefix):
			result = append(result, "Sabab: "+strings.TrimSpace(strings.TrimPrefix(trimmed, accordReasonLinePrefix)))
		case strings.HasPrefix(trimmed, accordCommentLinePrefix):
			result = append(result, "Izoh: "+strings.TrimSpace(strings.TrimPrefix(trimmed, accordCommentLinePrefix)))
		case strings.HasPrefix(trimmed, accordSupplierAckPrefix):
			result = append(result, "Supplier tasdiqladi: "+strings.TrimSpace(strings.TrimPrefix(trimmed, accordSupplierAckPrefix)))
		}
	}
	return strings.Join(result, "\n")
}

func ExtractAccordDecisionQuantities(remarks string) (acceptedQty, returnedQty float64) {
	lines := strings.Split(strings.ReplaceAll(remarks, "\r\n", "\n"), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, accordAcceptedLinePrefix):
			value := strings.TrimSpace(strings.TrimPrefix(trimmed, accordAcceptedLinePrefix))
			fields := strings.Fields(value)
			if len(fields) > 0 {
				acceptedQty, _ = strconv.ParseFloat(fields[0], 64)
			}
		case strings.HasPrefix(trimmed, accordReturnedLinePrefix):
			value := strings.TrimSpace(strings.TrimPrefix(trimmed, accordReturnedLinePrefix))
			fields := strings.Fields(value)
			if len(fields) > 0 {
				returnedQty, _ = strconv.ParseFloat(fields[0], 64)
			}
		}
	}
	return acceptedQty, returnedQty
}

func UpsertSupplierAcknowledgmentInRemarks(existingNote, message string) string {
	lines := strings.Split(strings.ReplaceAll(existingNote, "\r\n", "\n"), "\n")
	filtered := make([]string, 0, len(lines)+1)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "Supplier tasdiqladi:") {
			continue
		}
		filtered = append(filtered, trimmed)
	}
	filtered = append(filtered, accordSupplierAckPrefix+" "+strings.TrimSpace(message))
	return strings.Join(filtered, "\n")
}

func (c *Client) UpdatePurchaseReceiptRemarks(ctx context.Context, baseURL, apiKey, apiSecret, name, remarks string) error {
	normalized, err := normalizeBaseURL(baseURL)
	if err != nil {
		return err
	}
	endpoint := normalized + "/api/resource/Purchase%20Receipt/" + url.PathEscape(strings.TrimSpace(name))
	return c.doJSONRequest(ctx, http.MethodPut, endpoint, apiKey, apiSecret, map[string]string{
		"remarks": strings.TrimSpace(remarks),
	}, nil)
}

func buildTelegramReceiptMarker(phone string, qty float64, now time.Time) string {
	normalizedPhone := strings.TrimSpace(phone)
	if normalizedPhone == "" {
		normalizedPhone = "unknown"
	}
	return fmt.Sprintf("%s%s:%s:%.4f", telegramReceiptMarkerPrefix, normalizedPhone, now.Format("20060102150405"), qty)
}

func ParseTelegramReceiptMarkerQty(marker string) (float64, bool) {
	trimmed := strings.TrimSpace(marker)
	if !strings.HasPrefix(trimmed, telegramReceiptMarkerPrefix) {
		return 0, false
	}
	parts := strings.Split(trimmed, ":")
	if len(parts) < 4 {
		return 0, false
	}
	value, err := strconv.ParseFloat(strings.TrimSpace(parts[len(parts)-1]), 64)
	if err != nil {
		return 0, false
	}
	return value, true
}

func getStringValue(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return ""
	}
}

func getFloatValue(value interface{}) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int32:
		return float64(typed)
	case int64:
		return float64(typed)
	default:
		return 0
	}
}
