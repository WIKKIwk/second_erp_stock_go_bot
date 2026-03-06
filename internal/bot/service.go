package bot

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"strings"
	"sync"
	"time"

	adminsvc "erpnext_stock_telegram/internal/admin"
	"erpnext_stock_telegram/internal/erpnext"
	"erpnext_stock_telegram/internal/store"
	"erpnext_stock_telegram/internal/suplier"
)

type ERPAuthenticator interface {
	ValidateCredentials(ctx context.Context, baseURL, apiKey, apiSecret string) (erpnext.AuthInfo, error)
	SearchItems(ctx context.Context, baseURL, apiKey, apiSecret, query string, limit int) ([]erpnext.Item, error)
	SearchWarehouses(ctx context.Context, baseURL, apiKey, apiSecret, query string, limit int) ([]erpnext.Warehouse, error)
	SearchUOMs(ctx context.Context, baseURL, apiKey, apiSecret, query string, limit int) ([]erpnext.UOM, error)
	CreateAndSubmitStockEntry(ctx context.Context, baseURL, apiKey, apiSecret string, input erpnext.CreateStockEntryInput) (erpnext.StockEntryResult, error)
}

type EnvPersister interface {
	Upsert(values map[string]string) error
}

type AdminManager interface {
	IsConfigured() bool
	ValidatePassword(input string) bool
	SetPassword(input string) error
}

type SupplierManager interface {
	Add(ctx context.Context, name, phone string) (suplier.Supplier, error)
}

type Service struct {
	sessions               *SessionManager
	creds                  store.CredentialStore
	erp                    ERPAuthenticator
	admin                  AdminManager
	supplier               SupplierManager
	envPersister           EnvPersister
	settingsPassword       string
	defaultUOM             string
	defaultTargetWarehouse string
	defaultSourceWarehouse string
	defaultBaseURL         string
	defaultAPIKey          string
	defaultAPISecret       string
	mu                     sync.RWMutex
}

func NewService(
	sessions *SessionManager,
	creds store.CredentialStore,
	erp ERPAuthenticator,
	admin AdminManager,
	supplier SupplierManager,
	settingsPassword string,
	defaultTargetWarehouse string,
	defaultSourceWarehouse string,
	defaultUOM string,
	defaultBaseURL string,
	defaultAPIKey string,
	defaultAPISecret string,
	envPersister EnvPersister,
) *Service {
	uom := strings.TrimSpace(defaultUOM)
	if uom == "" {
		uom = "Kg"
	}
	if admin == nil {
		admin = adminsvc.NewService("", envPersister)
	}
	return &Service{
		sessions:               sessions,
		creds:                  creds,
		erp:                    erp,
		admin:                  admin,
		supplier:               supplier,
		envPersister:           envPersister,
		settingsPassword:       strings.TrimSpace(settingsPassword),
		defaultTargetWarehouse: strings.TrimSpace(defaultTargetWarehouse),
		defaultSourceWarehouse: strings.TrimSpace(defaultSourceWarehouse),
		defaultUOM:             uom,
		defaultBaseURL:         strings.TrimSpace(defaultBaseURL),
		defaultAPIKey:          strings.TrimSpace(defaultAPIKey),
		defaultAPISecret:       strings.TrimSpace(defaultAPISecret),
	}
}

func (s *Service) AddSupplier(ctx context.Context, name, phone string) (suplier.Supplier, error) {
	if s.supplier == nil {
		return suplier.Supplier{}, fmt.Errorf("supplier service is not configured")
	}
	return s.supplier.Add(ctx, name, phone)
}

func (s *Service) IsAdminConfigured() bool {
	return s.admin != nil && s.admin.IsConfigured()
}

func (s *Service) IsAdminPasswordValid(input string) bool {
	return s.admin != nil && s.admin.ValidatePassword(input)
}

func (s *Service) SetAdminPassword(input string) error {
	if s.admin == nil {
		return fmt.Errorf("admin service is not configured")
	}
	return s.admin.SetPassword(input)
}

func (s *Service) IsSettingsPasswordValid(input string) bool {
	if s.settingsPassword == "" {
		return false
	}
	return strings.TrimSpace(input) == s.settingsPassword
}

func (s *Service) SetDefaultWarehouse(warehouse string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	trimmed := strings.TrimSpace(warehouse)
	s.defaultTargetWarehouse = trimmed
	s.defaultSourceWarehouse = trimmed
	s.persistEnv(map[string]string{
		"ERP_DEFAULT_TARGET_WAREHOUSE": trimmed,
		"ERP_DEFAULT_SOURCE_WAREHOUSE": trimmed,
	})
}

func (s *Service) SetDefaultUOM(uom string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	trimmed := strings.TrimSpace(uom)
	if trimmed != "" {
		s.defaultUOM = trimmed
		s.persistEnv(map[string]string{"ERP_DEFAULT_UOM": trimmed})
	}
}

func (s *Service) EnsureCredentials(principalID int64) bool {
	if _, ok := s.creds.Get(principalID); ok {
		return true
	}

	s.mu.RLock()
	baseURL := s.defaultBaseURL
	apiKey := s.defaultAPIKey
	apiSecret := s.defaultAPISecret
	s.mu.RUnlock()

	if baseURL == "" || apiKey == "" || apiSecret == "" {
		return false
	}

	s.creds.Save(principalID, store.Credentials{
		BaseURL:   baseURL,
		APIKey:    apiKey,
		APISecret: apiSecret,
		UpdatedAt: time.Now(),
	})
	return true
}

func (s *Service) Defaults() (targetWarehouse, sourceWarehouse, uom string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.defaultTargetWarehouse, s.defaultSourceWarehouse, s.defaultUOM
}

func (s *Service) HandleStart(chatID int64) string {
	if _, ok := s.creds.Get(chatID); !ok {
		s.EnsureCredentials(chatID)
	}

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
		s.persistEnv(map[string]string{"ERP_URL": normalized})
		session.Step = LoginStepAwaitingAPIKey
		s.sessions.Upsert(chatID, session)
		return "2/3: API Key kiriting."

	case LoginStepAwaitingAPIKey:
		if value == "" {
			return "API Key bo'sh bo'lmasligi kerak. Qayta kiriting."
		}
		session.APIKey = value
		s.persistEnv(map[string]string{"ERP_API_KEY": value})
		session.Step = LoginStepAwaitingAPISecret
		s.sessions.Upsert(chatID, session)
		return "3/3: API Secret kiriting."

	case LoginStepAwaitingAPISecret:
		if value == "" {
			return "API Secret bo'sh bo'lmasligi kerak. Qayta kiriting."
		}
		s.persistEnv(map[string]string{"ERP_API_SECRET": value})

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
		s.mu.Lock()
		s.defaultBaseURL = session.BaseURL
		s.defaultAPIKey = session.APIKey
		s.defaultAPISecret = value
		s.mu.Unlock()
		s.persistEnv(map[string]string{
			"ERP_URL":        session.BaseURL,
			"ERP_API_KEY":    session.APIKey,
			"ERP_API_SECRET": value,
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

func (s *Service) persistEnv(values map[string]string) {
	if s.envPersister == nil || len(values) == 0 {
		return
	}
	if err := s.envPersister.Upsert(values); err != nil {
		log.Printf("failed to persist .env values: %v", err)
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
