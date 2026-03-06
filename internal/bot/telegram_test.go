package bot

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	adminsvc "erpnext_stock_telegram/internal/admin"
	"erpnext_stock_telegram/internal/store"
	"erpnext_stock_telegram/internal/suplier"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type telegramCall struct {
	endpoint string
	form     map[string]string
}

type fakeSupplierManager struct {
	added []suplier.Supplier
	err   error
}

type fakeAdminManager struct {
	configured bool
	password   string
	contacts   map[adminsvc.ContactKind]suplier.Supplier
}

func (f *fakeAdminManager) IsConfigured() bool {
	return f.configured
}

func (f *fakeAdminManager) ValidatePassword(input string) bool {
	return strings.TrimSpace(input) == f.password
}

func (f *fakeAdminManager) SetPassword(input string) error {
	f.password = strings.TrimSpace(input)
	f.configured = f.password != ""
	return nil
}

func (f *fakeAdminManager) SaveContact(kind adminsvc.ContactKind, phone, name string) error {
	if f.contacts == nil {
		f.contacts = map[adminsvc.ContactKind]suplier.Supplier{}
	}
	f.contacts[kind] = suplier.Supplier{Name: name, Phone: phone}
	return nil
}

func (f *fakeSupplierManager) Add(_ context.Context, name, phone string) (suplier.Supplier, error) {
	if f.err != nil {
		return suplier.Supplier{}, f.err
	}
	supplier := suplier.Supplier{Name: name, Phone: phone}
	f.added = append(f.added, supplier)
	return supplier, nil
}

func TestHandleCallbackQueryAgainDoesNotSendInvalidInlineKeyboard(t *testing.T) {
	var (
		mu    sync.Mutex
		calls []telegramCall
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		endpoint := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
		form := make(map[string]string, len(r.PostForm))
		for k, v := range r.PostForm {
			if len(v) > 0 {
				form[k] = v[0]
			}
		}

		mu.Lock()
		calls = append(calls, telegramCall{endpoint: endpoint, form: form})
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		switch endpoint {
		case "getMe":
			_, _ = w.Write([]byte(`{"ok":true,"result":{"id":1,"is_bot":true,"first_name":"bot","username":"bot"}}`))
		case "answerCallbackQuery":
			_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
		case "editMessageText":
			_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":11,"chat":{"id":123,"type":"private"},"date":1,"text":"ok"}}`))
		default:
			enc := json.NewEncoder(w)
			_ = enc.Encode(map[string]any{"ok": true, "result": true})
		}
	}))
	defer server.Close()

	api, err := tgbotapi.NewBotAPIWithClient("TEST_TOKEN", server.URL+"/bot%s/%s", server.Client())
	if err != nil {
		t.Fatalf("failed to init bot api: %v", err)
	}

	sessions := NewSessionManager()
	creds := store.NewMemoryCredentialStore()
	service := NewService(sessions, creds, &fakeERP{}, nil, nil, "secret", "", "", "Kg", "", "", "", nil)

	principalID := int64(777)
	creds.Save(principalID, store.Credentials{BaseURL: "https://erp.example.com", APIKey: "k", APISecret: "s"})
	sessions.Upsert(principalID, LoginSession{
		LastActionType: ActionTypeReceipt,
		LastItemCode:   "CHEARS NACHOS",
		LastUOM:        "Kg",
	})

	cb := &tgbotapi.CallbackQuery{
		ID:   "cb1",
		Data: callbackAgainAction,
		From: &tgbotapi.User{ID: principalID},
		Message: &tgbotapi.Message{
			MessageID: 11,
			Chat:      &tgbotapi.Chat{ID: 123},
		},
	}

	if err := handleCallbackQuery(context.Background(), api, service, cb); err != nil {
		t.Fatalf("handleCallbackQuery returned error: %v", err)
	}

	updated, ok := sessions.Get(principalID)
	if !ok {
		t.Fatal("expected session to exist")
	}
	if updated.ActionStep != ActionStepAwaitingQty {
		t.Fatalf("expected action step AwaitingQty, got %v", updated.ActionStep)
	}
	if updated.SelectedUOM != "Kg" {
		t.Fatalf("expected SelectedUOM=Kg, got %q", updated.SelectedUOM)
	}

	mu.Lock()
	defer mu.Unlock()
	var editFound bool
	for _, c := range calls {
		if c.endpoint != "editMessageText" {
			continue
		}
		editFound = true
		if _, hasReplyMarkup := c.form["reply_markup"]; hasReplyMarkup {
			t.Fatalf("unexpected reply_markup in editMessageText: %q", c.form["reply_markup"])
		}
	}
	if !editFound {
		t.Fatal("expected editMessageText call")
	}
}

