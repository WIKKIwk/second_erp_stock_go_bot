package bot

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	adminsvc "erpnext_stock_telegram/internal/admin"
	"erpnext_stock_telegram/internal/suplier"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	callbackStartAction      = "action:start"
	callbackAgainAction      = "action:again"
	callbackReceipt          = "action:type:receipt"
	callbackIssue            = "action:type:issue"
	callbackDispatchConfirm  = "dispatch:confirm"
	callbackDispatchCancel   = "dispatch:cancel"
	callbackNoticeOpenPrefix = "notice:open:"

	inlineItemPrefix      = "item::"
	inlineSupplierPrefix  = "sup::"
	inlineWarehousePrefix = "wer::"
	inlineUOMPrefix       = "uom::"
)

func RunTelegramLoop(ctx context.Context, token string, service *Service) error {
	log.Println("Telegram bot initialization started")
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return fmt.Errorf("telegram bot init failed: %w", err)
	}
	log.Printf("Telegram bot connected as @%s (id=%d)", api.Self.UserName, api.Self.ID)

	updateCfg := tgbotapi.NewUpdate(0)
	updateCfg.Timeout = 30
	log.Println("Telegram bot is running and waiting for updates")

	for {
		select {
		case <-ctx.Done():
			log.Println("Telegram bot shutdown signal received")
			return nil
		default:
		}

		updates, err := api.GetUpdates(updateCfg)
		if err != nil {
			if strings.Contains(err.Error(), "Conflict: terminated by other getUpdates request") {
				return fmt.Errorf("another bot instance is using this token; stop the other instance and retry")
			}
			log.Printf("failed to get updates, retrying in 3 seconds: %v", err)
			time.Sleep(3 * time.Second)
			continue
		}

		for _, update := range updates {
			if update.UpdateID >= updateCfg.Offset {
				updateCfg.Offset = update.UpdateID + 1
			}

			if update.CallbackQuery != nil {
				if err := handleCallbackQuery(ctx, api, service, update.CallbackQuery); err != nil {
					log.Printf("callback handling failed: %v", err)
				}
				continue
			}

			if update.InlineQuery != nil {
				if err := handleInlineQuery(ctx, api, service, update.InlineQuery); err != nil {
					log.Printf("inline query handling failed: %v", err)
				}
				continue
			}

			if update.Message == nil {
				continue
			}

			if err := handleIncomingMessage(ctx, api, service, update.Message); err != nil {
				log.Printf("message handling failed: %v", err)
			}
		}
	}
}

