package bot

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"

	"erpnext_stock_telegram/internal/suplier"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

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

func sendHTMLMessage(api *tgbotapi.BotAPI, chatID int64, text string) (int, error) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeHTML
	sent, err := api.Send(msg)
	if err != nil {
		return 0, err
	}
	return sent.MessageID, nil
}

func sendTextMessageWithReplyMarkup(api *tgbotapi.BotAPI, chatID int64, text string, markup interface{}) (int, error) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = markup
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

func contactRequestKeyboard() tgbotapi.ReplyKeyboardMarkup {
	button := tgbotapi.NewKeyboardButtonContact("Telefon raqamni yuborish")
	row := tgbotapi.NewKeyboardButtonRow(button)
	keyboard := tgbotapi.NewReplyKeyboard(row)
	keyboard.ResizeKeyboard = true
	keyboard.OneTimeKeyboard = true
	return keyboard
}

func removeKeyboard() tgbotapi.ReplyKeyboardRemove {
	return tgbotapi.NewRemoveKeyboard(true)
}

func authenticatedStartText(session LoginSession) string {
	switch session.UserRole {
	case UserRoleAdmin:
		return "Siz admin panelga kirdingiz.\n\n" + adminPanelCommandsText()
	case UserRoleWerka:
		return "Siz omborchi sifatida tanildingiz.\n/bildirishnoma - pending qabul ro'yxati"
	case UserRoleSupplier:
		if strings.TrimSpace(session.UserName) != "" {
			return "Siz supplier sifatida tanildingiz: " + session.UserName + "\n/dispatch - jo'natilgan mahsulotni bildirish"
		}
		return "Siz supplier sifatida tanildingiz.\n/dispatch - jo'natilgan mahsulotni bildirish"
	default:
		return "Telefon raqamingizni yuboring."
	}
}

func parseInlineItemCode(text string) (string, bool) {
	return parseInlineValue(text, inlineItemPrefix)
}

func parseInlineWarehouseName(text string) (string, bool) {
	return parseInlineValue(text, inlineWarehousePrefix)
}

func parseInlineSupplierValue(text string) (string, bool) {
	return parseInlineValue(text, inlineSupplierPrefix)
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

func userFacingSupplierAuthError(err error) string {
	switch {
	case errors.Is(err, suplier.ErrSupplierAuthInvalidPassword):
		return "Parol noto'g'ri. Qayta kiriting."
	case errors.Is(err, suplier.ErrSupplierAuthLocked):
		return "Juda ko'p noto'g'ri urinish bo'ldi. Birozdan keyin qayta urinib ko'ring."
	case errors.Is(err, suplier.ErrSupplierAuthAlreadyRegistered):
		return "Bu supplier allaqachon ro'yxatdan o'tgan. Parolni kiriting."
	case errors.Is(err, suplier.ErrSupplierAuthNotFound):
		return "Supplier auth topilmadi. /start ni qayta yuboring."
	default:
		return "Supplier sifatida kirish amalga oshmadi. Qayta urinib ko'ring."
	}
}

func parseTelegramReceiptPhone(marker string) string {
	trimmed := strings.TrimSpace(marker)
	if !strings.HasPrefix(trimmed, "TG:") {
		return ""
	}
	payload := strings.TrimPrefix(trimmed, "TG:")
	parts := strings.SplitN(payload, ":", 2)
	if len(parts) == 0 {
		return ""
	}
	return strings.TrimSpace(parts[0])
}
