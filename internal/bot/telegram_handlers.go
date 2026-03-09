package bot

import (
	"context"
	"fmt"
	"log"
	"strings"

	adminsvc "erpnext_stock_telegram/internal/admin"
	"erpnext_stock_telegram/internal/erpnext"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func handleSharedContact(ctx context.Context, api *tgbotapi.BotAPI, service *Service, message *tgbotapi.Message, principalID, chatID int64, session LoginSession) error {
	if message.Contact.UserID != 0 && message.Contact.UserID != principalID {
		if _, err := sendTextMessageWithReplyMarkup(api, chatID, "Iltimos, o'zingizning kontaktingizni yuboring.", removeKeyboard()); err != nil {
			return fmt.Errorf("telegram send failed: %w", err)
		}
		return nil
	}

	phone, err := adminsvc.NormalizeContactPhone(message.Contact.PhoneNumber)
	if err != nil {
		if _, sendErr := sendTextMessage(api, chatID, err.Error()); sendErr != nil {
			return fmt.Errorf("telegram send failed: %w", sendErr)
		}
		return nil
	}

	if role, name, ok := service.MatchPrivilegedContact(phone); ok {
		session.UserRole = role
		session.UserName = name
		session.UserPhone = phone
		session.SupplierAuthMode = SupplierAuthModeNone
		session.SupplierAuthStep = SupplierAuthStepNone
		session.SupplierAuthName = ""
		session.SupplierAuthPhone = ""
		session.SupplierAuthPromptMsgID = 0
		session.SupplierAuthInputMsgID = 0
		if role == UserRoleAdmin {
			session.AdminAuthed = true
		}
		if role == UserRoleWerka {
			service.BindWerkaTelegramID(principalID)
		}
		service.sessions.Upsert(principalID, session)
		if _, err := sendTextMessageWithReplyMarkup(api, chatID, authenticatedStartText(session), removeKeyboard()); err != nil {
			return fmt.Errorf("telegram send failed: %w", err)
		}
		return nil
	}

	supplier, found, err := service.FindSupplierByPhone(ctx, principalID, phone)
	if err != nil {
		return fmt.Errorf("supplier lookup failed: %w", err)
	}
	log.Printf("supplier contact lookup phone=%s found=%v role=%s", phone, found, session.UserRole)

	if !found {
		if _, err := sendTextMessageWithReplyMarkup(api, chatID, "Telefon raqamingiz tizimda topilmadi.", removeKeyboard()); err != nil {
			return fmt.Errorf("telegram send failed: %w", err)
		}
		return nil
	}

	promptText := "Telefon topildi. Ismingizni kiriting:"
	session.SupplierAuthMode = SupplierAuthModeRegister
	session.SupplierAuthStep = SupplierAuthStepAwaitingName

	promptID, err := sendTextMessageWithReplyMarkup(api, chatID, promptText, removeKeyboard())
	if err != nil {
		return fmt.Errorf("telegram send failed: %w", err)
	}
	session.UserPhone = phone
	session.SupplierAuthName = supplier.Name
	session.SupplierAuthPhone = supplier.Phone
	session.SupplierAuthPromptMsgID = promptID
	session.SupplierAuthInputMsgID = 0
	service.sessions.Upsert(principalID, session)
	return nil
}

func handleCommand(ctx context.Context, api *tgbotapi.BotAPI, service *Service, message *tgbotapi.Message, principalID, chatID int64, session LoginSession) error {
	_ = ctx
	switch message.Command() {
	case "start":
		deleteMessageBestEffort(api, chatID, message.MessageID)

		if session.UserRole != UserRoleNone {
			text := authenticatedStartText(session)
			welcomeID, err := sendTextMessage(api, chatID, text)
			if err != nil {
				return fmt.Errorf("telegram send failed: %w", err)
			}
			session.WelcomeMessageID = welcomeID
			service.sessions.Upsert(principalID, session)
			return nil
		}

		welcomeID, err := sendTextMessageWithReplyMarkup(api, chatID, "Telefon raqamingizni yuboring.", contactRequestKeyboard())
		if err != nil {
			return fmt.Errorf("telegram send failed: %w", err)
		}

		service.sessions.Upsert(principalID, LoginSession{Step: LoginStepNone, WelcomeMessageID: welcomeID})
		return nil

	case "login":
		deleteMessageBestEffort(api, chatID, message.MessageID)

		text := service.HandleLoginCommand(principalID)
		promptID, err := sendTextMessage(api, chatID, text)
		if err != nil {
			return fmt.Errorf("telegram send failed: %w", err)
		}

		session = LoginSession{}
		session.PromptMessageID = promptID
		session.Step = LoginStepAwaitingURL
		service.sessions.Upsert(principalID, session)
		return nil

	case "settings":
		deleteMessageBestEffort(api, chatID, message.MessageID)
		deleteMessageBestEffort(api, chatID, session.SettingsPanelID)
		session.SettingsPanelID = 0
		session.SettingsStep = SettingsStepAwaitingPassword
		session.SettingsAuthed = false
		session.SettingsSelect = SettingsSelectionNone
		session.ActionStep = ActionStepNone
		session.ActionType = ""
		session.SelectedItemCode = ""
		session.SelectedUOM = ""
		session.RequireUOMFirst = false
		session.PromptMessageID = 0
		session.WelcomeMessageID = 0
		service.sessions.Upsert(principalID, session)
		return ensurePanelText(api, chatID, &session, service, principalID, "Settings parolini kiriting:", tgbotapi.InlineKeyboardMarkup{})

	case "admin":
		deleteMessageBestEffort(api, chatID, message.MessageID)
		if session.AdminAuthed {
			service.sessions.Upsert(principalID, session)
			return ensureAdminPanelText(api, chatID, &session, service, principalID, adminWelcomeText(), tgbotapi.InlineKeyboardMarkup{})
		}
		if session.UserRole == UserRoleAdmin {
			session.AdminAuthed = true
			service.sessions.Upsert(principalID, session)
			return ensureAdminPanelText(api, chatID, &session, service, principalID, adminWelcomeText(), tgbotapi.InlineKeyboardMarkup{})
		}
		deleteMessageBestEffort(api, chatID, session.AdminPanelID)
		session.AdminPanelID = 0
		session.AdminAuthed = false
		if service.IsAdminConfigured() {
			session.AdminStep = AdminStepAwaitingPassword
			service.sessions.Upsert(principalID, session)
			return ensureAdminPanelText(api, chatID, &session, service, principalID, "Admin parolini kiriting:", tgbotapi.InlineKeyboardMarkup{})
		}

		session.AdminStep = AdminStepAwaitingSetupPassword
		service.sessions.Upsert(principalID, session)
		return ensureAdminPanelText(api, chatID, &session, service, principalID, "Admin parol yarating:", tgbotapi.InlineKeyboardMarkup{})

	case "supplier":
		deleteMessageBestEffort(api, chatID, message.MessageID)
		if !session.AdminAuthed {
			if _, err := sendTextMessage(api, chatID, "Iltimos, avval /admin qiling."); err != nil {
				return fmt.Errorf("telegram send failed: %w", err)
			}
			return nil
		}
		namePromptID, err := sendTextMessage(api, chatID, "Supplier ismini kiriting:")
		if err != nil {
			return fmt.Errorf("telegram send failed: %w", err)
		}
		session.SupplierStep = SupplierStepAwaitingName
		session.SupplierName = ""
		session.SupplierNameMsgID = namePromptID
		session.SupplierPhoneMsgID = 0
		session.SupplierNameInputMsgID = 0
		session.SupplierPhoneInputMsgID = 0
		service.sessions.Upsert(principalID, session)
		return nil

	case "suplier_list", "supplier_list":
		deleteMessageBestEffort(api, chatID, message.MessageID)
		if !session.AdminAuthed {
			if _, err := sendTextMessage(api, chatID, "Iltimos, avval /admin qiling."); err != nil {
				return fmt.Errorf("telegram send failed: %w", err)
			}
			return nil
		}
		session.AdminSupplierListActive = true
		service.sessions.Upsert(principalID, session)
		return ensureAdminPanelText(api, chatID, &session, service, principalID, "Supplier tanlang. Quyidagi 'Supplier' tugmasini bosing.", supplierPickerKeyboard())

	case "adminka":
		deleteMessageBestEffort(api, chatID, message.MessageID)
		if !session.AdminAuthed {
			if _, err := sendTextMessage(api, chatID, "Iltimos, avval /admin qiling."); err != nil {
				return fmt.Errorf("telegram send failed: %w", err)
			}
			return nil
		}
		phonePromptID, err := sendTextMessage(api, chatID, "Adminka telefon raqamini kiriting. Format: +998901234567")
		if err != nil {
			return fmt.Errorf("telegram send failed: %w", err)
		}
		session.ContactSetupStep = ContactSetupStepAwaitingPhone
		session.ContactSetupKind = ContactSetupKindAdminka
		session.ContactPhone = ""
		session.ContactPhoneMsgID = phonePromptID
		session.ContactNameMsgID = 0
		session.ContactPhoneInputMsgID = 0
		session.ContactNameInputMsgID = 0
		service.sessions.Upsert(principalID, session)
		return nil

	case "werka":
		deleteMessageBestEffort(api, chatID, message.MessageID)
		if !session.AdminAuthed {
			if _, err := sendTextMessage(api, chatID, "Iltimos, avval /admin qiling."); err != nil {
				return fmt.Errorf("telegram send failed: %w", err)
			}
			return nil
		}
		phonePromptID, err := sendTextMessage(api, chatID, "Omborchi telefon raqamini kiriting. Format: +998901234567")
		if err != nil {
			return fmt.Errorf("telegram send failed: %w", err)
		}
		session.ContactSetupStep = ContactSetupStepAwaitingPhone
		session.ContactSetupKind = ContactSetupKindWerka
		session.ContactPhone = ""
		session.ContactPhoneMsgID = phonePromptID
		session.ContactNameMsgID = 0
		session.ContactPhoneInputMsgID = 0
		session.ContactNameInputMsgID = 0
		service.sessions.Upsert(principalID, session)
		return nil

	case "wer":
		deleteMessageBestEffort(api, chatID, message.MessageID)
		if !session.SettingsAuthed {
			return ensurePanelText(api, chatID, &session, service, principalID, "Iltimos, avval /settings ga kirib parolni kiriting.", tgbotapi.InlineKeyboardMarkup{})
		}
		session.SettingsSelect = SettingsSelectionWarehouse
		service.sessions.Upsert(principalID, session)
		return ensurePanelText(api, chatID, &session, service, principalID, "Ombor tanlang. Quyidagi 'Ombor' tugmasini bosing.", warehousePickerKeyboard())

	case "uom":
		deleteMessageBestEffort(api, chatID, message.MessageID)
		if !session.SettingsAuthed {
			return ensurePanelText(api, chatID, &session, service, principalID, "Iltimos, avval /settings ga kirib parolni kiriting.", tgbotapi.InlineKeyboardMarkup{})
		}
		session.SettingsSelect = SettingsSelectionUOM
		service.sessions.Upsert(principalID, session)
		return ensurePanelText(api, chatID, &session, service, principalID, "Default UOM tanlang. Quyidagi 'UOM' tugmasini bosing.", uomPickerKeyboard())

	case "dispatch":
		deleteMessageBestEffort(api, chatID, message.MessageID)
		return beginSupplierDispatch(api, service, chatID, principalID, session)

	case "not", "bildirishnoma":
		deleteMessageBestEffort(api, chatID, message.MessageID)
		return openPendingReceipts(api, service, chatID, principalID, session)

	case "stock":
		deleteMessageBestEffort(api, chatID, message.MessageID)
		if !service.EnsureCredentials(principalID) {
			if _, err := sendTextMessage(api, chatID, "Iltimos, avval /login qiling."); err != nil {
				return fmt.Errorf("telegram send failed: %w", err)
			}
			return nil
		}

		promptID, err := sendTextMessageWithKeyboard(api, chatID, "Harakatni tanlang:", actionTypeKeyboard())
		if err != nil {
			return fmt.Errorf("telegram send failed: %w", err)
		}
		clearRecentMessagesAsync(api, chatID, message.MessageID, 40)

		session.Step = LoginStepNone
		session.PromptMessageID = promptID
		session.WelcomeMessageID = 0
		session.ActionStep = ActionStepAwaitingType
		session.ActionType = ""
		session.SelectedItemCode = ""
		session.SelectedUOM = ""
		session.RequireUOMFirst = false
		service.sessions.Upsert(principalID, session)
		return nil

	case "logout":
		deleteMessageBestEffort(api, chatID, message.MessageID)
		if session.SettingsPanelID > 0 {
			deleteMessageBestEffort(api, chatID, session.SettingsPanelID)
		}
		if session.AdminPanelID > 0 {
			deleteMessageBestEffort(api, chatID, session.AdminPanelID)
		}
		clearSupplierMessages(api, chatID, session)
		clearContactSetupMessages(api, chatID, session)
		clearRecentMessagesAsync(api, chatID, message.MessageID, 40)
		adminAuthed := session.AdminAuthed
		session.SettingsStep = SettingsStepNone
		session.SettingsAuthed = false
		session.SettingsSelect = SettingsSelectionNone
		session.SettingsPanelID = 0
		session.AdminStep = AdminStepNone
		session.AdminAuthed = false
		session.AdminPanelID = 0
		session.AdminSupplierListActive = false
		session.SupplierStep = SupplierStepNone
		session.SupplierName = ""
		session.SupplierNameMsgID = 0
		session.SupplierPhoneMsgID = 0
		session.SupplierNameInputMsgID = 0
		session.SupplierPhoneInputMsgID = 0
		session.ContactSetupStep = ContactSetupStepNone
		session.ContactSetupKind = ContactSetupKindNone
		session.ContactPhone = ""
		session.ContactPhoneMsgID = 0
		session.ContactNameMsgID = 0
		session.ContactPhoneInputMsgID = 0
		session.ContactNameInputMsgID = 0
		clearDispatchState(&session)
		clearNoticeState(&session)
		session.SupplierAuthMode = SupplierAuthModeNone
		session.SupplierAuthStep = SupplierAuthStepNone
		session.SupplierAuthName = ""
		session.SupplierAuthPhone = ""
		session.SupplierAuthPromptMsgID = 0
		session.SupplierAuthInputMsgID = 0
		service.sessions.Upsert(principalID, session)
		logoutText := "Siz settings dan chiqdingiz."
		if adminAuthed {
			logoutText = "Siz admin paneldan chiqdingiz."
		}
		if _, err := sendTextMessage(api, chatID, logoutText); err != nil {
			return fmt.Errorf("telegram send failed: %w", err)
		}
		return nil

	default:
		deleteMessageBestEffort(api, chatID, message.MessageID)
		if _, err := sendTextMessage(api, chatID, "Noma'lum buyruq. Mavjud buyruqlar: /start, /login, /stock, /dispatch, /not, /settings, /admin, /supplier, /suplier_list, /adminka, /werka"); err != nil {
			return fmt.Errorf("telegram send failed: %w", err)
		}
		return nil
	}
}

func handleCallbackQuery(ctx context.Context, api *tgbotapi.BotAPI, service *Service, cb *tgbotapi.CallbackQuery) error {
	_, _ = api.Request(tgbotapi.NewCallback(cb.ID, ""))
	if cb.Message == nil {
		return nil
	}

	chatID := cb.Message.Chat.ID
	principalID := cb.From.ID

	session, ok := service.sessions.Get(principalID)
	if !ok {
		session = LoginSession{}
	}

	switch cb.Data {
	case callbackStartAction:
		if !service.EnsureCredentials(principalID) {
			if _, err := sendTextMessage(api, chatID, "Iltimos, avval /login qiling."); err != nil {
				return fmt.Errorf("telegram send failed: %w", err)
			}
			return nil
		}

		promptID, err := sendTextMessageWithKeyboard(api, chatID, "Harakatni tanlang:", actionTypeKeyboard())
		if err != nil {
			return fmt.Errorf("telegram send failed: %w", err)
		}
		clearRecentMessagesAsync(api, chatID, cb.Message.MessageID, 40)

		session.Step = LoginStepNone
		session.PromptMessageID = promptID
		session.WelcomeMessageID = 0
		session.ActionStep = ActionStepAwaitingType
		session.ActionType = ""
		session.SelectedItemCode = ""
		session.SelectedUOM = ""
		service.sessions.Upsert(principalID, session)
		return nil

	case callbackAgainAction:
		if !service.EnsureCredentials(principalID) {
			if _, err := sendTextMessage(api, chatID, "Iltimos, avval /login qiling."); err != nil {
				return fmt.Errorf("telegram send failed: %w", err)
			}
			return nil
		}
		if session.LastActionType == "" || strings.TrimSpace(session.LastItemCode) == "" {
			promptID, err := sendTextMessageWithKeyboard(api, chatID, "Oldingi harakat topilmadi. Avval harakat turini tanlang:", actionTypeKeyboard())
			if err != nil {
				return fmt.Errorf("telegram send failed: %w", err)
			}
			session.Step = LoginStepNone
			session.PromptMessageID = promptID
			session.WelcomeMessageID = 0
			session.ActionStep = ActionStepAwaitingType
			session.ActionType = ""
			session.SelectedItemCode = ""
			session.SelectedUOM = ""
			session.RequireUOMFirst = false
			service.sessions.Upsert(principalID, session)
			return nil
		}

		session.Step = LoginStepNone
		session.PromptMessageID = cb.Message.MessageID
		session.WelcomeMessageID = 0
		session.ActionStep = ActionStepAwaitingQty
		session.ActionType = session.LastActionType
		session.SelectedItemCode = session.LastItemCode
		session.SelectedUOM = session.LastUOM
		session.RequireUOMFirst = false
		service.sessions.Upsert(principalID, session)

		text := fmt.Sprintf("Yana rejim.\nMahsulot: %s\nMiqdor kiriting (faqat 0 dan katta son).", session.LastItemCode)
		if err := editMessageText(api, chatID, cb.Message.MessageID, text); err != nil {
			return fmt.Errorf("telegram edit failed: %w", err)
		}
		return nil

	case callbackReceipt, callbackIssue:
		if session.PromptMessageID == 0 {
			session.PromptMessageID = cb.Message.MessageID
		}

		if cb.Data == callbackReceipt {
			session.ActionType = ActionTypeReceipt
		} else {
			session.ActionType = ActionTypeIssue
		}
		session.ActionStep = ActionStepAwaitingItem
		session.SelectedItemCode = ""
		session.SelectedUOM = ""
		session.RequireUOMFirst = false
		service.sessions.Upsert(principalID, session)

		if err := editMessageTextWithKeyboard(api, chatID, cb.Message.MessageID, "Mahsulot tanlaymiz. Quyidagi 'Mahsulot' tugmasini bosing.", itemPickerKeyboard()); err != nil {
			return fmt.Errorf("telegram edit failed: %w", err)
		}
		return nil

	case callbackDispatchConfirm:
		return handleDispatchConfirmCallback(ctx, api, service, chatID, principalID, session, cb.Message.MessageID)

	case callbackDispatchCancel:
		return handleDispatchCancelCallback(api, service, chatID, principalID, session, cb.Message.MessageID)
	}

	if strings.HasPrefix(cb.Data, callbackNoticeOpenPrefix) {
		receiptName := strings.TrimSpace(strings.TrimPrefix(cb.Data, callbackNoticeOpenPrefix))
		if receiptName == "" {
			return nil
		}
		return handleNoticeOpenCallback(ctx, api, service, chatID, principalID, session, receiptName, cb.Message.MessageID)
	}

	_ = ctx
	return nil
}

func handleInlineQuery(ctx context.Context, api *tgbotapi.BotAPI, service *Service, q *tgbotapi.InlineQuery) error {
	principalID := q.From.ID
	session, ok := service.sessions.Get(principalID)
	if !ok {
		return answerEmptyInline(api, q.ID)
	}

	if !service.EnsureCredentials(principalID) {
		return answerEmptyInline(api, q.ID)
	}
	creds, _ := service.creds.Get(principalID)

	if session.SupplierDispatchStep == SupplierDispatchStepAwaitingItem {
		query := normalizeInlineQuery(q.Query, "item")
		items, err := service.erp.SearchSupplierItems(ctx, creds.BaseURL, creds.APIKey, creds.APISecret, session.UserName, query, 20)
		if err != nil {
			return fmt.Errorf("supplier item search failed: %w", err)
		}
		results := make([]interface{}, 0, len(items))
		for _, item := range items {
			text := inlineItemPrefix + item.Code
			title := item.Code
			if item.Name != "" && item.Name != item.Code {
				title = fmt.Sprintf("%s - %s", item.Code, item.Name)
			}
			article := tgbotapi.NewInlineQueryResultArticle(item.Code, title, text)
			if item.UOM != "" {
				article.Description = "UOM: " + item.UOM
			}
			results = append(results, article)
		}
		return answerInline(api, q.ID, results)
	}

	if session.ActionStep == ActionStepAwaitingItem {
		query := normalizeInlineQuery(q.Query, "item")
		items, err := service.erp.SearchItems(ctx, creds.BaseURL, creds.APIKey, creds.APISecret, query, 20)
		if err != nil {
			return fmt.Errorf("item search failed: %w", err)
		}
		results := make([]interface{}, 0, len(items))
		for _, item := range items {
			text := inlineItemPrefix + item.Code
			title := item.Code
			if item.Name != "" && item.Name != item.Code {
				title = fmt.Sprintf("%s - %s", item.Code, item.Name)
			}
			article := tgbotapi.NewInlineQueryResultArticle(item.Code, title, text)
			if item.UOM != "" {
				article.Description = "UOM: " + item.UOM
			}
			results = append(results, article)
		}
		return answerInline(api, q.ID, results)
	}

	if session.ActionStep == ActionStepAwaitingUOM {
		query := normalizeInlineQuery(q.Query, "uom")
		uoms, err := service.erp.SearchUOMs(ctx, creds.BaseURL, creds.APIKey, creds.APISecret, query, 20)
		if err != nil {
			return fmt.Errorf("uom search failed: %w", err)
		}
		results := make([]interface{}, 0, len(uoms))
		for _, uom := range uoms {
			article := tgbotapi.NewInlineQueryResultArticle(uom.Name, uom.Name, inlineUOMPrefix+uom.Name)
			results = append(results, article)
		}
		return answerInline(api, q.ID, results)
	}

	if session.AdminAuthed && session.AdminSupplierListActive {
		query := strings.ToLower(normalizeInlineQuery(q.Query, "sup"))
		results := make([]interface{}, 0, 20)

		baseURL, apiKey, apiSecret, ok := service.erpCredentials(principalID)
		if !ok {
			return answerEmptyInline(api, q.ID)
		}
		erpSuppliers, err := service.erp.SearchSuppliers(ctx, baseURL, apiKey, apiSecret, query, 20)
		if err != nil {
			return fmt.Errorf("erp supplier search failed: %w", err)
		}
		for _, supplier := range erpSuppliers {
			article := tgbotapi.NewInlineQueryResultArticle(
				supplier.Name,
				supplier.Name,
				inlineSupplierPrefix+supplier.Name,
			)
			article.Description = strings.TrimSpace(supplier.Phone)
			if article.Description == "" {
				article.Description = "Mobile No yo'q"
			}
			results = append(results, article)
		}
		return answerInline(api, q.ID, results)
	}

	if session.WarehouseNoticeListActive && (session.UserRole == UserRoleWerka || session.UserRole == UserRoleAdmin || session.AdminAuthed) {
		items, err := service.erp.ListPendingPurchaseReceipts(ctx, creds.BaseURL, creds.APIKey, creds.APISecret, 20)
		if err != nil {
			return fmt.Errorf("pending receipt search failed: %w", err)
		}
		query := strings.ToLower(normalizeInlineQuery(q.Query, "notice"))
		results := make([]interface{}, 0, len(items))
		for _, item := range items {
			title := fmt.Sprintf("%s | %s", item.Supplier, item.ItemCode)
			if query != "" {
				candidate := strings.ToLower(strings.Join([]string{item.Supplier, item.ItemCode, item.ItemName, item.Name}, " "))
				if !strings.Contains(candidate, query) {
					continue
				}
			}
			article := tgbotapi.NewInlineQueryResultArticle(
				item.Name,
				title,
				inlineNoticePrefix+item.Name,
			)
			article.Description = fmt.Sprintf("%.2f %s", item.Qty, item.UOM)
			results = append(results, article)
		}
		return answerInline(api, q.ID, results)
	}

	if session.SettingsAuthed && session.SettingsSelect == SettingsSelectionWarehouse {
		query := normalizeInlineQuery(q.Query, "wer")
		warehouses, err := service.erp.SearchWarehouses(ctx, creds.BaseURL, creds.APIKey, creds.APISecret, query, 20)
		if err != nil {
			return fmt.Errorf("warehouse search failed: %w", err)
		}
		results := make([]interface{}, 0, len(warehouses))
		for _, wh := range warehouses {
			article := tgbotapi.NewInlineQueryResultArticle(wh.Name, wh.Name, inlineWarehousePrefix+wh.Name)
			results = append(results, article)
		}
		return answerInline(api, q.ID, results)
	}

	if session.SettingsAuthed && session.SettingsSelect == SettingsSelectionUOM {
		query := normalizeInlineQuery(q.Query, "uom")
		uoms, err := service.erp.SearchUOMs(ctx, creds.BaseURL, creds.APIKey, creds.APISecret, query, 20)
		if err != nil {
			return fmt.Errorf("uom search failed: %w", err)
		}
		results := make([]interface{}, 0, len(uoms))
		for _, uom := range uoms {
			article := tgbotapi.NewInlineQueryResultArticle(uom.Name, uom.Name, inlineUOMPrefix+uom.Name)
			results = append(results, article)
		}
		return answerInline(api, q.ID, results)
	}

	return answerEmptyInline(api, q.ID)
}

func buildStockEntryInput(service *Service, session LoginSession, qty float64) (erpnext.CreateStockEntryInput, error) {
	targetWarehouse, sourceWarehouse, defaultUOM := service.Defaults()
	if strings.TrimSpace(defaultUOM) == "" {
		defaultUOM = "Kg"
	}

	input := erpnext.CreateStockEntryInput{
		ItemCode: session.SelectedItemCode,
		Qty:      qty,
		UOM:      defaultUOM,
	}
	if strings.TrimSpace(session.SelectedUOM) != "" {
		input.UOM = strings.TrimSpace(session.SelectedUOM)
	}

	switch session.ActionType {
	case ActionTypeReceipt:
		if strings.TrimSpace(targetWarehouse) == "" {
			return erpnext.CreateStockEntryInput{}, fmt.Errorf("ERP_DEFAULT_TARGET_WAREHOUSE sozlanmagan")
		}
		input.EntryType = "Material Receipt"
		input.TargetWarehouse = targetWarehouse
	case ActionTypeIssue:
		if strings.TrimSpace(sourceWarehouse) == "" {
			return erpnext.CreateStockEntryInput{}, fmt.Errorf("ERP_DEFAULT_SOURCE_WAREHOUSE sozlanmagan")
		}
		input.EntryType = "Material Issue"
		input.SourceWarehouse = sourceWarehouse
	default:
		return erpnext.CreateStockEntryInput{}, fmt.Errorf("harakat turi tanlanmagan")
	}

	return input, nil
}
