package bot

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"erpnext_stock_telegram/internal/erpnext"
	"erpnext_stock_telegram/internal/store"
)

type ERPAuthenticator interface {
	ValidateCredentials(ctx context.Context, baseURL, apiKey, apiSecret string) (erpnext.AuthInfo, error)
}

type Service struct {
	sessions *SessionManager
	creds    store.CredentialStore
	erp      ERPAuthenticator
}

func NewService(sessions *SessionManager, creds store.CredentialStore, erp ERPAuthenticator) *Service {
	return &Service{sessions: sessions, creds: creds, erp: erp}
}

func (s *Service) HandleStart(chatID int64) []string {
	if creds, ok := s.creds.Get(chatID); ok {
		rolesText := "aniqlanmadi"
		if len(creds.Roles) > 0 {
			rolesText = strings.Join(creds.Roles, ", ")
		}
		return []string{
			fmt.Sprintf("Siz allaqachon tizimga kirgansiz.\nERP foydalanuvchi: %s\nRollar: %s\nQayta login uchun /login buyrug'ini bering.", creds.Username, rolesText),
		}
	}

	return []string{
		"Assalomu alaykum. Bu bot ERPNext stock entry jarayonlari uchun ishlatiladi.",
		"Davom etish uchun /login buyrug'ini yuboring.",
	}
}

func (s *Service) HandleLoginCommand(chatID int64) []string {
	s.sessions.StartLogin(chatID)
	return []string{
		"Login boshlandi.",
		"1/3: ERPNext URL kiriting. Masalan: https://erp.example.com",
	}
}

func (s *Service) HandleText(ctx context.Context, chatID int64, text string) []string {
	session, ok := s.sessions.Get(chatID)
	if !ok || session.Step == LoginStepNone {
		if normalized, err := validateAndNormalizeURL(strings.TrimSpace(text)); err == nil {
			s.sessions.Upsert(chatID, LoginSession{
				Step:    LoginStepAwaitingAPIKey,
				BaseURL: normalized,
			})
			return []string{
				"Login sessiyasi qayta tiklandi.",
				"2/3: API Key kiriting.",
			}
		}
		return []string{"Iltimos, avval /login buyrug'ini yuboring."}
	}

	value := strings.TrimSpace(text)
	switch session.Step {
	case LoginStepAwaitingURL:
		normalized, err := validateAndNormalizeURL(value)
		if err != nil {
			return []string{fmt.Sprintf("Noto'g'ri URL: %v", err), "Qayta kiriting. Masalan: https://erp.example.com"}
		}
		session.BaseURL = normalized
		session.Step = LoginStepAwaitingAPIKey
		s.sessions.Upsert(chatID, session)
		return []string{"2/3: API Key kiriting."}

	case LoginStepAwaitingAPIKey:
		if value == "" {
			return []string{"API Key bo'sh bo'lmasligi kerak. Qayta kiriting."}
		}
		session.APIKey = value
		session.Step = LoginStepAwaitingAPISecret
		s.sessions.Upsert(chatID, session)
		return []string{"3/3: API Secret kiriting."}

	case LoginStepAwaitingAPISecret:
		if value == "" {
			return []string{"API Secret bo'sh bo'lmasligi kerak. Qayta kiriting."}
		}

		authInfo, err := s.erp.ValidateCredentials(ctx, session.BaseURL, session.APIKey, value)
		if err != nil {
			return []string{
				"Kirish muvaffaqiyatsiz. URL/API Key/API Secret noto'g'ri bo'lishi mumkin.",
				fmt.Sprintf("Texnik sabab: %v", err),
				"Qayta urinish uchun /login buyrug'ini yuboring.",
			}
		}

		s.creds.Save(chatID, store.Credentials{
			BaseURL:   session.BaseURL,
			APIKey:    session.APIKey,
			APISecret: value,
			Username:  authInfo.Username,
			Roles:     authInfo.Roles,
			UpdatedAt: time.Now(),
		})
		s.sessions.Clear(chatID)

		rolesText := "aniqlanmadi"
		if len(authInfo.Roles) > 0 {
			rolesText = strings.Join(authInfo.Roles, ", ")
		}

		return []string{fmt.Sprintf("Kirish muvaffaqiyatli.\nERP foydalanuvchi: %s\nRollar: %s", authInfo.Username, rolesText)}
	default:
		return []string{"Noma'lum holat. Qayta boshlash uchun /login yuboring."}
	}
}

func validateAndNormalizeURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("URL http:// yoki https:// bilan boshlanishi kerak")
	}
	if u.Host == "" {
		return "", fmt.Errorf("host topilmadi")
	}
	u.RawQuery = ""
	u.Fragment = ""
	return strings.TrimRight(u.String(), "/"), nil
}
