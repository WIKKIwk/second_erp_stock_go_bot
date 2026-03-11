package bot

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	adminsvc "erpnext_stock_telegram/internal/admin"
	"erpnext_stock_telegram/internal/erpnext"
	"erpnext_stock_telegram/internal/store"
	"erpnext_stock_telegram/internal/suplier"
)

type ERPAuthenticator interface {
	ValidateCredentials(ctx context.Context, baseURL, apiKey, apiSecret string) (erpnext.AuthInfo, error)
	SearchItems(ctx context.Context, baseURL, apiKey, apiSecret, query string, limit int) ([]erpnext.Item, error)
	SearchSupplierItems(ctx context.Context, baseURL, apiKey, apiSecret, supplier, query string, limit int) ([]erpnext.Item, error)
	SearchSuppliers(ctx context.Context, baseURL, apiKey, apiSecret, query string, limit int) ([]erpnext.Supplier, error)
	EnsureSupplier(ctx context.Context, baseURL, apiKey, apiSecret string, input erpnext.CreateSupplierInput) (erpnext.Supplier, error)
	SearchWarehouses(ctx context.Context, baseURL, apiKey, apiSecret, query string, limit int) ([]erpnext.Warehouse, error)
	SearchUOMs(ctx context.Context, baseURL, apiKey, apiSecret, query string, limit int) ([]erpnext.UOM, error)
	CreateAndSubmitStockEntry(ctx context.Context, baseURL, apiKey, apiSecret string, input erpnext.CreateStockEntryInput) (erpnext.StockEntryResult, error)
	CreateDraftPurchaseReceipt(ctx context.Context, baseURL, apiKey, apiSecret string, input erpnext.CreatePurchaseReceiptInput) (erpnext.PurchaseReceiptDraft, error)
	ListPendingPurchaseReceipts(ctx context.Context, baseURL, apiKey, apiSecret string, limit int) ([]erpnext.PurchaseReceiptDraft, error)
	GetPurchaseReceipt(ctx context.Context, baseURL, apiKey, apiSecret, name string) (erpnext.PurchaseReceiptDraft, error)
	ConfirmAndSubmitPurchaseReceipt(ctx context.Context, baseURL, apiKey, apiSecret, name string, acceptedQty, returnedQty float64, returnReason string) (erpnext.PurchaseReceiptSubmissionResult, error)
}

type EnvPersister interface {
	Upsert(values map[string]string) error
}

type AdminManager interface {
	IsConfigured() bool
	ValidatePassword(input string) bool
	SetPassword(input string) error
	SaveContact(kind adminsvc.ContactKind, phone, name string) error
}

type SupplierManager interface {
	Add(ctx context.Context, name, phone string) (suplier.Supplier, error)
	FindByPhone(ctx context.Context, phone string) (suplier.Supplier, bool, error)
	List(ctx context.Context) ([]suplier.Supplier, error)
}

type SupplierAuthManager interface {
	FindByPhone(ctx context.Context, phone string) (suplier.SupplierAuth, bool, error)
	Register(ctx context.Context, phone string, telegramUserID int64, password string) (suplier.SupplierAuth, error)
	Authenticate(ctx context.Context, phone string, telegramUserID int64, password string) (suplier.SupplierAuth, error)
}

type Service struct {
	sessions               *SessionManager
	creds                  store.CredentialStore
	erp                    ERPAuthenticator
	admin                  AdminManager
	supplier               SupplierManager
	supplierAuth           SupplierAuthManager
	envPersister           EnvPersister
	settingsPassword       string
	defaultUOM             string
	defaultTargetWarehouse string
	defaultSourceWarehouse string
	defaultBaseURL         string
	defaultAPIKey          string
	defaultAPISecret       string
	adminkaPhone           string
	adminkaName            string
	werkaPhone             string
	werkaName              string
	werkaTelegramID        int64
	mu                     sync.RWMutex
}