func TestHandleCommandAdminStartsPasswordSetupWhenUnconfigured(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		endpoint := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
		w.Header().Set("Content-Type", "application/json")
		switch endpoint {
		case "getMe":
			_, _ = w.Write([]byte(`{"ok":true,"result":{"id":1,"is_bot":true,"first_name":"bot","username":"bot"}}`))
		case "sendMessage":
			_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":77,"chat":{"id":123,"type":"private"},"date":1,"text":"ok"}}`))
		case "deleteMessage":
			_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
		default:
			enc := json.NewEncoder(w)
			_ = enc.Encode(map[string]any{"ok": true, "result": true})
		}
	}))
	defer server.Close()

	api, err := tgbotapi.NewBotAPIWithClient("TEST_TOKEN", server.URL+"/bot%s/%s", server.Client())
	if err != nil {
		t.Fatalf("failed to init bot api: %v", err)
	}

	sessions := NewSessionManager()
	creds := store.NewMemoryCredentialStore()
	service := NewService(sessions, creds, &fakeERP{}, nil, nil, "secret", "", "", "Kg", "", "", "", nil)

	message := &tgbotapi.Message{
		MessageID: 1,
		Text:      "/admin",
		Chat:      &tgbotapi.Chat{ID: 123},
		Entities: []tgbotapi.MessageEntity{
			{Type: "bot_command", Offset: 0, Length: len("/admin")},
		},
	}

	if err := handleCommand(context.Background(), api, service, message, 123, 123, LoginSession{}); err != nil {
		t.Fatalf("handleCommand returned error: %v", err)
	}

	session, ok := sessions.Get(123)
	if !ok {
		t.Fatal("expected session to exist")
	}
	if session.AdminStep != AdminStepAwaitingSetupPassword {
		t.Fatalf("expected admin setup step, got %+v", session)
	}
	if session.AdminPanelID == 0 {
		t.Fatalf("expected admin panel message id to be set, got %+v", session)
	}
}

func TestSupplierFlowDeletesTwoBotAndTwoUserMessagesOnSuccess(t *testing.T) {
	var (
		mu         sync.Mutex
		calls      []telegramCall
		sendCount  int
		messageIDs = []int{201, 202, 203}
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		endpoint := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
		form := make(map[string]string, len(r.PostForm))
		for k, v := range r.PostForm {
			if len(v) > 0 {
				form[k] = v[0]
			}
		}

		mu.Lock()
		calls = append(calls, telegramCall{endpoint: endpoint, form: form})
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		switch endpoint {
		case "getMe":
			_, _ = w.Write([]byte(`{"ok":true,"result":{"id":1,"is_bot":true,"first_name":"bot","username":"bot"}}`))
		case "sendMessage":
			id := messageIDs[sendCount]
			sendCount++
			_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":` + strconv.Itoa(id) + `,"chat":{"id":123,"type":"private"},"date":1,"text":"ok"}}`))
		case "deleteMessage":
			_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
		default:
			enc := json.NewEncoder(w)
			_ = enc.Encode(map[string]any{"ok": true, "result": true})
		}
	}))
	defer server.Close()

	api, err := tgbotapi.NewBotAPIWithClient("TEST_TOKEN", server.URL+"/bot%s/%s", server.Client())
	if err != nil {
		t.Fatalf("failed to init bot api: %v", err)
	}

	sessions := NewSessionManager()
	creds := store.NewMemoryCredentialStore()
	supplierManager := &fakeSupplierManager{}
	service := NewService(sessions, creds, &fakeERP{}, nil, supplierManager, "secret", "", "", "Kg", "", "", "", nil)

	adminSession := LoginSession{AdminAuthed: true}
	commandMessage := &tgbotapi.Message{
		MessageID: 1,
		Text:      "/supplier",
		Chat:      &tgbotapi.Chat{ID: 123},
		Entities: []tgbotapi.MessageEntity{
			{Type: "bot_command", Offset: 0, Length: len("/supplier")},
		},
	}
	if err := handleCommand(context.Background(), api, service, commandMessage, 123, 123, adminSession); err != nil {
		t.Fatalf("handleCommand returned error: %v", err)
	}

	nameMessage := &tgbotapi.Message{
		MessageID: 301,
		Text:      "Ali",
		Chat:      &tgbotapi.Chat{ID: 123},
		From:      &tgbotapi.User{ID: 123},
	}
	if err := handleIncomingMessage(context.Background(), api, service, nameMessage); err != nil {
		t.Fatalf("handleIncomingMessage(name) returned error: %v", err)
	}

	phoneMessage := &tgbotapi.Message{
		MessageID: 302,
		Text:      "+998901234567",
		Chat:      &tgbotapi.Chat{ID: 123},
		From:      &tgbotapi.User{ID: 123},
	}
	if err := handleIncomingMessage(context.Background(), api, service, phoneMessage); err != nil {
		t.Fatalf("handleIncomingMessage(phone) returned error: %v", err)
	}

	if len(supplierManager.added) != 1 {
		t.Fatalf("expected supplier to be added, got %+v", supplierManager.added)
	}

	mu.Lock()
	defer mu.Unlock()
	deleted := map[string]bool{}
	successFound := false
	for _, call := range calls {
		if call.endpoint == "deleteMessage" {
			deleted[call.form["message_id"]] = true
		}
		if call.endpoint == "sendMessage" && call.form["text"] == "Supplier muvaffaqiyatli qo'shildi." {
			successFound = true
		}
	}
	for _, expected := range []string{"201", "202", "301", "302"} {
		if !deleted[expected] {
			t.Fatalf("expected message %s to be deleted, deleted=%v", expected, deleted)
		}
	}
	if !successFound {
		t.Fatalf("expected success message, calls=%+v", calls)
	}
}

