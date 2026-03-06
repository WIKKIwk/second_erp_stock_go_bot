package admin

import (
	"fmt"
	"strings"
	"unicode"
)

type ContactKind string

const (
	ContactKindAdminka ContactKind = "adminka"
	ContactKindWerka   ContactKind = "werka"
)

func (s *Service) SaveContact(kind ContactKind, phone, name string) error {
	normalizedPhone, err := NormalizeContactPhone(phone)
	if err != nil {
		return err
	}
	normalizedName, err := NormalizeContactName(name)
	if err != nil {
		return err
	}

	phoneKey, nameKey, err := contactEnvKeys(kind)
	if err != nil {
		return err
	}
	if s.envPersister == nil {
		return fmt.Errorf("env persister is not configured")
	}

	return s.envPersister.Upsert(map[string]string{
		phoneKey: normalizedPhone,
		nameKey:  normalizedName,
	})
}

func contactEnvKeys(kind ContactKind) (phoneKey, nameKey string, err error) {
	switch kind {
	case ContactKindAdminka:
		return "ADMINKA_PHONE", "ADMINKA_NAME", nil
	case ContactKindWerka:
		return "WERKA_PHONE", "WERKA_NAME", nil
	default:
		return "", "", fmt.Errorf("unknown contact kind: %s", kind)
	}
}

func NormalizeContactName(input string) (string, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", fmt.Errorf("Ism bo'sh bo'lmasligi kerak")
	}
	return trimmed, nil
}

func NormalizeContactPhone(input string) (string, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", fmt.Errorf("Telefon raqam bo'sh bo'lmasligi kerak")
	}

	var digits strings.Builder
	for _, r := range trimmed {
		if r == '+' {
			continue
		}
		if !unicode.IsDigit(r) {
			return "", fmt.Errorf("Telefon raqam faqat '+' va raqamlardan iborat bo'lishi kerak")
		}
		digits.WriteRune(r)
	}

	value := digits.String()
	if len(value) < 9 {
		return "", fmt.Errorf("Telefon raqam kamida 9 xonali bo'lishi kerak")
	}
	if len(value) > 12 {
		return "", fmt.Errorf("Telefon raqam 12 xonadan oshmasligi kerak")
	}

	return "+" + value, nil
}
