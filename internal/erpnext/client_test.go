package erpnext

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestValidateCredentialsSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "token key:secret" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		switch r.URL.Path {
		case "/api/method/frappe.auth.get_logged_user":
			_, _ = w.Write([]byte(`{"message":"user@example.com"}`))
		case "/api/method/frappe.core.doctype.user.user.get_roles":
			_, _ = w.Write([]byte(`{"message":["Stock User","Material Manager"]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(&http.Client{Timeout: 3 * time.Second})
	result, err := client.ValidateCredentials(context.Background(), server.URL, "key", "secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Username != "user@example.com" {
		t.Fatalf("unexpected username: %q", result.Username)
	}
	if len(result.Roles) != 2 {
		t.Fatalf("expected 2 roles, got: %v", result.Roles)
	}
}

func TestValidateCredentialsFallbackRoles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/method/frappe.auth.get_logged_user":
			_, _ = w.Write([]byte(`{"message":"user@example.com"}`))
		case "/api/method/frappe.core.doctype.user.user.get_roles":
			http.NotFound(w, r)
		case "/api/resource/User/user@example.com":
			_, _ = w.Write([]byte(`{"data":{"roles":[{"role":"Stock User"}]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(&http.Client{Timeout: 3 * time.Second})
	result, err := client.ValidateCredentials(context.Background(), server.URL, "key", "secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Roles) != 1 || result.Roles[0] != "Stock User" {
		t.Fatalf("unexpected roles: %v", result.Roles)
	}
}

func TestValidateCredentialsInvalidAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "invalid token", http.StatusUnauthorized)
	}))
	defer server.Close()

	client := NewClient(&http.Client{Timeout: 3 * time.Second})
	_, err := client.ValidateCredentials(context.Background(), server.URL, "bad", "bad")
	if err == nil {
		t.Fatal("expected error for invalid auth")
	}
}
