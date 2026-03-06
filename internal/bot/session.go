package bot

import "sync"

type LoginStep int

const (
	LoginStepNone LoginStep = iota
	LoginStepAwaitingURL
	LoginStepAwaitingAPIKey
	LoginStepAwaitingAPISecret
)

type ActionStep int

const (
	ActionStepNone ActionStep = iota
	ActionStepAwaitingType
	ActionStepAwaitingItem
	ActionStepAwaitingUOM
	ActionStepAwaitingQty
)

type ActionType string

const (
	ActionTypeReceipt ActionType = "receipt"
	ActionTypeIssue   ActionType = "issue"
)

type SettingsStep int

const (
	SettingsStepNone SettingsStep = iota
	SettingsStepAwaitingPassword
)

type AdminStep int

const (
	AdminStepNone AdminStep = iota
	AdminStepAwaitingSetupPassword
	AdminStepAwaitingPassword
)

type SupplierStep int

const (
	SupplierStepNone SupplierStep = iota
	SupplierStepAwaitingName
	SupplierStepAwaitingPhone
)

type ContactSetupStep int

const (
	ContactSetupStepNone ContactSetupStep = iota
	ContactSetupStepAwaitingPhone
	ContactSetupStepAwaitingName
)

type ContactSetupKind string

const (
	ContactSetupKindNone    ContactSetupKind = ""
	ContactSetupKindAdminka ContactSetupKind = "adminka"
	ContactSetupKindWerka   ContactSetupKind = "werka"
)

type SettingsSelectionType string

const (
	SettingsSelectionNone      SettingsSelectionType = ""
	SettingsSelectionWarehouse SettingsSelectionType = "wer"
	SettingsSelectionUOM       SettingsSelectionType = "uom"
)

type LoginSession struct {
	Step                    LoginStep
	BaseURL                 string
	APIKey                  string
	WelcomeMessageID        int
	PromptMessageID         int
	ActionStep              ActionStep
	ActionType              ActionType
	SelectedItemCode        string
	SelectedUOM             string
	RequireUOMFirst         bool
	LastActionType          ActionType
	LastItemCode            string
	LastUOM                 string
	SettingsStep            SettingsStep
	SettingsAuthed          bool
	SettingsPanelID         int
	SettingsSelect          SettingsSelectionType
	AdminStep               AdminStep
	AdminAuthed             bool
	AdminPanelID            int
	SupplierStep            SupplierStep
	SupplierName            string
	SupplierNameMsgID       int
	SupplierPhoneMsgID      int
	SupplierNameInputMsgID  int
	SupplierPhoneInputMsgID int
	ContactSetupStep        ContactSetupStep
	ContactSetupKind        ContactSetupKind
	ContactPhone            string
	ContactPhoneMsgID       int
	ContactNameMsgID        int
	ContactPhoneInputMsgID  int
	ContactNameInputMsgID   int
}

type SessionManager struct {
	mu       sync.RWMutex
	sessions map[int64]LoginSession
}

func commandUsesSettingsContext(command string) bool {
	switch command {
	case "wer", "uom", "logout":
		return true
	default:
		return false
	}
}

func commandUsesAdminContext(command string) bool {
	switch command {
	case "admin", "supplier", "adminka", "werka", "logout":
		return true
	default:
		return false
	}
}

func resetSessionForCommand(session LoginSession, command string) LoginSession {
	cleaned := LoginSession{}
	if commandUsesSettingsContext(command) {
		cleaned.SettingsAuthed = session.SettingsAuthed
		cleaned.SettingsPanelID = session.SettingsPanelID
	}
	if commandUsesAdminContext(command) {
		cleaned.AdminAuthed = session.AdminAuthed
		cleaned.AdminPanelID = session.AdminPanelID
	}
	return cleaned
}

func NewSessionManager() *SessionManager {
	return &SessionManager{sessions: make(map[int64]LoginSession)}
}

func (m *SessionManager) StartLogin(chatID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	existing := m.sessions[chatID]
	m.sessions[chatID] = LoginSession{
		Step:             LoginStepAwaitingURL,
		WelcomeMessageID: existing.WelcomeMessageID,
	}
}

func (m *SessionManager) Get(chatID int64) (LoginSession, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[chatID]
	return s, ok
}

func (m *SessionManager) Upsert(chatID int64, session LoginSession) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[chatID] = session
}

func (m *SessionManager) Clear(chatID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, chatID)
}
