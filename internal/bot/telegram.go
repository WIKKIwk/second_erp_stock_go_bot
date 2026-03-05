package bot

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"erpnext_stock_telegram/internal/erpnext"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	callbackStartAction = "action:start"
	callbackAgainAction = "action:again"
	callbackReceipt     = "action:type:receipt"
	callbackIssue       = "action:type:issue"

	inlineItemPrefix      = "item::"
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
		return handleCommand(ctx, api, service, message, principalID, chatID, session)
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
		if session.SettingsAuthed {
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

		creds, found := service.creds.Get(principalID)
		if !found {
			if session.PromptMessageID > 0 {
				_ = editMessageText(api, chatID, session.PromptMessageID, "Sessiya topilmadi. Iltimos, qayta /login qiling.")
			}
			service.sessions.Clear(principalID)
			return nil
		}

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

func handleCommand(ctx context.Context, api *tgbotapi.BotAPI, service *Service, message *tgbotapi.Message, principalID, chatID int64, session LoginSession) error {
	_ = ctx
	switch message.Command() {
	case "start":
		deleteMessageBestEffort(api, chatID, message.MessageID)
		resetSessionMessages(api, service, chatID, principalID)

		text := service.HandleStart(principalID)
		welcomeID, err := sendTextMessage(api, chatID, text)
		if err != nil {
			return fmt.Errorf("telegram send failed: %w", err)
		}

		service.sessions.Upsert(principalID, LoginSession{Step: LoginStepNone, WelcomeMessageID: welcomeID})
		return nil

	case "login":
		deleteMessageBestEffort(api, chatID, message.MessageID)
		resetSessionMessages(api, service, chatID, principalID)

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

	case "stock":
		deleteMessageBestEffort(api, chatID, message.MessageID)
		if _, ok := service.creds.Get(principalID); !ok {
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
		clearRecentMessagesAsync(api, chatID, message.MessageID, 40)
		session.SettingsStep = SettingsStepNone
		session.SettingsAuthed = false
		session.SettingsSelect = SettingsSelectionNone
		session.SettingsPanelID = 0
		service.sessions.Upsert(principalID, session)
		if _, err := sendTextMessage(api, chatID, "Siz settings dan chiqdingiz."); err != nil {
			return fmt.Errorf("telegram send failed: %w", err)
		}
		return nil

	default:
		deleteMessageBestEffort(api, chatID, message.MessageID)
		if _, err := sendTextMessage(api, chatID, "Noma'lum buyruq. Mavjud buyruqlar: /start, /login, /stock, /settings"); err != nil {
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
		if _, ok := service.creds.Get(principalID); !ok {
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
		if _, ok := service.creds.Get(principalID); !ok {
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
		if err := editMessageTextWithKeyboard(api, chatID, cb.Message.MessageID, text, tgbotapi.NewInlineKeyboardMarkup()); err != nil {
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

	creds, ok := service.creds.Get(principalID)
	if !ok {
		return answerEmptyInline(api, q.ID)
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

func sendStartActionPrompt(api *tgbotapi.BotAPI, service *Service, chatID, principalID int64) error {
	msgID, err := sendTextMessageWithKeyboard(api, chatID, "Harakatni boshlaymizmi yoki yana?", startActionKeyboard())
	if err != nil {
		return fmt.Errorf("telegram send failed: %w", err)
	}

	session, _ := service.sessions.Get(principalID)
	session.Step = LoginStepNone
	session.WelcomeMessageID = msgID
	session.ActionStep = ActionStepNone
	session.ActionType = ""
	session.SelectedItemCode = ""
	session.SelectedUOM = ""
	session.RequireUOMFirst = false
	service.sessions.Upsert(principalID, session)
	return nil
}

func resetSessionMessages(api *tgbotapi.BotAPI, service *Service, chatID, principalID int64) {
	session, ok := service.sessions.Get(principalID)
	if !ok {
		return
	}

	deleteMessageBestEffort(api, chatID, session.WelcomeMessageID)
	deleteMessageBestEffort(api, chatID, session.PromptMessageID)
	deleteMessageBestEffort(api, chatID, session.SettingsPanelID)
	service.sessions.Clear(principalID)
}

func clearRecentMessagesBestEffort(api *tgbotapi.BotAPI, chatID int64, fromMessageID int, window int) {
	if fromMessageID <= 0 {
		return
	}
	if window <= 0 {
		window = 50
	}
	minID := fromMessageID - window
	if minID < 1 {
		minID = 1
	}
	for id := fromMessageID; id >= minID; id-- {
		deleteMessageBestEffort(api, chatID, id)
	}
}

func clearRecentMessagesAsync(api *tgbotapi.BotAPI, chatID int64, fromMessageID int, window int) {
	go clearRecentMessagesBestEffort(api, chatID, fromMessageID, window)
}

func startActionKeyboard() tgbotapi.InlineKeyboardMarkup {
	row := tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("Harakat", callbackStartAction),
		tgbotapi.NewInlineKeyboardButtonData("Yana", callbackAgainAction),
	)
	return tgbotapi.NewInlineKeyboardMarkup(row)
}

func actionTypeKeyboard() tgbotapi.InlineKeyboardMarkup {
	row := tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("Mahsulot kirdi", callbackReceipt),
		tgbotapi.NewInlineKeyboardButtonData("Mahsulot chiqdi", callbackIssue),
	)
	return tgbotapi.NewInlineKeyboardMarkup(row)
}

func itemPickerKeyboard() tgbotapi.InlineKeyboardMarkup {
	query := "item"
	row := tgbotapi.NewInlineKeyboardRow(
		tgbotapi.InlineKeyboardButton{Text: "Mahsulot", SwitchInlineQueryCurrentChat: &query},
	)
	return tgbotapi.NewInlineKeyboardMarkup(row)
}

func warehousePickerKeyboard() tgbotapi.InlineKeyboardMarkup {
	query := "wer"
	row := tgbotapi.NewInlineKeyboardRow(
		tgbotapi.InlineKeyboardButton{Text: "Ombor", SwitchInlineQueryCurrentChat: &query},
	)
	return tgbotapi.NewInlineKeyboardMarkup(row)
}

func uomPickerKeyboard() tgbotapi.InlineKeyboardMarkup {
	query := "uom"
	row := tgbotapi.NewInlineKeyboardRow(
		tgbotapi.InlineKeyboardButton{Text: "UOM", SwitchInlineQueryCurrentChat: &query},
	)
	return tgbotapi.NewInlineKeyboardMarkup(row)
}

func ensurePanelText(api *tgbotapi.BotAPI, chatID int64, session *LoginSession, service *Service, principalID int64, text string, markup tgbotapi.InlineKeyboardMarkup) error {
	if session.SettingsPanelID > 0 {
		var err error
		if len(markup.InlineKeyboard) > 0 {
			err = editMessageTextWithKeyboard(api, chatID, session.SettingsPanelID, text, markup)
		} else {
			err = editMessageText(api, chatID, session.SettingsPanelID, text)
		}
		if err == nil {
			service.sessions.Upsert(principalID, *session)
			return nil
		}
	}

	msgID, err := sendTextMessageWithKeyboard(api, chatID, text, markup)
	if err != nil {
		return err
	}
	session.SettingsPanelID = msgID
	service.sessions.Upsert(principalID, *session)
	return nil
}

func answerInline(api *tgbotapi.BotAPI, inlineQueryID string, results []interface{}) error {
	cfg := tgbotapi.InlineConfig{InlineQueryID: inlineQueryID, IsPersonal: true, CacheTime: 0, Results: results}
	_, err := api.Request(cfg)
	return err
}

func answerEmptyInline(api *tgbotapi.BotAPI, inlineQueryID string) error {
	return answerInline(api, inlineQueryID, []interface{}{})
}

func sendTextMessage(api *tgbotapi.BotAPI, chatID int64, text string) (int, error) {
	msg := tgbotapi.NewMessage(chatID, text)
	sent, err := api.Send(msg)
	if err != nil {
		return 0, err
	}
	return sent.MessageID, nil
}

func sendTextMessageWithKeyboard(api *tgbotapi.BotAPI, chatID int64, text string, markup tgbotapi.InlineKeyboardMarkup) (int, error) {
	msg := tgbotapi.NewMessage(chatID, text)
	if len(markup.InlineKeyboard) > 0 {
		msg.ReplyMarkup = markup
	}
	sent, err := api.Send(msg)
	if err != nil {
		return 0, err
	}
	return sent.MessageID, nil
}

func editMessageText(api *tgbotapi.BotAPI, chatID int64, messageID int, text string) error {
	edit := tgbotapi.NewEditMessageText(chatID, messageID, text)
	_, err := api.Send(edit)
	return err
}

func editMessageTextWithKeyboard(api *tgbotapi.BotAPI, chatID int64, messageID int, text string, markup tgbotapi.InlineKeyboardMarkup) error {
	edit := tgbotapi.NewEditMessageTextAndMarkup(chatID, messageID, text, markup)
	_, err := api.Send(edit)
	return err
}

func deleteMessageBestEffort(api *tgbotapi.BotAPI, chatID int64, messageID int) {
	if messageID <= 0 {
		return
	}
	del := tgbotapi.NewDeleteMessage(chatID, messageID)
	if _, err := api.Request(del); err != nil {
		if strings.Contains(err.Error(), "message to delete not found") {
			return
		}
		log.Printf("failed to delete message %d in chat %d: %v", messageID, chatID, err)
	}
}

func parseInlineItemCode(text string) (string, bool) {
	return parseInlineValue(text, inlineItemPrefix)
}

func parseInlineWarehouseName(text string) (string, bool) {
	return parseInlineValue(text, inlineWarehousePrefix)
}

func parseInlineUOMName(text string) (string, bool) {
	return parseInlineValue(text, inlineUOMPrefix)
}

func parseInlineValue(text, prefix string) (string, bool) {
	trimmed := strings.TrimSpace(text)
	if !strings.HasPrefix(trimmed, prefix) {
		return "", false
	}
	value := strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
	if value == "" {
		return "", false
	}
	return value, true
}

func normalizeInlineQuery(query, trigger string) string {
	trimmed := strings.TrimSpace(query)
	trimmed = strings.TrimPrefix(trimmed, trigger)
	return strings.TrimSpace(trimmed)
}

func parsePositiveQuantity(text string) (float64, error) {
	value := strings.TrimSpace(strings.ReplaceAll(text, ",", "."))
	qty, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, err
	}
	if qty <= 0 {
		return 0, fmt.Errorf("quantity must be greater than 0")
	}
	return qty, nil
}

func userFacingStockError(err error) string {
	raw := strings.ToLower(err.Error())

	switch {
	case strings.Contains(raw, "linkvalidationerror"),
		strings.Contains(raw, "default target warehouse"),
		strings.Contains(raw, "default source warehouse"),
		strings.Contains(raw, "target warehouse"),
		strings.Contains(raw, "source warehouse"),
		strings.Contains(raw, "could not find"):
		return "Settingsni to'g'rilang."

	case strings.Contains(raw, "timestampmismatcherror"):
		return "Server band bo'ldi, qayta urinib ko'ring."

	default:
		return "Stock Entry yaratilmadi. Settingsni tekshirib qayta urinib ko'ring."
	}
}
