package erpnext

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
)

type AuthInfo struct {
	Username string
	Roles    []string
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", fmt.Sprintf("token %s:%s", apiKey, apiSecret))
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return err
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