func NewService(
	sessions *SessionManager,
	creds store.CredentialStore,
	erp ERPAuthenticator,
	admin AdminManager,
	supplier SupplierManager,
	supplierAuth SupplierAuthManager,
	settingsPassword string,
	defaultTargetWarehouse string,
	defaultSourceWarehouse string,
	defaultUOM string,
	defaultBaseURL string,
	defaultAPIKey string,
	defaultAPISecret string,
	adminkaPhone string,
	adminkaName string,
	werkaPhone string,
	werkaName string,
	werkaTelegramID int64,
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
		supplierAuth:           supplierAuth,
		envPersister:           envPersister,
		settingsPassword:       strings.TrimSpace(settingsPassword),
		defaultTargetWarehouse: strings.TrimSpace(defaultTargetWarehouse),
		defaultSourceWarehouse: strings.TrimSpace(defaultSourceWarehouse),
		defaultUOM:             uom,
		defaultBaseURL:         strings.TrimSpace(defaultBaseURL),
		defaultAPIKey:          strings.TrimSpace(defaultAPIKey),
		defaultAPISecret:       strings.TrimSpace(defaultAPISecret),
		adminkaPhone:           normalizePhoneForMatch(adminkaPhone),
		adminkaName:            strings.TrimSpace(adminkaName),
		werkaPhone:             normalizePhoneForMatch(werkaPhone),
		werkaName:              strings.TrimSpace(werkaName),
		werkaTelegramID:        werkaTelegramID,
	}
}

func (s *Service) AddSupplier(ctx context.Context, name, phone string) (suplier.Supplier, error) {
	if s.supplier == nil {
		return suplier.Supplier{}, fmt.Errorf("supplier service is not configured")
	}
	return s.supplier.Add(ctx, name, phone)
}

func (s *Service) AddSupplierWithERP(ctx context.Context, principalID int64, name, phone string) (suplier.Supplier, error) {
	baseURL, apiKey, apiSecret, ok := s.erpCredentials(principalID)
	if !ok {
		return suplier.Supplier{}, fmt.Errorf("Iltimos, avval /login qiling.")
	}

	created, err := s.erp.EnsureSupplier(ctx, baseURL, apiKey, apiSecret, erpnext.CreateSupplierInput{
		Name:  name,
		Phone: phone,
	})
	if err != nil {
		return suplier.Supplier{}, err
	}

	return suplier.Supplier{
		Ref:   created.ID,
		Name:  created.Name,
		Phone: created.Phone,
	}, nil
}

func (s *Service) FindSupplierByPhone(ctx context.Context, principalID int64, phone string) (suplier.Supplier, bool, error) {
	normalizedPhone, err := suplier.NormalizePhone(phone)
	if err != nil {
		return suplier.Supplier{}, false, err
	}

	baseURL, apiKey, apiSecret, ok := s.erpCredentials(principalID)
	if !ok || s.erp == nil {
		return suplier.Supplier{}, false, fmt.Errorf("Iltimos, avval /login qiling.")
	}

	items, err := s.erp.SearchSuppliers(ctx, baseURL, apiKey, apiSecret, normalizedPhone, 20)
	if err != nil {
		return suplier.Supplier{}, false, err
	}
	for _, item := range items {
		if !strings.EqualFold(strings.TrimSpace(item.Phone), normalizedPhone) {
			continue
		}
		found := suplier.Supplier{
			Ref:   item.ID,
			Name:  item.Name,
			Phone: item.Phone,
		}
		return found, true, nil
	}
	return suplier.Supplier{}, false, nil
}

func (s *Service) erpCredentials(principalID int64) (string, string, string, bool) {
	if s.creds != nil {
		if creds, ok := s.creds.Get(principalID); ok {
			return creds.BaseURL, creds.APIKey, creds.APISecret, true
		}
	}
	if s.defaultBaseURL != "" && s.defaultAPIKey != "" && s.defaultAPISecret != "" {
		return s.defaultBaseURL, s.defaultAPIKey, s.defaultAPISecret, true
	}
	return "", "", "", false
}

