package bot

import "testing"

func TestSupplierNameMatchesAllowsSmallTypos(t *testing.T) {
	if !supplierNameMatches("Abdulloh", "Abdullox") {
		t.Fatal("expected close names to match")
	}
	if supplierNameMatches("Ali", "Sardor") {
		t.Fatal("expected different names not to match")
	}
}

func TestValidateStrongPassword(t *testing.T) {
	if err := validateStrongPassword("abc12345"); err != nil {
		t.Fatalf("expected password to pass, got %v", err)
	}
	if err := validateStrongPassword("abcdefghi"); err == nil {
		t.Fatal("expected password without digits to fail")
	}
	if err := validateStrongPassword("12345678"); err == nil {
		t.Fatal("expected password without letters to fail")
	}
}
