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
	callbackReceipt     = "action:type:receipt"
	callbackIssue       = "action:type:issue"
	inlineItemPrefix    = "item::"
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
					return err
				}
				continue
			}

			if update.InlineQuery != nil {
				if err := handleInlineQuery(ctx, api, service, update.InlineQuery); err != nil {
					return err
				}
				continue
			}

			if update.Message == nil {
				continue
			}

			if err := handleIncomingMessage(ctx, api, service, update.Message); err != nil {
				return err
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

	if message.IsCommand() {
		switch message.Command() {
		case "start":
			deleteMessageBestEffort(api, chatID, message.MessageID)
			resetSessionMessages(api, service, chatID, principalID)

			text := service.HandleStart(principalID)
			welcomeID, err := sendTextMessage(api, chatID, text)
			if err != nil {
				return fmt.Errorf("telegram send failed: %w", err)
			}

			service.sessions.Upsert(principalID, LoginSession{
				Step:             LoginStepNone,
				WelcomeMessageID: welcomeID,
			})
			return nil

		case "login":
			deleteMessageBestEffort(api, chatID, message.MessageID)
			resetSessionMessages(api, service, chatID, principalID)

			text := service.HandleLoginCommand(principalID)
			promptID, err := sendTextMessage(api, chatID, text)
			if err != nil {
				return fmt.Errorf("telegram send failed: %w", err)
			}

			session, _ := service.sessions.Get(principalID)
			session.WelcomeMessageID = 0
			session.PromptMessageID = promptID
			session.ActionStep = ActionStepNone
			session.ActionType = ""
			session.SelectedItemCode = ""
			service.sessions.Upsert(principalID, session)
			return nil

		default:
			deleteMessageBestEffort(api, chatID, message.MessageID)
			if _, err := sendTextMessage(api, chatID, "Noma'lum buyruq. Mavjud buyruqlar: /start, /login"); err != nil {
				return fmt.Errorf("telegram send failed: %w", err)
			}
			return nil
		}
	}

	session, ok := service.sessions.Get(principalID)
	inLoginFlow := ok && session.Step != LoginStepNone
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

	if !ok || session.ActionStep == ActionStepNone {
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
		itemCode, ok := parseInlineItemCode(message.Text)
		if !ok {
			if session.PromptMessageID > 0 {
				_ = editMessageText(api, chatID, session.PromptMessageID, "Iltimos, mahsulotni faqat 'Mahsulot' tugmasi orqali tanlang.")
			}
			return nil
		}
		session.SelectedItemCode = itemCode
		session.ActionStep = ActionStepAwaitingQty
		service.sessions.Upsert(principalID, session)
		if session.PromptMessageID > 0 {
			_ = editMessageText(api, chatID, session.PromptMessageID, "Miqdor kiriting (faqat 0 dan katta son).")
		}
		return nil

	case ActionStepAwaitingQty:
		deleteMessageBestEffort(api, chatID, message.MessageID)
		qty, err := parsePositiveQuantity(message.Text)
		if err != nil {
			if session.PromptMessageID > 0 {
				_ = editMessageText(api, chatID, session.PromptMessageID, "Miqdor noto'g'ri. Iltimos, 0 dan katta son kiriting.")
			}
			return nil
		}

		creds, ok := service.creds.Get(principalID)
		if !ok {
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
			if session.PromptMessageID > 0 {
				_ = editMessageText(api, chatID, session.PromptMessageID, "Stock Entry yaratishda xatolik: "+err.Error())
			}
			return nil
		}

		session.ActionStep = ActionStepNone
		session.ActionType = ""
		session.SelectedItemCode = ""
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

		clearRecentMessagesBestEffort(api, chatID, cb.Message.MessageID, 80)
		promptID, err := sendTextMessageWithKeyboard(api, chatID, "Harakatni tanlang:", actionTypeKeyboard())
		if err != nil {
			return fmt.Errorf("telegram send failed: %w", err)
		}

		session.Step = LoginStepNone
		session.PromptMessageID = promptID
		session.WelcomeMessageID = 0
		session.ActionStep = ActionStepAwaitingType
		session.ActionType = ""
		session.SelectedItemCode = ""
		service.sessions.Upsert(principalID, session)
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
	if !ok || session.ActionStep != ActionStepAwaitingItem {
		cfg := tgbotapi.InlineConfig{InlineQueryID: q.ID, IsPersonal: true, CacheTime: 0, Results: []interface{}{}}
		_, _ = api.Request(cfg)
		return nil
	}

	creds, ok := service.creds.Get(principalID)
	if !ok {
		cfg := tgbotapi.InlineConfig{InlineQueryID: q.ID, IsPersonal: true, CacheTime: 0, Results: []interface{}{}}
		_, _ = api.Request(cfg)
		return nil
	}

	query := strings.TrimSpace(strings.TrimPrefix(q.Query, "item"))
	items, err := service.erp.SearchItems(ctx, creds.BaseURL, creds.APIKey, creds.APISecret, query, 20)
	if err != nil {
		return fmt.Errorf("item search failed: %w", err)
	}

	results := make([]interface{}, 0, len(items))
	for _, item := range items {
		messageText := inlineItemPrefix + item.Code
		title := item.Code
		if item.Name != "" && item.Name != item.Code {
			title = fmt.Sprintf("%s - %s", item.Code, item.Name)
		}
		article := tgbotapi.NewInlineQueryResultArticle(item.Code, title, messageText)
		if item.UOM != "" {
			article.Description = "UOM: " + item.UOM
		}
		results = append(results, article)
	}

	cfg := tgbotapi.InlineConfig{
		InlineQueryID: q.ID,
		IsPersonal:    true,
		CacheTime:     0,
		Results:       results,
	}
	if _, err := api.Request(cfg); err != nil {
		return fmt.Errorf("inline answer failed: %w", err)
	}
	return nil
}

func buildStockEntryInput(service *Service, session LoginSession, qty float64) (erpnext.CreateStockEntryInput, error) {
	input := erpnext.CreateStockEntryInput{
		ItemCode: session.SelectedItemCode,
		Qty:      qty,
		UOM:      "Kg",
	}

	switch session.ActionType {
	case ActionTypeReceipt:
		if strings.TrimSpace(service.defaultTargetWarehouse) == "" {
			return erpnext.CreateStockEntryInput{}, fmt.Errorf("ERP_DEFAULT_TARGET_WAREHOUSE sozlanmagan")
		}
		input.EntryType = "Material Receipt"
		input.TargetWarehouse = service.defaultTargetWarehouse
	case ActionTypeIssue:
		if strings.TrimSpace(service.defaultSourceWarehouse) == "" {
			return erpnext.CreateStockEntryInput{}, fmt.Errorf("ERP_DEFAULT_SOURCE_WAREHOUSE sozlanmagan")
		}
		input.EntryType = "Material Issue"
		input.SourceWarehouse = service.defaultSourceWarehouse
	default:
		return erpnext.CreateStockEntryInput{}, fmt.Errorf("harakat turi tanlanmagan")
	}

	return input, nil
}

func sendStartActionPrompt(api *tgbotapi.BotAPI, service *Service, chatID, principalID int64) error {
	msgID, err := sendTextMessageWithKeyboard(api, chatID, "Harakatni boshlaymizmi?", startActionKeyboard())
	if err != nil {
		return fmt.Errorf("telegram send failed: %w", err)
	}

	session, _ := service.sessions.Get(principalID)
	session.Step = LoginStepNone
	session.WelcomeMessageID = msgID
	session.ActionStep = ActionStepNone
	session.ActionType = ""
	session.SelectedItemCode = ""
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

func startActionKeyboard() tgbotapi.InlineKeyboardMarkup {
	row := tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("Harakat", callbackStartAction),
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
	msg.ReplyMarkup = markup
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
		log.Printf("failed to delete message %d in chat %d: %v", messageID, chatID, err)
	}
}

func parseInlineItemCode(text string) (string, bool) {
	trimmed := strings.TrimSpace(text)
	if !strings.HasPrefix(trimmed, inlineItemPrefix) {
		return "", false
	}
	code := strings.TrimSpace(strings.TrimPrefix(trimmed, inlineItemPrefix))
	if code == "" {
		return "", false
	}
	return code, true
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
