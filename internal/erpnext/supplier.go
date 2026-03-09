package erpnext

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type CreateSupplierInput struct {
	Name  string
	Phone string
}

func (c *Client) SearchSuppliers(ctx context.Context, baseURL, apiKey, apiSecret, query string, limit int) ([]Supplier, error) {
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
	params.Set("fields", `["name","supplier_name","mobile_no"]`)
	params.Set("filters", string(filtersJSON))
	params.Set("limit_page_length", strconv.Itoa(limit))

	if trimmed := strings.TrimSpace(query); trimmed != "" {
		like := "%" + strings.ReplaceAll(trimmed, "\"", "") + "%"
		orFiltersJSON, _ := json.Marshal([][]interface{}{
			{"name", "like", like},
			{"supplier_name", "like", like},
			{"mobile_no", "like", like},
		})
		params.Set("or_filters", string(orFiltersJSON))
	}

	var payload struct {
		Data []struct {
			Name         string `json:"name"`
			SupplierName string `json:"supplier_name"`
			MobileNo     string `json:"mobile_no"`
		} `json:"data"`
	}

	endpoint := normalized + "/api/resource/Supplier?" + params.Encode()
	if err := c.doJSON(ctx, endpoint, apiKey, apiSecret, &payload); err != nil {
		return nil, err
	}

	items := make([]Supplier, 0, len(payload.Data))
	for _, row := range payload.Data {
		name := strings.TrimSpace(row.SupplierName)
		if name == "" {
			name = strings.TrimSpace(row.Name)
		}
		items = append(items, Supplier{
			ID:    strings.TrimSpace(row.Name),
			Name:  name,
			Phone: strings.TrimSpace(row.MobileNo),
		})
	}
	return items, nil
}

func (c *Client) EnsureSupplier(ctx context.Context, baseURL, apiKey, apiSecret string, input CreateSupplierInput) (Supplier, error) {
	normalized, err := normalizeBaseURL(baseURL)
	if err != nil {
		return Supplier{}, err
	}

	name := strings.TrimSpace(input.Name)
	if name == "" {
		return Supplier{}, fmt.Errorf("supplier name is required")
	}
	phone := strings.TrimSpace(input.Phone)

	existing, err := c.SearchSuppliers(ctx, normalized, apiKey, apiSecret, name, 20)
	if err != nil {
		return Supplier{}, err
	}
	for _, item := range existing {
		if strings.EqualFold(strings.TrimSpace(item.Name), name) ||
			(phone != "" && strings.EqualFold(strings.TrimSpace(item.Phone), phone)) {
			return item, nil
		}
	}

	payload := map[string]interface{}{
		"supplier_name":  name,
		"supplier_type":  "Company",
		"supplier_group": "Services",
		"mobile_no":      phone,
	}

	var response struct {
		Data struct {
			Name         string `json:"name"`
			SupplierName string `json:"supplier_name"`
			MobileNo     string `json:"mobile_no"`
		} `json:"data"`
	}
	endpoint := normalized + "/api/resource/Supplier"
	if err := c.doJSONRequest(ctx, http.MethodPost, endpoint, apiKey, apiSecret, payload, &response); err != nil {
		return Supplier{}, err
	}

	return Supplier{
		ID:    strings.TrimSpace(response.Data.Name),
		Name:  strings.TrimSpace(response.Data.SupplierName),
		Phone: strings.TrimSpace(response.Data.MobileNo),
	}, nil
}
