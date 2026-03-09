package bot

import (
	"fmt"

	"erpnext_stock_telegram/internal/erpnext"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

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

func interruptSessionMessages(api *tgbotapi.BotAPI, chatID int64, session LoginSession, command string) {
	deleteMessageBestEffort(api, chatID, session.WelcomeMessageID)
	deleteMessageBestEffort(api, chatID, session.PromptMessageID)
	deleteMessageBestEffort(api, chatID, session.AdminPanelID)
	deleteMessageBestEffort(api, chatID, session.SupplierNameMsgID)
	deleteMessageBestEffort(api, chatID, session.SupplierPhoneMsgID)
	deleteMessageBestEffort(api, chatID, session.ContactPhoneMsgID)
	deleteMessageBestEffort(api, chatID, session.ContactNameMsgID)
	deleteMessageBestEffort(api, chatID, session.SupplierAuthPromptMsgID)
	if !commandUsesSettingsContext(command) {
		deleteMessageBestEffort(api, chatID, session.SettingsPanelID)
	}
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

func supplierPickerKeyboard() tgbotapi.InlineKeyboardMarkup {
	query := "sup"
	row := tgbotapi.NewInlineKeyboardRow(
		tgbotapi.InlineKeyboardButton{Text: "Supplier", SwitchInlineQueryCurrentChat: &query},
	)
	return tgbotapi.NewInlineKeyboardMarkup(row)
}

func pendingReceiptPickerKeyboard() tgbotapi.InlineKeyboardMarkup {
	query := "notice"
	row := tgbotapi.NewInlineKeyboardRow(
		tgbotapi.InlineKeyboardButton{Text: "Draft", SwitchInlineQueryCurrentChat: &query},
	)
	return tgbotapi.NewInlineKeyboardMarkup(row)
}

func dispatchConfirmKeyboard() tgbotapi.InlineKeyboardMarkup {
	row := tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("Ha", callbackDispatchConfirm),
		tgbotapi.NewInlineKeyboardButtonData("Bekor qilish", callbackDispatchCancel),
	)
	return tgbotapi.NewInlineKeyboardMarkup(row)
}

func pendingReceiptKeyboard(items []erpnext.PurchaseReceiptDraft) tgbotapi.InlineKeyboardMarkup {
	rows := make([][]tgbotapi.InlineKeyboardButton, 0, len(items))
	for _, item := range items {
		text := fmt.Sprintf("%s | %s | %.2f %s", item.Supplier, item.ItemCode, item.Qty, item.UOM)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(text, callbackNoticeOpenPrefix+item.Name),
		))
	}
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
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

func ensureAdminPanelText(api *tgbotapi.BotAPI, chatID int64, session *LoginSession, service *Service, principalID int64, text string, markup tgbotapi.InlineKeyboardMarkup) error {
	if session.AdminPanelID > 0 {
		var err error
		if len(markup.InlineKeyboard) > 0 {
			err = editMessageTextWithKeyboard(api, chatID, session.AdminPanelID, text, markup)
		} else {
			err = editMessageText(api, chatID, session.AdminPanelID, text)
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
	session.AdminPanelID = msgID
	service.sessions.Upsert(principalID, *session)
	return nil
}

func adminWelcomeText() string {
	return "Admin panelga xush kelibsiz.\n\n" + adminPanelCommandsText()
}

func adminOnlyCommandText() string {
	return "Admin panelda faqat /admin, /supplier, /suplier_list, /adminka, /werka, /logout ishlaydi."
}

func adminPanelCommandsText() string {
	return "/admin - panelni qayta ochish\n/supplier - supplier qo'shish\n/suplier_list - supplier code ko'rish\n/adminka - adminka kontaktini saqlash\n/werka - omborchi kontaktini saqlash\n/logout - paneldan chiqish"
}

func isAdminCommand(command string) bool {
	switch command {
	case "admin", "supplier", "suplier_list", "supplier_list", "adminka", "werka", "logout":
		return true
	default:
		return false
	}
}

func clearSupplierMessages(api *tgbotapi.BotAPI, chatID int64, session LoginSession) {
	deleteMessageBestEffort(api, chatID, session.SupplierNameMsgID)
	deleteMessageBestEffort(api, chatID, session.SupplierPhoneMsgID)
	deleteMessageBestEffort(api, chatID, session.SupplierNameInputMsgID)
	deleteMessageBestEffort(api, chatID, session.SupplierPhoneInputMsgID)
}

func clearContactSetupMessages(api *tgbotapi.BotAPI, chatID int64, session LoginSession) {
	deleteMessageBestEffort(api, chatID, session.ContactPhoneMsgID)
	deleteMessageBestEffort(api, chatID, session.ContactNameMsgID)
	deleteMessageBestEffort(api, chatID, session.ContactPhoneInputMsgID)
	deleteMessageBestEffort(api, chatID, session.ContactNameInputMsgID)
}

func contactNamePromptText(kind ContactSetupKind) string {
	switch kind {
	case ContactSetupKindAdminka:
		return "Adminka ismini kiriting:"
	case ContactSetupKindWerka:
		return "Omborchi ismini kiriting:"
	default:
		return "Ismni kiriting:"
	}
}

func contactSuccessText(kind ContactSetupKind) string {
	switch kind {
	case ContactSetupKindAdminka:
		return "Adminka muvaffaqiyatli saqlandi."
	case ContactSetupKindWerka:
		return "Omborchi muvaffaqiyatli saqlandi."
	default:
		return "Ma'lumot muvaffaqiyatli saqlandi."
	}
}
