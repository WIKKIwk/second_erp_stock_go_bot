package admin

import "testing"

type fakeEnvPersister struct {
	values map[string]string
}

func (f *fakeEnvPersister) Upsert(values map[string]string) error {
	if f.values == nil {
		f.values = map[string]string{}
	}
	for key, value := range values {
		f.values[key] = value
	}
	return nil
}

func TestServiceSetPasswordPersistsValue(t *testing.T) {
	persister := &fakeEnvPersister{}
	service := NewService("", persister)

	if err := service.SetPassword("secret-123"); err != nil {
		t.Fatalf("SetPassword returned error: %v", err)
	}
	if !service.IsConfigured() {
		t.Fatal("expected service to be configured")
	}
	if !service.ValidatePassword("secret-123") {
		t.Fatal("expected password to validate")
	}
	if persister.values["ADMIN_PASSWORD"] != "secret-123" {
		t.Fatalf("expected ADMIN_PASSWORD to persist, got %q", persister.values["ADMIN_PASSWORD"])
	}
}

func TestServiceRejectsEmptyPassword(t *testing.T) {
	service := NewService("", nil)
	if err := service.SetPassword("   "); err == nil {
		t.Fatal("expected empty password error")
	}
}

func TestServiceSaveAdminkaContactPersistsValue(t *testing.T) {
	persister := &fakeEnvPersister{}
	service := NewService("", persister)

	if err := service.SaveContact(ContactKindAdminka, "+998901234567", "Aziza"); err != nil {
		t.Fatalf("SaveContact returned error: %v", err)
	}
	if persister.values["ADMINKA_PHONE"] != "+998901234567" {
		t.Fatalf("unexpected ADMINKA_PHONE: %q", persister.values["ADMINKA_PHONE"])
	}
	if persister.values["ADMINKA_NAME"] != "Aziza" {
		t.Fatalf("unexpected ADMINKA_NAME: %q", persister.values["ADMINKA_NAME"])
	}
}

func TestServiceSaveContactRejectsShortPhone(t *testing.T) {
	service := NewService("", &fakeEnvPersister{})
	if err := service.SaveContact(ContactKindWerka, "+12345", "Vali"); err == nil {
		t.Fatal("expected phone validation error")
	}
}

func TestServiceSaveWerkaContactPersistsValue(t *testing.T) {
	persister := &fakeEnvPersister{}
	service := NewService("", persister)

	if err := service.SaveContact(ContactKindWerka, "+998901111111", "Vali"); err != nil {
		t.Fatalf("SaveContact returned error: %v", err)
	}
	if persister.values["WERKA_PHONE"] != "+998901111111" {
		t.Fatalf("unexpected WERKA_PHONE: %q", persister.values["WERKA_PHONE"])
	}
	if persister.values["WERKA_NAME"] != "Vali" {
		t.Fatalf("unexpected WERKA_NAME: %q", persister.values["WERKA_NAME"])
	}
}