func handleIncomingMessage(ctx context.Context, api *tgbotapi.BotAPI, service *Service, message *tgbotapi.Message) error {
	chatID := message.Chat.ID
	principalID := chatID
	if message.From != nil {
		principalID = message.From.ID
	}

	session, ok := service.sessions.Get(principalID)
	if !ok {
		session = LoginSession{}
	}

	if message.IsCommand() {
		if session.AdminAuthed && !isAdminCommand(message.Command()) {
			deleteMessageBestEffort(api, chatID, message.MessageID)
			return ensureAdminPanelText(api, chatID, &session, service, principalID, adminOnlyCommandText(), tgbotapi.InlineKeyboardMarkup{})
		}
		interruptSessionMessages(api, chatID, session, message.Command())
		session = resetSessionForCommand(session, message.Command())
		service.sessions.Upsert(principalID, session)
		return handleCommand(ctx, api, service, message, principalID, chatID, session)
	}

	if message.Contact != nil {
		return handleSharedContact(ctx, api, service, message, principalID, chatID, session)
	}

	if session.AdminStep == AdminStepAwaitingSetupPassword {
		deleteMessageBestEffort(api, chatID, message.MessageID)
		if err := service.SetAdminPassword(message.Text); err != nil {
			return ensureAdminPanelText(api, chatID, &session, service, principalID, err.Error()+"\nQayta kiriting:", tgbotapi.InlineKeyboardMarkup{})
		}

		session.AdminStep = AdminStepNone
		session.AdminAuthed = true
		service.sessions.Upsert(principalID, session)
		return ensureAdminPanelText(api, chatID, &session, service, principalID, adminWelcomeText(), tgbotapi.InlineKeyboardMarkup{})
	}

	if session.AdminStep == AdminStepAwaitingPassword {
		deleteMessageBestEffort(api, chatID, message.MessageID)
		if !service.IsAdminPasswordValid(message.Text) {
			return ensureAdminPanelText(api, chatID, &session, service, principalID, "Admin parol noto'g'ri. Qayta kiriting:", tgbotapi.InlineKeyboardMarkup{})
		}

		session.AdminStep = AdminStepNone
		session.AdminAuthed = true
		service.sessions.Upsert(principalID, session)
		return ensureAdminPanelText(api, chatID, &session, service, principalID, adminWelcomeText(), tgbotapi.InlineKeyboardMarkup{})
	}

	if session.ContactSetupStep != ContactSetupStepNone {
		switch session.ContactSetupStep {
		case ContactSetupStepAwaitingPhone:
			phone, err := adminsvc.NormalizeContactPhone(message.Text)
			if err != nil {
				deleteMessageBestEffort(api, chatID, message.MessageID)
				if session.ContactPhoneMsgID > 0 {
					_ = editMessageText(api, chatID, session.ContactPhoneMsgID, err.Error()+"\nQayta kiriting. Format: +998901234567")
				}
				return nil
			}

			session.ContactPhone = phone
			session.ContactPhoneInputMsgID = message.MessageID
			session.ContactSetupStep = ContactSetupStepAwaitingName
			namePromptID, err := sendTextMessage(api, chatID, contactNamePromptText(session.ContactSetupKind))
			if err != nil {
				return fmt.Errorf("telegram send failed: %w", err)
			}
			session.ContactNameMsgID = namePromptID
			service.sessions.Upsert(principalID, session)
			return nil

		case ContactSetupStepAwaitingName:
			if err := service.SaveContact(session.ContactSetupKind, session.ContactPhone, message.Text); err != nil {
				deleteMessageBestEffort(api, chatID, message.MessageID)
				if session.ContactNameMsgID > 0 {
					_ = editMessageText(api, chatID, session.ContactNameMsgID, err.Error()+"\nQayta kiriting:")
				}
				return nil
			}

			session.ContactNameInputMsgID = message.MessageID
			clearContactSetupMessages(api, chatID, session)
			successText := contactSuccessText(session.ContactSetupKind)
			session.ContactSetupStep = ContactSetupStepNone
			session.ContactSetupKind = ContactSetupKindNone
			session.ContactPhone = ""
			session.ContactPhoneMsgID = 0
			session.ContactNameMsgID = 0
			session.ContactPhoneInputMsgID = 0
			session.ContactNameInputMsgID = 0
			service.sessions.Upsert(principalID, session)
			if _, err := sendTextMessage(api, chatID, successText); err != nil {
				return fmt.Errorf("telegram send failed: %w", err)
			}
			return nil
		}
	}

	if session.SupplierAuthStep != SupplierAuthStepNone {
		switch session.SupplierAuthStep {
		case SupplierAuthStepAwaitingName:
			if !supplierNameMatches(session.SupplierAuthName, message.Text) {
				deleteMessageBestEffort(api, chatID, message.MessageID)
				if session.SupplierAuthPromptMsgID > 0 {
					_ = editMessageText(api, chatID, session.SupplierAuthPromptMsgID, "Ism mos kelmadi. Qayta kiriting:")
				}
				return nil
			}

			session.SupplierAuthInputMsgID = message.MessageID
			session.SupplierAuthMode = SupplierAuthModeRegister
			session.SupplierAuthStep = SupplierAuthStepAwaitingPassword
			service.sessions.Upsert(principalID, session)
			if session.SupplierAuthPromptMsgID > 0 {
				_ = editMessageText(api, chatID, session.SupplierAuthPromptMsgID, "Siz supplier sifatida ro'yxatdan o'tyapsiz.\nYangi kuchli parol qo'ying: harf va son aralash bo'lsin.")
			}
			return nil

		case SupplierAuthStepAwaitingPassword:
			deleteMessageBestEffort(api, chatID, message.MessageID)

			successText := "Kirish muvaffaqiyatli. Siz supplier sifatida kirdingiz."
			switch session.SupplierAuthMode {
			case SupplierAuthModeRegister:
				if err := validateStrongPassword(message.Text); err != nil {
					if session.SupplierAuthPromptMsgID > 0 {
						_ = editMessageText(api, chatID, session.SupplierAuthPromptMsgID, err.Error()+"\nQayta kiriting.")
					}
					return nil
				}
				if _, err := service.RegisterSupplierAuth(ctx, session.SupplierAuthPhone, principalID, message.Text); err != nil {
					if session.SupplierAuthPromptMsgID > 0 {
						_ = editMessageText(api, chatID, session.SupplierAuthPromptMsgID, userFacingSupplierAuthError(err))
					}
					return nil
				}
				successText = "Ro'yxatdan o'tish yakunlandi. Siz supplier sifatida kirdingiz."
			case SupplierAuthModeLogin:
				if _, err := service.AuthenticateSupplier(ctx, session.SupplierAuthPhone, principalID, message.Text); err != nil {
					if session.SupplierAuthPromptMsgID > 0 {
						_ = editMessageText(api, chatID, session.SupplierAuthPromptMsgID, userFacingSupplierAuthError(err))
					}
					return nil
				}
			default:
				if session.SupplierAuthPromptMsgID > 0 {
					_ = editMessageText(api, chatID, session.SupplierAuthPromptMsgID, "Supplier auth holati topilmadi. /start ni qayta yuboring.")
				}
				return nil
			}

			session.UserRole = UserRoleSupplier
			session.UserName = session.SupplierAuthName
			session.UserPhone = session.SupplierAuthPhone
			session.SupplierAuthInputMsgID = message.MessageID
			if session.SupplierAuthPromptMsgID > 0 {
				deleteMessageBestEffort(api, chatID, session.SupplierAuthPromptMsgID)
			}
			deleteMessageBestEffort(api, chatID, session.SupplierAuthInputMsgID)
			session.SupplierAuthMode = SupplierAuthModeNone
			session.SupplierAuthStep = SupplierAuthStepNone
			session.SupplierAuthName = ""
			session.SupplierAuthPhone = ""
			session.SupplierAuthPromptMsgID = 0
			session.SupplierAuthInputMsgID = 0
			service.sessions.Upsert(principalID, session)
			successText = successText + "\n\n" + authenticatedStartText(session)
			if _, err := sendTextMessageWithReplyMarkup(api, chatID, successText, removeKeyboard()); err != nil {
				return fmt.Errorf("telegram send failed: %w", err)
			}
			return nil
		}
	}

	if session.AdminAuthed && session.SupplierStep != SupplierStepNone {
		switch session.SupplierStep {
		case SupplierStepAwaitingName:
			name := strings.TrimSpace(message.Text)
			if name == "" {
				deleteMessageBestEffort(api, chatID, message.MessageID)
				if session.SupplierNameMsgID > 0 {
					_ = editMessageText(api, chatID, session.SupplierNameMsgID, "Supplier ismi bo'sh bo'lmasligi kerak.\nQayta kiriting:")
				}
				return nil
			}

			session.SupplierName = name
			session.SupplierNameInputMsgID = message.MessageID
			session.SupplierStep = SupplierStepAwaitingPhone
			phonePromptID, err := sendTextMessage(api, chatID, "Telefon raqam kiriting. Format: +998901234567")
			if err != nil {
				return fmt.Errorf("telegram send failed: %w", err)
			}
			session.SupplierPhoneMsgID = phonePromptID
			service.sessions.Upsert(principalID, session)
			return nil

		case SupplierStepAwaitingPhone:
			_, err := service.AddSupplierWithERP(ctx, principalID, session.SupplierName, message.Text)
			if err != nil {
				deleteMessageBestEffort(api, chatID, message.MessageID)
				if session.SupplierPhoneMsgID > 0 {
					_ = editMessageText(api, chatID, session.SupplierPhoneMsgID, err.Error()+"\nQayta kiriting. Format: +998901234567")
				}
				return nil
			}

			session.SupplierPhoneInputMsgID = message.MessageID
			clearSupplierMessages(api, chatID, session)
			session.SupplierStep = SupplierStepNone
			session.SupplierName = ""
			session.SupplierNameMsgID = 0
			session.SupplierPhoneMsgID = 0
			session.SupplierNameInputMsgID = 0
			session.SupplierPhoneInputMsgID = 0
			service.sessions.Upsert(principalID, session)
			if _, err := sendTextMessage(api, chatID, "Supplier muvaffaqiyatli qo'shildi."); err != nil {
				return fmt.Errorf("telegram send failed: %w", err)
			}
			return nil
		}
	}

	if session.AdminAuthed && session.AdminSupplierListActive {
		deleteMessageBestEffort(api, chatID, message.MessageID)
		selected, parsed := parseInlineSupplierValue(message.Text)
		if !parsed {
			return ensureAdminPanelText(api, chatID, &session, service, principalID, "Iltimos, supplierni inline menyudan tanlang.", supplierPickerKeyboard())
		}

		var supplier suplier.Supplier
		var found bool

		suppliers, err := service.ListSuppliers(ctx)
		if err != nil && !strings.Contains(err.Error(), "supplier service is not configured") {
			return fmt.Errorf("supplier list failed: %w", err)
		}
		for _, item := range suppliers {
			if strings.EqualFold(strings.TrimSpace(item.Phone), strings.TrimSpace(selected)) ||
				strings.EqualFold(strings.TrimSpace(item.Name), strings.TrimSpace(selected)) {
				supplier = item
				found = true
				break
			}
		}

		if !found && service.EnsureCredentials(principalID) {
			creds, _ := service.creds.Get(principalID)
			erpSuppliers, err := service.erp.SearchSuppliers(ctx, creds.BaseURL, creds.APIKey, creds.APISecret, selected, 5)
			if err != nil {
				return fmt.Errorf("erp supplier search failed: %w", err)
			}
			for _, item := range erpSuppliers {
				if strings.EqualFold(strings.TrimSpace(item.Name), strings.TrimSpace(selected)) {
					supplier = suplier.Supplier{Ref: item.ID, Name: item.Name, Phone: item.Phone}
					found = true
					break
				}
			}
			if !found && len(erpSuppliers) > 0 {
				supplier = suplier.Supplier{Ref: erpSuppliers[0].ID, Name: erpSuppliers[0].Name, Phone: erpSuppliers[0].Phone}
				found = true
			}
		}

		if !found {
			return ensureAdminPanelText(api, chatID, &session, service, principalID, "Supplier topilmadi. Qayta tanlang.", supplierPickerKeyboard())
		}

		accessMessage, err := suplier.SupplierAccessMessage(supplier)
		if err != nil {
			return fmt.Errorf("supplier access code generation failed: %w", err)
		}

		session.AdminSupplierListActive = false
		service.sessions.Upsert(principalID, session)

		if _, err := sendHTMLMessage(
			api,
			chatID,
			strings.Replace(accessMessage, "Code: ", "Code: <code>", 1)+"</code>",
		); err != nil {
			return fmt.Errorf("telegram send failed: %w", err)
		}

		return ensureAdminPanelText(api, chatID, &session, service, principalID, "Supplier code yuborildi.\nYana ko'rish uchun /suplier_list", tgbotapi.InlineKeyboardMarkup{})
	}

	if session.SettingsStep == SettingsStepAwaitingPassword {
		deleteMessageBestEffort(api, chatID, message.MessageID)
		if !service.IsSettingsPasswordValid(message.Text) {
			return ensurePanelText(api, chatID, &session, service, principalID, "Parol noto'g'ri. Qayta kiriting:", tgbotapi.InlineKeyboardMarkup{})
		}

		session.SettingsStep = SettingsStepNone
		session.SettingsAuthed = true
		session.SettingsSelect = SettingsSelectionNone
		session.ActionStep = ActionStepNone
		session.ActionType = ""
		session.SelectedItemCode = ""
		session.SelectedUOM = ""
		session.RequireUOMFirst = false
		service.sessions.Upsert(principalID, session)

		welcome := "Salom, Admin. Settings panelga xush kelibsiz.\n/wer - default ombor\n/uom - default UOM\n/logout - paneldan chiqish"
		return ensurePanelText(api, chatID, &session, service, principalID, welcome, tgbotapi.InlineKeyboardMarkup{})
	}

	if session.SettingsAuthed && session.SettingsSelect != SettingsSelectionNone {
		deleteMessageBestEffort(api, chatID, message.MessageID)
		switch session.SettingsSelect {
		case SettingsSelectionWarehouse:
			warehouse, parsed := parseInlineWarehouseName(message.Text)
			if !parsed {
				return ensurePanelText(api, chatID, &session, service, principalID, "Iltimos, omborni inline menyudan tanlang.", warehousePickerKeyboard())
			}
			service.SetDefaultWarehouse(warehouse)
			session.SettingsSelect = SettingsSelectionNone
			service.sessions.Upsert(principalID, session)
			return ensurePanelText(api, chatID, &session, service, principalID, "Tanlandi: "+warehouse+"\n/wer /uom /logout", tgbotapi.InlineKeyboardMarkup{})
		case SettingsSelectionUOM:
			uom, parsed := parseInlineUOMName(message.Text)
			if !parsed {
				return ensurePanelText(api, chatID, &session, service, principalID, "Iltimos, UOM ni inline menyudan tanlang.", uomPickerKeyboard())
			}
			service.SetDefaultUOM(uom)
			session.SettingsSelect = SettingsSelectionNone
			service.sessions.Upsert(principalID, session)
			return ensurePanelText(api, chatID, &session, service, principalID, "Tanlandi: "+uom+"\n/wer /uom /logout", tgbotapi.InlineKeyboardMarkup{})
		}
	}

	if session.SupplierDispatchStep != SupplierDispatchStepNone {
		return handleSupplierDispatchText(ctx, api, service, message, principalID, chatID, session)
	}

	if session.WarehouseNoticeStep != WarehouseNoticeStepNone {
		return handleWarehouseNoticeText(ctx, api, service, message, principalID, chatID, session)
	}

	inLoginFlow := session.Step != LoginStepNone
	if inLoginFlow {
		deleteMessageBestEffort(api, chatID, message.MessageID)
		responseText := service.HandleText(ctx, principalID, message.Text)
		if strings.TrimSpace(responseText) == "" {
			return nil
		}

		if session.PromptMessageID > 0 {
			if err := editMessageText(api, chatID, session.PromptMessageID, responseText); err == nil {
				if strings.HasPrefix(responseText, "Kirish muvaffaqiyatli.") {
					if err := sendStartActionPrompt(api, service, chatID, principalID); err != nil {
						return err
					}
				}
				return nil
			}
			log.Printf("prompt edit failed for user %d, sending fallback message", principalID)
		}

		newPromptID, err := sendTextMessage(api, chatID, responseText)
		if err != nil {
			return fmt.Errorf("telegram send failed: %w", err)
		}
		if strings.HasPrefix(responseText, "Kirish muvaffaqiyatli.") {
			if err := sendStartActionPrompt(api, service, chatID, principalID); err != nil {
				return err
			}
		}

		if updated, exists := service.sessions.Get(principalID); exists && updated.Step != LoginStepNone {
			updated.PromptMessageID = newPromptID
			service.sessions.Upsert(principalID, updated)
		}
		return nil
	}

	if session.ActionStep == ActionStepNone {
		if session.AdminAuthed {
			deleteMessageBestEffort(api, chatID, message.MessageID)
			return nil
		}
		if session.SettingsAuthed {
			deleteMessageBestEffort(api, chatID, message.MessageID)
			return nil
		}
		if session.UserRole != UserRoleNone {
			deleteMessageBestEffort(api, chatID, message.MessageID)
			return nil
		}
		responseText := service.HandleText(ctx, principalID, message.Text)
		if strings.TrimSpace(responseText) == "" {
			return nil
		}
		if _, err := sendTextMessage(api, chatID, responseText); err != nil {
			return fmt.Errorf("telegram send failed: %w", err)
		}
		return nil
	}

	switch session.ActionStep {
	case ActionStepAwaitingItem:
		deleteMessageBestEffort(api, chatID, message.MessageID)
		itemCode, parsed := parseInlineItemCode(message.Text)
		if !parsed {
			if session.PromptMessageID > 0 {
				_ = editMessageText(api, chatID, session.PromptMessageID, "Iltimos, mahsulotni faqat 'Mahsulot' tugmasi orqali tanlang.")
			}
			return nil
		}
		session.SelectedItemCode = itemCode
		session.SelectedUOM = ""
		session.RequireUOMFirst = false
		session.ActionStep = ActionStepAwaitingQty
		service.sessions.Upsert(principalID, session)
		if session.PromptMessageID > 0 {
			_ = editMessageText(api, chatID, session.PromptMessageID, "Miqdor kiriting (faqat 0 dan katta son).")
		}
		return nil

	case ActionStepAwaitingUOM:
		deleteMessageBestEffort(api, chatID, message.MessageID)
		uom, parsed := parseInlineUOMName(message.Text)
		if !parsed {
			if session.PromptMessageID > 0 {
				_ = editMessageTextWithKeyboard(api, chatID, session.PromptMessageID, "Iltimos, UOM ni inline menyudan tanlang.", uomPickerKeyboard())
			}
			return nil
		}
		session.SelectedUOM = uom
		session.RequireUOMFirst = false
		session.ActionStep = ActionStepAwaitingQty
		service.sessions.Upsert(principalID, session)
		if session.PromptMessageID > 0 {
			_ = editMessageText(api, chatID, session.PromptMessageID, fmt.Sprintf("UOM tanlandi: %s\nMiqdor kiriting (faqat 0 dan katta son).", uom))
		}
		return nil

	case ActionStepAwaitingQty:
		deleteMessageBestEffort(api, chatID, message.MessageID)
		if session.RequireUOMFirst && strings.TrimSpace(session.SelectedUOM) == "" {
			session.ActionStep = ActionStepAwaitingUOM
			service.sessions.Upsert(principalID, session)
			if session.PromptMessageID > 0 {
				_ = editMessageTextWithKeyboard(api, chatID, session.PromptMessageID, "Avval UOM ni tanlang. Quyidagi 'UOM' tugmasini bosing.", uomPickerKeyboard())
			}
			return nil
		}
		qty, err := parsePositiveQuantity(message.Text)
		if err != nil {
			if session.PromptMessageID > 0 {
				_ = editMessageText(api, chatID, session.PromptMessageID, "Miqdor noto'g'ri. Iltimos, 0 dan katta son kiriting.")
			}
			return nil
		}

		if !service.EnsureCredentials(principalID) {
			if session.PromptMessageID > 0 {
				_ = editMessageText(api, chatID, session.PromptMessageID, "Sessiya topilmadi. Iltimos, qayta /login qiling.")
			}
			service.sessions.Clear(principalID)
			return nil
		}
		creds, _ := service.creds.Get(principalID)

		input, err := buildStockEntryInput(service, session, qty)
		if err != nil {
			if session.PromptMessageID > 0 {
				_ = editMessageText(api, chatID, session.PromptMessageID, err.Error())
			}
			return nil
		}

		result, err := service.erp.CreateAndSubmitStockEntry(ctx, creds.BaseURL, creds.APIKey, creds.APISecret, input)
		if err != nil {
			log.Printf(
				"stock entry failed: user=%d action=%s item=%s qty=%.4f uom=%s err=%v",
				principalID,
				session.ActionType,
				session.SelectedItemCode,
				qty,
				input.UOM,
				err,
			)
			if session.PromptMessageID > 0 {
				_ = editMessageText(api, chatID, session.PromptMessageID, userFacingStockError(err))
			}
			return nil
		}

		session.LastActionType = session.ActionType
		session.LastItemCode = session.SelectedItemCode
		session.LastUOM = input.UOM
		session.ActionStep = ActionStepNone
		session.ActionType = ""
		session.SelectedItemCode = ""
		session.SelectedUOM = ""
		session.RequireUOMFirst = false
		service.sessions.Upsert(principalID, session)

		success := fmt.Sprintf("Muvaffaqiyatli. Stock Entry yaratildi va submit qilindi: %s", result.Name)
		if session.PromptMessageID > 0 {
			_ = editMessageText(api, chatID, session.PromptMessageID, success)
		}
		if err := sendStartActionPrompt(api, service, chatID, principalID); err != nil {
			return err
		}
		return nil
	}

	return nil
}