func (s *Service) ListSuppliers(ctx context.Context) ([]suplier.Supplier, error) {
	baseURL, apiKey, apiSecret, ok := s.erpCredentials(0)
	if !ok || s.erp == nil {
		return nil, fmt.Errorf("Iltimos, avval /login qiling.")
	}
	rows, err := s.erp.SearchSuppliers(ctx, baseURL, apiKey, apiSecret, "", 100)
	if err != nil {
		return nil, err
	}
	items := make([]suplier.Supplier, 0, len(rows))
	for _, item := range rows {
		items = append(items, suplier.Supplier{
			Ref:   item.ID,
			Name:  item.Name,
			Phone: item.Phone,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})
	return items, nil
}

func (s *Service) FindSupplierAuthByPhone(ctx context.Context, phone string) (suplier.SupplierAuth, bool, error) {
	if s.supplierAuth == nil {
		return suplier.SupplierAuth{}, false, fmt.Errorf("supplier auth service is not configured")
	}
	return s.supplierAuth.FindByPhone(ctx, phone)
}

func (s *Service) RegisterSupplierAuth(ctx context.Context, phone string, telegramUserID int64, password string) (suplier.SupplierAuth, error) {
	if s.supplierAuth == nil {
		return suplier.SupplierAuth{}, fmt.Errorf("supplier auth service is not configured")
	}
	return s.supplierAuth.Register(ctx, phone, telegramUserID, password)
}

func (s *Service) AuthenticateSupplier(ctx context.Context, phone string, telegramUserID int64, password string) (suplier.SupplierAuth, error) {
	if s.supplierAuth == nil {
		return suplier.SupplierAuth{}, fmt.Errorf("supplier auth service is not configured")
	}
	return s.supplierAuth.Authenticate(ctx, phone, telegramUserID, password)
}

func (s *Service) FindSupplierChatIDByPhone(phone string) (int64, bool) {
	normalized := normalizePhoneForMatch(phone)
	if normalized == "" {
		return 0, false
	}

	s.sessions.mu.RLock()
	defer s.sessions.mu.RUnlock()
	for principalID, session := range s.sessions.sessions {
		if session.UserRole != UserRoleSupplier {
			continue
		}
		if normalizePhoneForMatch(session.UserPhone) == normalized {
			return principalID, true
		}
	}
	return 0, false
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

func (s *Service) SaveContact(kind ContactSetupKind, phone, name string) error {
	if s.admin == nil {
		return fmt.Errorf("admin service is not configured")
	}

	normalizedPhone, err := adminsvc.NormalizeContactPhone(phone)
	if err != nil {
		return err
	}
	normalizedName, err := adminsvc.NormalizeContactName(name)
	if err != nil {
		return err
	}

	switch kind {
	case ContactSetupKindAdminka:
		if err := s.admin.SaveContact(adminsvc.ContactKindAdminka, normalizedPhone, normalizedName); err != nil {
			return err
		}
		s.mu.Lock()
		s.adminkaPhone = normalizedPhone
		s.adminkaName = normalizedName
		s.mu.Unlock()
		return nil
	case ContactSetupKindWerka:
		if err := s.admin.SaveContact(adminsvc.ContactKindWerka, normalizedPhone, normalizedName); err != nil {
			return err
		}
		s.mu.Lock()
		s.werkaPhone = normalizedPhone
		s.werkaName = normalizedName
		s.werkaTelegramID = 0
		s.mu.Unlock()
		s.persistEnv(map[string]string{"WERKA_TELEGRAM_ID": ""})
		return nil
	default:
		return fmt.Errorf("unknown contact setup kind: %s", kind)
	}
}

func (s *Service) BindWerkaTelegramID(principalID int64) {
	s.mu.Lock()
	s.werkaTelegramID = principalID
	s.mu.Unlock()
	s.persistEnv(map[string]string{"WERKA_TELEGRAM_ID": fmt.Sprintf("%d", principalID)})
}

func (s *Service) WerkaTelegramID() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.werkaTelegramID
}

func (s *Service) MatchPrivilegedContact(phone string) (UserRole, string, bool) {
	normalizedPhone := normalizePhoneForMatch(phone)
	if normalizedPhone == "" {
		return UserRoleNone, "", false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	switch normalizedPhone {
	case s.adminkaPhone:
		return UserRoleAdmin, s.adminkaName, s.adminkaPhone != ""
	case s.werkaPhone:
		return UserRoleWerka, s.werkaName, s.werkaPhone != ""
	default:
		return UserRoleNone, "", false
	}
}

func normalizePhoneForMatch(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}

	var digits strings.Builder
	for _, r := range trimmed {
		if unicode.IsDigit(r) {
			digits.WriteRune(r)
		}
	}

	normalizedDigits := digits.String()
	if len(normalizedDigits) < 9 || len(normalizedDigits) > 12 {
		return ""
	}
	return "+" + normalizedDigits
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
