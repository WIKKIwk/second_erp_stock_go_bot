package suplier

import (
	"strings"
	"testing"
)

func TestGenerateAccessCredentialsIsDeterministic(t *testing.T) {
	supplier := Supplier{Name: "Ali", Phone: "+998901234567"}

	first, err := GenerateAccessCredentials(supplier)
	if err != nil {
		t.Fatalf("GenerateAccessCredentials returned error: %v", err)
	}
	second, err := GenerateAccessCredentials(supplier)
	if err != nil {
		t.Fatalf("GenerateAccessCredentials returned error: %v", err)
	}

	if first != second {
		t.Fatalf("expected deterministic credentials, got %+v and %+v", first, second)
	}
	if len(first.Code) != 12 {
		t.Fatalf("expected 12-char code, got %q", first.Code)
	}
	if len(first.Secret) != 12 {
		t.Fatalf("expected 12-char secret, got %q", first.Secret)
	}
}

func TestSupplierAccessMessageOmitsSecret(t *testing.T) {
	supplier := Supplier{Name: "Ali", Phone: "+998901234567"}

	message, err := SupplierAccessMessage(supplier)
	if err != nil {
		t.Fatalf("SupplierAccessMessage returned error: %v", err)
	}
	if message == "" {
		t.Fatal("expected non-empty message")
	}
	if strings.Contains(message, "Secret:") {
		t.Fatalf("expected secret to be omitted, got %q", message)
	}
}