func TestAdminkaFlowDeletesTwoBotAndTwoUserMessagesOnSuccess(t *testing.T) {
	var (
		mu         sync.Mutex
		calls      []telegramCall
		sendCount  int
		messageIDs = []int{401, 402, 403}
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		endpoint := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
		form := make(map[string]string, len(r.PostForm))
		for k, v := range r.PostForm {
			if len(v) > 0 {
				form[k] = v[0]
			}
		}

		mu.Lock()
		calls = append(calls, telegramCall{endpoint: endpoint, form: form})
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		switch endpoint {
		case "getMe":
			_, _ = w.Write([]byte(`{"ok":true,"result":{"id":1,"is_bot":true,"first_name":"bot","username":"bot"}}`))
		case "sendMessage":
			id := messageIDs[sendCount]
			sendCount++
			_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":` + strconv.Itoa(id) + `,"chat":{"id":123,"type":"private"},"date":1,"text":"ok"}}`))
		case "deleteMessage":
			_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
		default:
			enc := json.NewEncoder(w)
			_ = enc.Encode(map[string]any{"ok": true, "result": true})
		}
	}))
	defer server.Close()

	api, err := tgbotapi.NewBotAPIWithClient("TEST_TOKEN", server.URL+"/bot%s/%s", server.Client())
	if err != nil {
		t.Fatalf("failed to init bot api: %v", err)
	}

	adminManager := &fakeAdminManager{}
	sessions := NewSessionManager()
	creds := store.NewMemoryCredentialStore()
	service := NewService(sessions, creds, &fakeERP{}, adminManager, nil, "secret", "", "", "Kg", "", "", "", nil)

	commandMessage := &tgbotapi.Message{
		MessageID: 1,
		Text:      "/adminka",
		Chat:      &tgbotapi.Chat{ID: 123},
		Entities: []tgbotapi.MessageEntity{
			{Type: "bot_command", Offset: 0, Length: len("/adminka")},
		},
	}
	if err := handleCommand(context.Background(), api, service, commandMessage, 123, 123, LoginSession{}); err != nil {
		t.Fatalf("handleCommand returned error: %v", err)
	}

	phoneMessage := &tgbotapi.Message{
		MessageID: 501,
		Text:      "+998901234567",
		Chat:      &tgbotapi.Chat{ID: 123},
		From:      &tgbotapi.User{ID: 123},
	}
	if err := handleIncomingMessage(context.Background(), api, service, phoneMessage); err != nil {
		t.Fatalf("handleIncomingMessage(phone) returned error: %v", err)
	}

	nameMessage := &tgbotapi.Message{
		MessageID: 502,
		Text:      "Aziza",
		Chat:      &tgbotapi.Chat{ID: 123},
		From:      &tgbotapi.User{ID: 123},
	}
	if err := handleIncomingMessage(context.Background(), api, service, nameMessage); err != nil {
		t.Fatalf("handleIncomingMessage(name) returned error: %v", err)
	}

	if got := adminManager.contacts[adminsvc.ContactKindAdminka]; got.Phone != "+998901234567" || got.Name != "Aziza" {
		t.Fatalf("unexpected saved contact: %+v", got)
	}

	mu.Lock()
	defer mu.Unlock()
	deleted := map[string]bool{}
	successFound := false
	for _, call := range calls {
		if call.endpoint == "deleteMessage" {
			deleted[call.form["message_id"]] = true
		}
		if call.endpoint == "sendMessage" && call.form["text"] == "Adminka muvaffaqiyatli saqlandi." {
			successFound = true
		}
	}
	for _, expected := range []string{"401", "402", "501", "502"} {
		if !deleted[expected] {
			t.Fatalf("expected message %s to be deleted, deleted=%v", expected, deleted)
		}
	}
	if !successFound {
		t.Fatalf("expected success message, calls=%+v", calls)
	}
}
