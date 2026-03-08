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
	"erpnext_stock_telegram/internal/erpnext"
	"erpnext_stock_telegram/internal/store"
	"erpnext_stock_telegram/internal/suplier"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type telegramCall struct {
	endpoint string
	form     map[string]string
}

type fakeSupplierManager struct {
	items []suplier.Supplier
	added []suplier.Supplier
	err   error
}

type fakeSupplierAuthManager struct {
	items       map[string]suplier.SupplierAuth
	passwords   map[string]string
	registerErr error
	authErr     error
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
	f.items = append(f.items, supplier)
	f.added = append(f.added, supplier)
	return supplier, nil
}

func (f *fakeSupplierManager) FindByPhone(_ context.Context, phone string) (suplier.Supplier, bool, error) {
	if f.err != nil {
		return suplier.Supplier{}, false, f.err
	}
	for _, supplier := range f.items {
		if supplier.Phone == phone {
			return supplier, true, nil
		}
	}
	return suplier.Supplier{}, false, nil
}

func (f *fakeSupplierManager) List(_ context.Context) ([]suplier.Supplier, error) {
	if f.err != nil {
		return nil, f.err
	}
	return append([]suplier.Supplier(nil), f.items...), nil
}

func (f *fakeSupplierAuthManager) FindByPhone(_ context.Context, phone string) (suplier.SupplierAuth, bool, error) {
	if f.items == nil {
		return suplier.SupplierAuth{}, false, nil
	}
	item, ok := f.items[phone]
	return item, ok, nil
}

func (f *fakeSupplierAuthManager) Register(_ context.Context, phone string, telegramUserID int64, password string) (suplier.SupplierAuth, error) {
	if f.registerErr != nil {
		return suplier.SupplierAuth{}, f.registerErr
	}
	if f.items == nil {
		f.items = map[string]suplier.SupplierAuth{}
	}
	if f.passwords == nil {
		f.passwords = map[string]string{}
	}
	auth := suplier.SupplierAuth{Phone: phone, TelegramUserID: telegramUserID, PasswordHash: "hashed:" + password}
	f.items[phone] = auth
	f.passwords[phone] = password
	return auth, nil
}

func (f *fakeSupplierAuthManager) Authenticate(_ context.Context, phone string, telegramUserID int64, password string) (suplier.SupplierAuth, error) {
	if f.authErr != nil {
		return suplier.SupplierAuth{}, f.authErr
	}
	expected, ok := f.passwords[phone]
	if !ok {
		return suplier.SupplierAuth{}, suplier.ErrSupplierAuthNotFound
	}
	if expected != password {
		return suplier.SupplierAuth{}, suplier.ErrSupplierAuthInvalidPassword
	}
	auth := f.items[phone]
	auth.TelegramUserID = telegramUserID
	f.items[phone] = auth
	return auth, nil
}

