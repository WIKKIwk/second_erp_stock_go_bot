package erpnext

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
)

type AuthInfo struct {
	Username string
	Roles    []string
}

type Item struct {
	Code string
	Name string
	UOM  string
}

type Warehouse struct {
	Name string
}

type UOM struct {
	Name string
}

type CreateStockEntryInput struct {
	EntryType       string
	ItemCode        string
	Qty             float64
	UOM             string
	SourceWarehouse string
	TargetWarehouse string
}

type StockEntryResult struct {
	Name string
}

type Client struct {
	httpClient *http.Client
}

func NewClient(httpClient *http.Client) *Client {
	return &Client{httpClient: httpClient}
}

func (c *Client) ValidateCredentials(ctx context.Context, baseURL, apiKey, apiSecret string) (AuthInfo, error) {
	normalized, err := normalizeBaseURL(baseURL)
	if err != nil {
		return AuthInfo{}, err
	}

	username, err := c.fetchLoggedUser(ctx, normalized, apiKey, apiSecret)
	if err != nil {
		return AuthInfo{}, err
	}

	roles, err := c.fetchRoles(ctx, normalized, username, apiKey, apiSecret)
	if err != nil {
		roles = nil
	}

	return AuthInfo{Username: username, Roles: roles}, nil
}

func (c *Client) SearchItems(ctx context.Context, baseURL, apiKey, apiSecret, query string, limit int) ([]Item, error) {
	normalized, err := normalizeBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	filtersJSON, _ := json.Marshal([][]interface{}{
		{"disabled", "=", 0},
		{"is_stock_item", "=", 1},
	})

	params := url.Values{}
	params.Set("fields", `["name","item_name","stock_uom"]`)
	params.Set("filters", string(filtersJSON))
	params.Set("limit_page_length", strconv.Itoa(limit))

	if trimmed := strings.TrimSpace(query); trimmed != "" {
		like := "%" + strings.ReplaceAll(trimmed, "\"", "") + "%"
		orFiltersJSON, _ := json.Marshal([][]interface{}{
			{"name", "like", like},
			{"item_name", "like", like},
		})
		params.Set("or_filters", string(orFiltersJSON))
	}

	endpoint := normalized + "/api/resource/Item?" + params.Encode()
	var payload struct {
		Data []struct {
			Name     string `json:"name"`
			ItemName string `json:"item_name"`
			StockUOM string `json:"stock_uom"`
		} `json:"data"`
	}
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

func (c *Client) SearchWarehouses(ctx context.Context, baseURL, apiKey, apiSecret, query string, limit int) ([]Warehouse, error) {
	normalized, err := normalizeBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	filtersJSON, _ := json.Marshal([][]interface{}{
		{"disabled", "=", 0},
	})

	params := url.Values{}
	params.Set("fields", `["name"]`)
	params.Set("filters", string(filtersJSON))
	params.Set("limit_page_length", strconv.Itoa(limit))

	if trimmed := strings.TrimSpace(query); trimmed != "" {
		like := "%" + strings.ReplaceAll(trimmed, "\"", "") + "%"
		orFiltersJSON, _ := json.Marshal([][]interface{}{
			{"name", "like", like},
			{"warehouse_name", "like", like},
		})
		params.Set("or_filters", string(orFiltersJSON))
	}

	endpoint := normalized + "/api/resource/Warehouse?" + params.Encode()
	var payload struct {
		Data []struct {
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := c.doJSON(ctx, endpoint, apiKey, apiSecret, &payload); err != nil {
		return nil, err
	}

	warehouses := make([]Warehouse, 0, len(payload.Data))
	for _, row := range payload.Data {
		if row.Name != "" {
			warehouses = append(warehouses, Warehouse{Name: row.Name})
		}
	}
	return warehouses, nil
}

func (c *Client) SearchUOMs(ctx context.Context, baseURL, apiKey, apiSecret, query string, limit int) ([]UOM, error) {
	normalized, err := normalizeBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	params := url.Values{}
	params.Set("fields", `["name"]`)
	params.Set("limit_page_length", strconv.Itoa(limit))

	if trimmed := strings.TrimSpace(query); trimmed != "" {
		like := "%" + strings.ReplaceAll(trimmed, "\"", "") + "%"
		filtersJSON, _ := json.Marshal([][]interface{}{
			{"name", "like", like},
		})
		params.Set("filters", string(filtersJSON))
	}

	endpoint := normalized + "/api/resource/UOM?" + params.Encode()
	var payload struct {
		Data []struct {
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := c.doJSON(ctx, endpoint, apiKey, apiSecret, &payload); err != nil {
		return nil, err
	}

	uoms := make([]UOM, 0, len(payload.Data))
	for _, row := range payload.Data {
		if row.Name != "" {
			uoms = append(uoms, UOM{Name: row.Name})
		}
	}
	return uoms, nil
}

func (c *Client) CreateAndSubmitStockEntry(ctx context.Context, baseURL, apiKey, apiSecret string, input CreateStockEntryInput) (StockEntryResult, error) {
	normalized, err := normalizeBaseURL(baseURL)
	if err != nil {
		return StockEntryResult{}, err
	}
	if input.Qty <= 0 {
		return StockEntryResult{}, fmt.Errorf("qty must be greater than 0")
	}
	if input.ItemCode == "" {
		return StockEntryResult{}, fmt.Errorf("item code is required")
	}
	if input.EntryType == "" {
		return StockEntryResult{}, fmt.Errorf("entry type is required")
	}
	if input.UOM == "" {
		input.UOM = "Kg"
	}

	itemRow := map[string]interface{}{
		"item_code":         input.ItemCode,
		"qty":               input.Qty,
		"uom":               input.UOM,
		"stock_uom":         input.UOM,
		"conversion_factor": 1,
	}

	payload := map[string]interface{}{
		"stock_entry_type": input.EntryType,
		"items":            []map[string]interface{}{itemRow},
	}

	switch input.EntryType {
	case "Material Receipt":
		if strings.TrimSpace(input.TargetWarehouse) == "" {
			return StockEntryResult{}, fmt.Errorf("target warehouse is required for Material Receipt")
		}
		payload["to_warehouse"] = input.TargetWarehouse
		itemRow["t_warehouse"] = input.TargetWarehouse
	case "Material Issue":
		if strings.TrimSpace(input.SourceWarehouse) == "" {
			return StockEntryResult{}, fmt.Errorf("source warehouse is required for Material Issue")
		}
		payload["from_warehouse"] = input.SourceWarehouse
		itemRow["s_warehouse"] = input.SourceWarehouse
	default:
		return StockEntryResult{}, fmt.Errorf("unsupported stock entry type: %s", input.EntryType)
	}

	var createResp struct {
		Data struct {
			Name string `json:"name"`
		} `json:"data"`
	}
	createEndpoint := normalized + "/api/resource/Stock Entry"
	if err := c.doJSONRequest(ctx, http.MethodPost, createEndpoint, apiKey, apiSecret, payload, &createResp); err != nil {
		return StockEntryResult{}, err
	}
	if createResp.Data.Name == "" {
		return StockEntryResult{}, fmt.Errorf("stock entry create response did not return name")
	}

	submitPayload := map[string]interface{}{
		"doc": map[string]interface{}{
			"doctype": "Stock Entry",
			"name":    createResp.Data.Name,
		},
	}
	submitEndpoint := normalized + "/api/method/frappe.client.submit"
	if err := c.doJSONRequest(ctx, http.MethodPost, submitEndpoint, apiKey, apiSecret, submitPayload, nil); err != nil {
		return StockEntryResult{}, err
	}

	return StockEntryResult{Name: createResp.Data.Name}, nil
}

func (c *Client) fetchLoggedUser(ctx context.Context, baseURL, apiKey, apiSecret string) (string, error) {
	type response struct {
		Message string `json:"message"`
	}

	var payload response
	endpoint := baseURL + "/api/method/frappe.auth.get_logged_user"
	if err := c.doJSON(ctx, endpoint, apiKey, apiSecret, &payload); err != nil {
		return "", fmt.Errorf("ERPNext authentication failed: %w", err)
	}
	if payload.Message == "" {
		return "", fmt.Errorf("ERPNext authentication failed: empty user")
	}
	return payload.Message, nil
}

func (c *Client) fetchRoles(ctx context.Context, baseURL, username, apiKey, apiSecret string) ([]string, error) {
	type roleMethodResponse struct {
		Message []string `json:"message"`
	}

	methodEndpoint := baseURL + "/api/method/frappe.core.doctype.user.user.get_roles"
	var methodPayload roleMethodResponse
	if err := c.doJSON(ctx, methodEndpoint, apiKey, apiSecret, &methodPayload); err == nil && len(methodPayload.Message) > 0 {
		return methodPayload.Message, nil
	}

	type userDocResponse struct {
		Data struct {
			Roles []struct {
				Role string `json:"role"`
			} `json:"roles"`
		} `json:"data"`
	}

	fields := `["name","roles"]`
	resourceEndpoint := fmt.Sprintf(
		"%s/api/resource/User/%s?fields=%s",
		baseURL,
		url.PathEscape(username),
		url.QueryEscape(fields),
	)

	var resourcePayload userDocResponse
	if err := c.doJSON(ctx, resourceEndpoint, apiKey, apiSecret, &resourcePayload); err != nil {
		return nil, err
	}

	roles := make([]string, 0, len(resourcePayload.Data.Roles))
	for _, item := range resourcePayload.Data.Roles {
		if item.Role != "" {
			roles = append(roles, item.Role)
		}
	}
	return roles, nil
}

func (c *Client) doJSON(ctx context.Context, endpoint, apiKey, apiSecret string, out interface{}) error {
	return c.doJSONRequest(ctx, http.MethodGet, endpoint, apiKey, apiSecret, nil, out)
}

func (c *Client) doJSONRequest(ctx context.Context, method, endpoint, apiKey, apiSecret string, body interface{}, out interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		bodyReader = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, bodyReader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", fmt.Sprintf("token %s:%s", apiKey, apiSecret))
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil && err != io.EOF {
			return err
		}
	}
	return nil
}

func normalizeBaseURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", fmt.Errorf("invalid ERPNext URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("ERPNext URL must start with http:// or https://")
	}
	if u.Host == "" {
		return "", fmt.Errorf("ERPNext URL host is missing")
	}
	u.RawQuery = ""
	u.Fragment = ""
	u.Path = strings.TrimSuffix(path.Clean(u.Path), "/")
	if u.Path == "." {
		u.Path = ""
	}
	return strings.TrimRight(u.String(), "/"), nil
}
