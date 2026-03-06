package suplier

import "testing"

func TestNormalizePhone(t *testing.T) {
	phone, err := NormalizePhone("+998901234567")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if phone != "+998901234567" {
		t.Fatalf("unexpected phone: %q", phone)
	}
}

func TestNormalizePhoneRejectsShortNumber(t *testing.T) {
	if _, err := NormalizePhone("+12345"); err == nil {
		t.Fatal("expected short phone error")
	}
}

func TestNormalizePhoneRejectsTooLongNumber(t *testing.T) {
	if _, err := NormalizePhone("+1234567890123"); err == nil {
		t.Fatal("expected too long phone error")
	}
}