func TestHandleCommandStartRequestsContact(t *testing.T) {
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
		case "sendMessage":
			_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":55,"chat":{"id":123,"type":"private"},"date":1,"text":"ok"}}`))
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
	service := NewService(sessions, creds, &fakeERP{}, nil, nil, nil, "secret", "", "", "Kg", "", "", "", "", "", "", "", 0, nil)

	message := &tgbotapi.Message{
		MessageID: 1,
		Text:      "/start",
		Chat:      &tgbotapi.Chat{ID: 123},
		Entities: []tgbotapi.MessageEntity{
			{Type: "bot_command", Offset: 0, Length: len("/start")},
		},
	}

	if err := handleCommand(context.Background(), api, service, message, 123, 123, LoginSession{}); err != nil {
		t.Fatalf("handleCommand returned error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	var sendFound bool
	for _, call := range calls {
		if call.endpoint != "sendMessage" {
			continue
		}
		sendFound = true
		if call.form["text"] != "Telefon raqamingizni yuboring." {
			t.Fatalf("unexpected text: %+v", call.form)
		}
		if !strings.Contains(call.form["reply_markup"], "request_contact") {
			t.Fatalf("expected contact request keyboard, got %+v", call.form)
		}
	}
	if !sendFound {
		t.Fatal("expected sendMessage call")
	}
}

func TestHandleIncomingMessageSharedContactAuthenticatesAdminDirectly(t *testing.T) {
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
		case "sendMessage":
			_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":56,"chat":{"id":123,"type":"private"},"date":1,"text":"ok"}}`))
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
	supplierManager := &fakeSupplierManager{
		items: []suplier.Supplier{{Name: "Ali", Phone: "+998901234567"}},
	}
	service := NewService(sessions, creds, &fakeERP{}, nil, supplierManager, nil, "secret", "", "", "Kg", "", "", "", "+998 90 123 45 67", "Aziza", "", "", 0, nil)

	message := &tgbotapi.Message{
		MessageID: 2,
		Chat:      &tgbotapi.Chat{ID: 123},
		From:      &tgbotapi.User{ID: 123},
		Contact:   &tgbotapi.Contact{PhoneNumber: "+998901234567"},
	}

	if err := handleIncomingMessage(context.Background(), api, service, message); err != nil {
		t.Fatalf("handleIncomingMessage returned error: %v", err)
	}

	session, ok := sessions.Get(123)
	if !ok {
		t.Fatal("expected session to exist")
	}
	if session.UserRole != UserRoleAdmin || !session.AdminAuthed {
		t.Fatalf("expected admin auth, got %+v", session)
	}

	mu.Lock()
	defer mu.Unlock()
	var sendFound bool
	for _, call := range calls {
		if call.endpoint != "sendMessage" {
			continue
		}
		sendFound = true
		if call.form["text"] != authenticatedStartText(session) {
			t.Fatalf("unexpected text: %+v", call.form)
		}
		if !strings.Contains(call.form["text"], "/supplier - supplier qo'shish") {
			t.Fatalf("expected admin commands in text, got %+v", call.form)
		}
		if !strings.Contains(call.form["text"], "/logout - paneldan chiqish") {
			t.Fatalf("expected logout command in text, got %+v", call.form)
		}
		if !strings.Contains(call.form["reply_markup"], "remove_keyboard") {
			t.Fatalf("expected keyboard removal, got %+v", call.form)
		}
	}
	if !sendFound {
		t.Fatal("expected sendMessage call")
	}
}

func TestHandleIncomingMessageSharedContactSupplierAuthFlow(t *testing.T) {
	var (
		mu         sync.Mutex
		calls      []telegramCall
		sendCount  int
		messageIDs = []int{61, 62}
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
		case "editMessageText":
			_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":61,"chat":{"id":123,"type":"private"},"date":1,"text":"ok"}}`))
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
	supplierManager := &fakeSupplierManager{
		items: []suplier.Supplier{{Name: "Abdulloh", Phone: "+998901234567"}},
	}
	service := NewService(sessions, creds, &fakeERP{}, nil, supplierManager, &fakeSupplierAuthManager{}, "secret", "", "", "Kg", "", "", "", "", "", "", "", 0, nil)

	contactMessage := &tgbotapi.Message{
		MessageID: 3,
		Chat:      &tgbotapi.Chat{ID: 123},
		From:      &tgbotapi.User{ID: 123},
		Contact:   &tgbotapi.Contact{PhoneNumber: "+998901234567"},
	}
	if err := handleIncomingMessage(context.Background(), api, service, contactMessage); err != nil {
		t.Fatalf("handleIncomingMessage(contact) returned error: %v", err)
	}

	nameMessage := &tgbotapi.Message{
		MessageID: 4,
		Text:      "Abdullox",
		Chat:      &tgbotapi.Chat{ID: 123},
		From:      &tgbotapi.User{ID: 123},
	}
	if err := handleIncomingMessage(context.Background(), api, service, nameMessage); err != nil {
		t.Fatalf("handleIncomingMessage(name) returned error: %v", err)
	}

	weakPassword := &tgbotapi.Message{
		MessageID: 5,
		Text:      "abcdefg",
		Chat:      &tgbotapi.Chat{ID: 123},
		From:      &tgbotapi.User{ID: 123},
	}
	if err := handleIncomingMessage(context.Background(), api, service, weakPassword); err != nil {
		t.Fatalf("handleIncomingMessage(weakPassword) returned error: %v", err)
	}

	strongPassword := &tgbotapi.Message{
		MessageID: 6,
		Text:      "abc12345",
		Chat:      &tgbotapi.Chat{ID: 123},
		From:      &tgbotapi.User{ID: 123},
	}
	if err := handleIncomingMessage(context.Background(), api, service, strongPassword); err != nil {
		t.Fatalf("handleIncomingMessage(strongPassword) returned error: %v", err)
	}

	session, ok := sessions.Get(123)
	if !ok {
		t.Fatal("expected session to exist")
	}
	if session.UserRole != UserRoleSupplier || session.UserName != "Abdulloh" {
		t.Fatalf("expected supplier auth to complete, got %+v", session)
	}
	if session.SupplierAuthStep != SupplierAuthStepNone {
		t.Fatalf("expected supplier auth to be cleared, got %+v", session)
	}

	mu.Lock()
	defer mu.Unlock()
	var askName bool
	var weakPasswordEdit bool
	var success bool
	for _, call := range calls {
		switch {
		case call.endpoint == "sendMessage" && call.form["text"] == "Telefon topildi. Ismingizni kiriting:":
			askName = true
		case call.endpoint == "editMessageText" && strings.Contains(call.form["text"], "Parol kamida 8 belgidan iborat bo'lishi kerak"):
			weakPasswordEdit = true
		case call.endpoint == "sendMessage" &&
			strings.Contains(call.form["text"], "Ro'yxatdan o'tish yakunlandi. Siz supplier sifatida kirdingiz.") &&
			strings.Contains(call.form["text"], "/dispatch - jo'natilgan mahsulotni bildirish"):
			success = true
		}
	}
	if !askName || !weakPasswordEdit || !success {
		t.Fatalf("unexpected call set: %+v", calls)
	}
}

func TestHandleIncomingMessageSharedContactSupplierRepeatLoginFlow(t *testing.T) {
	var (
		mu         sync.Mutex
		calls      []telegramCall
		sendCount  int
		messageIDs = []int{71, 72}
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
		case "editMessageText":
			_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":71,"chat":{"id":123,"type":"private"},"date":1,"text":"ok"}}`))
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
	supplierManager := &fakeSupplierManager{
		items: []suplier.Supplier{{Name: "Abdulloh", Phone: "+998901234567"}},
	}
	authManager := &fakeSupplierAuthManager{
		items: map[string]suplier.SupplierAuth{
			"+998901234567": {Phone: "+998901234567", TelegramUserID: 123, PasswordHash: "hashed:abc12345"},
		},
		passwords: map[string]string{
			"+998901234567": "abc12345",
		},
	}
	service := NewService(sessions, creds, &fakeERP{}, nil, supplierManager, authManager, "secret", "", "", "Kg", "", "", "", "", "", "", "", 0, nil)

	contactMessage := &tgbotapi.Message{
		MessageID: 7,
		Chat:      &tgbotapi.Chat{ID: 123},
		From:      &tgbotapi.User{ID: 123},
		Contact:   &tgbotapi.Contact{PhoneNumber: "+998901234567"},
	}
	if err := handleIncomingMessage(context.Background(), api, service, contactMessage); err != nil {
		t.Fatalf("handleIncomingMessage(contact) returned error: %v", err)
	}

	wrongPassword := &tgbotapi.Message{
		MessageID: 8,
		Text:      "wrong",
		Chat:      &tgbotapi.Chat{ID: 123},
		From:      &tgbotapi.User{ID: 123},
	}
	if err := handleIncomingMessage(context.Background(), api, service, wrongPassword); err != nil {
		t.Fatalf("handleIncomingMessage(wrongPassword) returned error: %v", err)
	}

	rightPassword := &tgbotapi.Message{
		MessageID: 9,
		Text:      "abc12345",
		Chat:      &tgbotapi.Chat{ID: 123},
		From:      &tgbotapi.User{ID: 123},
	}
	if err := handleIncomingMessage(context.Background(), api, service, rightPassword); err != nil {
		t.Fatalf("handleIncomingMessage(rightPassword) returned error: %v", err)
	}

	session, ok := sessions.Get(123)
	if !ok {
		t.Fatal("expected session to exist")
	}
	if session.UserRole != UserRoleSupplier || session.UserName != "Abdulloh" {
		t.Fatalf("expected supplier login to complete, got %+v", session)
	}
	if session.SupplierAuthStep != SupplierAuthStepNone || session.SupplierAuthMode != SupplierAuthModeNone {
		t.Fatalf("expected supplier auth state to be cleared, got %+v", session)
	}

	mu.Lock()
	defer mu.Unlock()
	var askPassword bool
	var wrongPasswordEdit bool
	var success bool
	for _, call := range calls {
		switch {
		case call.endpoint == "sendMessage" && call.form["text"] == "Telefon topildi. Parolingizni kiriting:":
			askPassword = true
		case call.endpoint == "editMessageText" && strings.Contains(call.form["text"], "Parol noto'g'ri"):
			wrongPasswordEdit = true
		case call.endpoint == "sendMessage" &&
			strings.Contains(call.form["text"], "Kirish muvaffaqiyatli. Siz supplier sifatida kirdingiz.") &&
			strings.Contains(call.form["text"], "/dispatch - jo'natilgan mahsulotni bildirish"):
			success = true
		}
	}
	if !askPassword || !wrongPasswordEdit || !success {
		t.Fatalf("unexpected call set: %+v", calls)
	}
}

func TestSupplierDispatchCreatesDraftPurchaseReceiptAndNotifiesWerka(t *testing.T) {
	var (
		mu         sync.Mutex
		calls      []telegramCall
		sendCount  int
		messageIDs = []int{101, 102}
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
		case "editMessageText":
			_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":101,"chat":{"id":123,"type":"private"},"date":1,"text":"ok"}}`))
		case "answerCallbackQuery", "deleteMessage":
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

	fakeERP := &fakeERP{
		supplierItems: []erpnext.Item{{Code: "ITEM-001", Name: "Rice", UOM: "Kg"}},
		createDraftResult: erpnext.PurchaseReceiptDraft{
			Name:     "MAT-PRE-0001",
			Supplier: "Abdulloh",
			ItemCode: "ITEM-001",
			Qty:      10,
			UOM:      "Kg",
		},
	}

	sessions := NewSessionManager()
	creds := store.NewMemoryCredentialStore()
	service := NewService(sessions, creds, fakeERP, nil, nil, nil, "secret", "Stores - CH", "", "Kg", "https://erp.example.com", "key", "secret", "", "", "", "", 999, nil)
	session := LoginSession{UserRole: UserRoleSupplier, UserName: "Abdulloh", UserPhone: "+998901234567"}
	sessions.Upsert(123, session)

	command := &tgbotapi.Message{
		MessageID: 10,
		Text:      "/dispatch",
		Chat:      &tgbotapi.Chat{ID: 123},
		From:      &tgbotapi.User{ID: 123},
		Entities: []tgbotapi.MessageEntity{
			{Type: "bot_command", Offset: 0, Length: len("/dispatch")},
		},
	}
	if err := handleIncomingMessage(context.Background(), api, service, command); err != nil {
		t.Fatalf("handleIncomingMessage(command) returned error: %v", err)
	}

	itemMessage := &tgbotapi.Message{
		MessageID: 11,
		Text:      "item::ITEM-001",
		Chat:      &tgbotapi.Chat{ID: 123},
		From:      &tgbotapi.User{ID: 123},
	}
	if err := handleIncomingMessage(context.Background(), api, service, itemMessage); err != nil {
		t.Fatalf("handleIncomingMessage(item) returned error: %v", err)
	}

	qtyMessage := &tgbotapi.Message{
		MessageID: 12,
		Text:      "10",
		Chat:      &tgbotapi.Chat{ID: 123},
		From:      &tgbotapi.User{ID: 123},
	}
	if err := handleIncomingMessage(context.Background(), api, service, qtyMessage); err != nil {
		t.Fatalf("handleIncomingMessage(qty) returned error: %v", err)
	}

	cb := &tgbotapi.CallbackQuery{
		ID:   "cb-dispatch",
		Data: callbackDispatchConfirm,
		From: &tgbotapi.User{ID: 123},
		Message: &tgbotapi.Message{
			MessageID: 101,
			Chat:      &tgbotapi.Chat{ID: 123},
		},
	}
	if err := handleCallbackQuery(context.Background(), api, service, cb); err != nil {
		t.Fatalf("handleCallbackQuery returned error: %v", err)
	}

	if fakeERP.createDraftInput.Supplier != "Abdulloh" || fakeERP.createDraftInput.ItemCode != "ITEM-001" {
		t.Fatalf("unexpected draft input: %+v", fakeERP.createDraftInput)
	}
	if fakeERP.createDraftInput.Warehouse != "Stores - CH" || fakeERP.createDraftInput.Qty != 10 {
		t.Fatalf("unexpected draft input: %+v", fakeERP.createDraftInput)
	}

	mu.Lock()
	defer mu.Unlock()
	var supplierPrompt bool
	var dispatchSuccess bool
	var werkaNotified bool
	for _, call := range calls {
		switch {
		case call.endpoint == "sendMessage" && strings.Contains(call.form["text"], "Mahsulot tanlang."):
			supplierPrompt = true
		case call.endpoint == "editMessageText" && strings.Contains(call.form["text"], "Draft Purchase Receipt: MAT-PRE-0001"):
			dispatchSuccess = true
		case call.endpoint == "sendMessage" && call.form["chat_id"] == "999" && strings.Contains(call.form["text"], "Yangi bildirishnoma."):
			werkaNotified = true
		}
	}
	if !supplierPrompt || !dispatchSuccess || !werkaNotified {
		t.Fatalf("unexpected call set: %+v", calls)
	}
}

func TestWerkaBildirishnomaFlowSubmitsPurchaseReceiptAndNotifiesSupplier(t *testing.T) {
	var (
		mu         sync.Mutex
		calls      []telegramCall
		sendCount  int
		messageIDs = []int{201, 202}
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
		case "editMessageText":
			_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":201,"chat":{"id":123,"type":"private"},"date":1,"text":"ok"}}`))
		case "answerCallbackQuery", "deleteMessage":
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

	fakeERP := &fakeERP{
		pendingReceipts: []erpnext.PurchaseReceiptDraft{{
			Name:                 "MAT-PRE-0002",
			Supplier:             "Abdulloh",
			SupplierDeliveryNote: "TG:+998901234567:20260307120000",
			ItemCode:             "ITEM-001",
			ItemName:             "Rice",
			Qty:                  10,
			UOM:                  "Kg",
		}},
		receipt: erpnext.PurchaseReceiptDraft{
			Name:                 "MAT-PRE-0002",
			Supplier:             "Abdulloh",
			SupplierDeliveryNote: "TG:+998901234567:20260307120000",
			ItemCode:             "ITEM-001",
			ItemName:             "Rice",
			Qty:                  10,
			UOM:                  "Kg",
		},
		submitResult: erpnext.PurchaseReceiptSubmissionResult{
			Name:                 "MAT-PRE-0002",
			Supplier:             "Abdulloh",
			ItemCode:             "ITEM-001",
			UOM:                  "Kg",
			SentQty:              10,
			AcceptedQty:          7,
			SupplierDeliveryNote: "TG:+998901234567:20260307120000",
		},
	}

	authManager := &fakeSupplierAuthManager{
		items: map[string]suplier.SupplierAuth{
			"+998901234567": {Phone: "+998901234567", TelegramUserID: 555},
		},
	}

	sessions := NewSessionManager()
	creds := store.NewMemoryCredentialStore()
	service := NewService(sessions, creds, fakeERP, nil, nil, authManager, "secret", "Stores - CH", "", "Kg", "https://erp.example.com", "key", "secret", "", "", "", "", 0, nil)
	sessions.Upsert(321, LoginSession{UserRole: UserRoleWerka})

	command := &tgbotapi.Message{
		MessageID: 20,
		Text:      "/bildirishnoma",
		Chat:      &tgbotapi.Chat{ID: 321},
		From:      &tgbotapi.User{ID: 321},
		Entities: []tgbotapi.MessageEntity{
			{Type: "bot_command", Offset: 0, Length: len("/bildirishnoma")},
		},
	}
	if err := handleIncomingMessage(context.Background(), api, service, command); err != nil {
		t.Fatalf("handleIncomingMessage(command) returned error: %v", err)
	}

	cb := &tgbotapi.CallbackQuery{
		ID:   "cb-notice",
		Data: callbackNoticeOpenPrefix + "MAT-PRE-0002",
		From: &tgbotapi.User{ID: 321},
		Message: &tgbotapi.Message{
			MessageID: 201,
			Chat:      &tgbotapi.Chat{ID: 321},
		},
	}
	if err := handleCallbackQuery(context.Background(), api, service, cb); err != nil {
		t.Fatalf("handleCallbackQuery returned error: %v", err)
	}

	qtyMessage := &tgbotapi.Message{
		MessageID: 21,
		Text:      "7",
		Chat:      &tgbotapi.Chat{ID: 321},
		From:      &tgbotapi.User{ID: 321},
	}
	if err := handleIncomingMessage(context.Background(), api, service, qtyMessage); err != nil {
		t.Fatalf("handleIncomingMessage(qty) returned error: %v", err)
	}

	if fakeERP.confirmName != "MAT-PRE-0002" || fakeERP.confirmQty != 7 {
		t.Fatalf("unexpected submit call: name=%q qty=%v", fakeERP.confirmName, fakeERP.confirmQty)
	}

	mu.Lock()
	defer mu.Unlock()
	var pendingListShown bool
	var qtyPromptShown bool
	var supplierNotified bool
	for _, call := range calls {
		switch {
		case call.endpoint == "sendMessage" && strings.Contains(call.form["text"], "Pending qabul qilish ro'yxati"):
			pendingListShown = true
		case call.endpoint == "editMessageText" && strings.Contains(call.form["text"], "Qabul qilingan miqdorni kiriting."):
			qtyPromptShown = true
		case call.endpoint == "sendMessage" && call.form["chat_id"] == "555" && strings.Contains(call.form["text"], "Omborchi mahsulotni qabul qildi."):
			supplierNotified = true
		}
	}
	if !pendingListShown || !qtyPromptShown || !supplierNotified {
		t.Fatalf("unexpected call set: %+v", calls)
	}
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
	service := NewService(sessions, creds, &fakeERP{}, nil, nil, nil, "secret", "", "", "Kg", "", "", "", "", "", "", "", 0, nil)

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
	service := NewService(sessions, creds, &fakeERP{}, nil, nil, nil, "secret", "", "", "Kg", "", "", "", "", "", "", "", 0, nil)

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
	service := NewService(sessions, creds, &fakeERP{}, nil, supplierManager, nil, "secret", "", "", "Kg", "", "", "", "", "", "", "", 0, nil)

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
	service := NewService(sessions, creds, &fakeERP{}, adminManager, nil, nil, "secret", "", "", "Kg", "", "", "", "", "", "", "", 0, nil)
	adminSession := LoginSession{AdminAuthed: true}

	commandMessage := &tgbotapi.Message{
		MessageID: 1,
		Text:      "/adminka",
		Chat:      &tgbotapi.Chat{ID: 123},
		Entities: []tgbotapi.MessageEntity{
			{Type: "bot_command", Offset: 0, Length: len("/adminka")},
		},
	}
	if err := handleCommand(context.Background(), api, service, commandMessage, 123, 123, adminSession); err != nil {
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

func TestAdminPanelRejectsNonAdminCommandsWithoutLeavingSession(t *testing.T) {
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
		case "editMessageText":
			_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":700,"chat":{"id":123,"type":"private"},"date":1,"text":"ok"}}`))
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
	service := NewService(sessions, creds, &fakeERP{}, &fakeAdminManager{configured: true, password: "p"}, nil, nil, "secret", "", "", "Kg", "", "", "", "", "", "", "", 0, nil)
	sessions.Upsert(123, LoginSession{
		AdminAuthed:  true,
		AdminPanelID: 700,
	})

	message := &tgbotapi.Message{
		MessageID: 10,
		Text:      "/stock",
		Chat:      &tgbotapi.Chat{ID: 123},
		From:      &tgbotapi.User{ID: 123},
		Entities: []tgbotapi.MessageEntity{
			{Type: "bot_command", Offset: 0, Length: len("/stock")},
		},
	}

	if err := handleIncomingMessage(context.Background(), api, service, message); err != nil {
		t.Fatalf("handleIncomingMessage returned error: %v", err)
	}

	session, ok := sessions.Get(123)
	if !ok {
		t.Fatal("expected session to exist")
	}
	if !session.AdminAuthed {
		t.Fatalf("expected admin session to remain active, got %+v", session)
	}

	mu.Lock()
	defer mu.Unlock()
	var edited bool
	for _, call := range calls {
		if call.endpoint == "editMessageText" && call.form["text"] == adminOnlyCommandText() {
			edited = true
		}
	}
	if !edited {
		t.Fatalf("expected admin-only warning to be shown, calls=%+v", calls)
	}
}
