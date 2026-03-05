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
	SearchItems(ctx context.Context, baseURL, apiKey, apiSecret, query string, limit int) ([]erpnext.Item, error)
	CreateAndSubmitStockEntry(ctx context.Context, baseURL, apiKey, apiSecret string, input erpnext.CreateStockEntryInput) (erpnext.StockEntryResult, error)
}

type Service struct {
	sessions               *SessionManager
	creds                  store.CredentialStore
	erp                    ERPAuthenticator
	defaultTargetWarehouse string
	defaultSourceWarehouse string
}

func NewService(sessions *SessionManager, creds store.CredentialStore, erp ERPAuthenticator, defaultTargetWarehouse, defaultSourceWarehouse string) *Service {
	return &Service{
		sessions:               sessions,
		creds:                  creds,
		erp:                    erp,
		defaultTargetWarehouse: strings.TrimSpace(defaultTargetWarehouse),
		defaultSourceWarehouse: strings.TrimSpace(defaultSourceWarehouse),
	}
}

func (s *Service) HandleStart(chatID int64) string {
	if creds, ok := s.creds.Get(chatID); ok {
		rolesText := "aniqlanmadi"
		if len(creds.Roles) > 0 {
			rolesText = strings.Join(creds.Roles, ", ")
		}
		return fmt.Sprintf("Siz allaqachon tizimga kirgansiz.\nERP foydalanuvchi: %s\nRollar: %s\nQayta login uchun /login buyrug'ini bering.", creds.Username, rolesText)
	}

	return "Assalomu alaykum. Bu bot ERPNext stock entry jarayonlari uchun ishlatiladi.\nDavom etish uchun /login buyrug'ini yuboring."
}

func (s *Service) HandleLoginCommand(chatID int64) string {
	s.sessions.StartLogin(chatID)
	return "Login boshlandi.\n1/3: ERPNext URL kiriting. Masalan: https://erp.example.com"
}

func (s *Service) HandleText(ctx context.Context, chatID int64, text string) string {
	session, ok := s.sessions.Get(chatID)
	if !ok || session.Step == LoginStepNone {
		return "Iltimos, avval /login buyrug'ini yuboring."
	}

	value := strings.TrimSpace(text)
	switch session.Step {
	case LoginStepAwaitingURL:
		normalized, err := validateAndNormalizeURL(value)
		if err != nil {
			return fmt.Sprintf("Noto'g'ri URL: %v\nQayta kiriting. Masalan: https://erp.example.com", err)
		}
		session.BaseURL = normalized
		session.Step = LoginStepAwaitingAPIKey
		s.sessions.Upsert(chatID, session)
		return "2/3: API Key kiriting."

	case LoginStepAwaitingAPIKey:
		if value == "" {
			return "API Key bo'sh bo'lmasligi kerak. Qayta kiriting."
		}
		session.APIKey = value
		session.Step = LoginStepAwaitingAPISecret
		s.sessions.Upsert(chatID, session)
		return "3/3: API Secret kiriting."

	case LoginStepAwaitingAPISecret:
		if value == "" {
			return "API Secret bo'sh bo'lmasligi kerak. Qayta kiriting."
		}

		authInfo, err := s.erp.ValidateCredentials(ctx, session.BaseURL, session.APIKey, value)
		if err != nil {
			s.sessions.Clear(chatID)
			return fmt.Sprintf("Kirish muvaffaqiyatsiz. URL/API Key/API Secret noto'g'ri bo'lishi mumkin.\nTexnik sabab: %v\nQayta urinish uchun /login buyrug'ini yuboring.", err)
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

		return fmt.Sprintf("Kirish muvaffaqiyatli.\nERP foydalanuvchi: %s\nRollar: %s", authInfo.Username, rolesText)
	default:
		return "Noma'lum holat. Qayta boshlash uchun /login yuboring."
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
