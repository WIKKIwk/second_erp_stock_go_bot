package bot

import (
	"context"
	"fmt"
	"log"
	"strings"

	"erpnext_stock_telegram/internal/erpnext"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func beginSupplierDispatch(api *tgbotapi.BotAPI, service *Service, chatID, principalID int64, session LoginSession) error {
	if session.UserRole != UserRoleSupplier && session.UserRole != UserRoleAdmin && !session.AdminAuthed {
		if _, err := sendTextMessage(api, chatID, "Bu buyruq faqat supplier uchun."); err != nil {
			return fmt.Errorf("telegram send failed: %w", err)
		}
		return nil
	}
	if !service.EnsureCredentials(principalID) {
		if _, err := sendTextMessage(api, chatID, "Iltimos, avval /login qiling."); err != nil {
			return fmt.Errorf("telegram send failed: %w", err)
		}
		return nil
	}

	promptID, err := sendTextMessageWithKeyboard(api, chatID, "Mahsulot tanlang. Quyidagi 'Mahsulot' tugmasini bosing.", itemPickerKeyboard())
	if err != nil {
		return fmt.Errorf("telegram send failed: %w", err)
	}

	clearDispatchState(&session)
	session.PromptMessageID = promptID
	session.SupplierDispatchStep = SupplierDispatchStepAwaitingItem
	service.sessions.Upsert(principalID, session)
	return nil
}

func openPendingReceipts(api *tgbotapi.BotAPI, service *Service, chatID, principalID int64, session LoginSession) error {
	if session.UserRole != UserRoleWerka && session.UserRole != UserRoleAdmin && !session.AdminAuthed {
		if _, err := sendTextMessage(api, chatID, "Bu buyruq faqat omborchi uchun."); err != nil {
			return fmt.Errorf("telegram send failed: %w", err)
		}
		return nil
	}
	if !service.EnsureCredentials(principalID) {
		if _, err := sendTextMessage(api, chatID, "Iltimos, avval /login qiling."); err != nil {
			return fmt.Errorf("telegram send failed: %w", err)
		}
		return nil
	}

	creds, _ := service.creds.Get(principalID)
	items, err := service.erp.ListPendingPurchaseReceipts(context.Background(), creds.BaseURL, creds.APIKey, creds.APISecret, 10)
	if err != nil {
		if _, sendErr := sendTextMessage(api, chatID, "Pending bildirishnomalarni olishda xatolik bo'ldi."); sendErr != nil {
			return fmt.Errorf("telegram send failed: %w", sendErr)
		}
		return nil
	}
	if len(items) == 0 {
		if _, err := sendTextMessage(api, chatID, "Pending bildirishnomalar yo'q."); err != nil {
			return fmt.Errorf("telegram send failed: %w", err)
		}
		return nil
	}

	clearNoticeState(&session)
	session.WarehouseNoticeListActive = true
	service.sessions.Upsert(principalID, session)

	text := fmt.Sprintf("Pending qabul qilish ro'yxati: %d ta draft bor.\nQuyidagi 'Draft' tugmasini bosing.", len(items))
	if _, err := sendTextMessageWithKeyboard(api, chatID, text, pendingReceiptPickerKeyboard()); err != nil {
		return fmt.Errorf("telegram send failed: %w", err)
	}
	return nil
}

func handleSupplierDispatchText(ctx context.Context, api *tgbotapi.BotAPI, service *Service, message *tgbotapi.Message, principalID, chatID int64, session LoginSession) error {
	switch session.SupplierDispatchStep {
	case SupplierDispatchStepAwaitingItem:
		deleteMessageBestEffort(api, chatID, message.MessageID)
		itemCode, parsed := parseInlineItemCode(message.Text)
		if !parsed {
			if session.PromptMessageID > 0 {
				_ = editMessageText(api, chatID, session.PromptMessageID, "Iltimos, mahsulotni faqat 'Mahsulot' tugmasi orqali tanlang.")
			}
			return nil
		}

		dispatchItem := erpnext.Item{Code: itemCode, Name: itemCode}
		if service.EnsureCredentials(principalID) {
			creds, _ := service.creds.Get(principalID)
			items, err := service.erp.SearchSupplierItems(ctx, creds.BaseURL, creds.APIKey, creds.APISecret, session.UserName, itemCode, 10)
			if err == nil {
				for _, item := range items {
					if strings.EqualFold(strings.TrimSpace(item.Code), strings.TrimSpace(itemCode)) {
						dispatchItem = item
						break
					}
				}
			}
		}

		session.DispatchItemCode = itemCode
		session.DispatchItemName = dispatchItem.Name
		session.DispatchUOM = dispatchItem.UOM
		session.SupplierDispatchStep = SupplierDispatchStepAwaitingQty
		service.sessions.Upsert(principalID, session)
		if session.PromptMessageID > 0 {
			_ = editMessageText(api, chatID, session.PromptMessageID, fmt.Sprintf("Mahsulot tanlandi: %s\nMiqdor kiriting.", itemCode))
		}
		return nil

	case SupplierDispatchStepAwaitingQty:
		deleteMessageBestEffort(api, chatID, message.MessageID)
		qty, err := parsePositiveQuantity(message.Text)
		if err != nil {
			if session.PromptMessageID > 0 {
				_ = editMessageText(api, chatID, session.PromptMessageID, "Miqdor noto'g'ri. Iltimos, 0 dan katta son kiriting.")
			}
			return nil
		}

		session.DispatchQty = qty
		session.SupplierDispatchStep = SupplierDispatchStepAwaitingConfirm
		service.sessions.Upsert(principalID, session)

		uom := strings.TrimSpace(session.DispatchUOM)
		if uom == "" {
			uom = "Nos"
		}
		text := fmt.Sprintf(
			"Jo'natishni tasdiqlaysizmi?\nSupplier: %s\nMahsulot: %s\nMiqdor: %.2f %s",
			session.UserName,
			session.DispatchItemCode,
			qty,
			uom,
		)
		if session.PromptMessageID > 0 {
			_ = editMessageTextWithKeyboard(api, chatID, session.PromptMessageID, text, dispatchConfirmKeyboard())
		}
		return nil
	}

	return nil
}

func handleWarehouseNoticeText(ctx context.Context, api *tgbotapi.BotAPI, service *Service, message *tgbotapi.Message, principalID, chatID int64, session LoginSession) error {
	if session.WarehouseNoticeStep != WarehouseNoticeStepAwaitingAcceptedQty {
		return nil
	}

	deleteMessageBestEffort(api, chatID, message.MessageID)
	qty, err := parsePositiveQuantity(message.Text)
	if err != nil {
		if session.PromptMessageID > 0 {
			_ = editMessageText(api, chatID, session.PromptMessageID, "Miqdor noto'g'ri. Iltimos, 0 dan katta son kiriting.")
		}
		return nil
	}
	if qty > session.NoticeSentQty {
		if session.PromptMessageID > 0 {
			_ = editMessageText(api, chatID, session.PromptMessageID, fmt.Sprintf("Qabul qilingan miqdor %.2f %s dan oshmasligi kerak.", session.NoticeSentQty, session.NoticeUOM))
		}
		return nil
	}
	if !service.EnsureCredentials(principalID) {
		if session.PromptMessageID > 0 {
			_ = editMessageText(api, chatID, session.PromptMessageID, "Sessiya topilmadi. Iltimos, qayta /login qiling.")
		}
		return nil
	}

	creds, _ := service.creds.Get(principalID)
	result, err := service.erp.ConfirmAndSubmitPurchaseReceipt(ctx, creds.BaseURL, creds.APIKey, creds.APISecret, session.NoticeReceiptName, qty)
	if err != nil {
		log.Printf("purchase receipt submit failed: receipt=%s qty=%.4f err=%v", session.NoticeReceiptName, qty, err)
		if session.PromptMessageID > 0 {
			_ = editMessageText(api, chatID, session.PromptMessageID, "Purchase Receipt submit bo'lmadi. Qayta urinib ko'ring.")
		}
		return nil
	}

	if session.PromptMessageID > 0 {
		_ = editMessageText(api, chatID, session.PromptMessageID, fmt.Sprintf("Qabul qilindi va submit bo'ldi: %s\nMahsulot: %s\nQabul qilingan: %.2f %s", result.Name, result.ItemCode, result.AcceptedQty, result.UOM))
	}

	notifySupplierAboutReceipt(ctx, api, service, result)
	clearNoticeState(&session)
	service.sessions.Upsert(principalID, session)
	return nil
}

func handleDispatchConfirmCallback(ctx context.Context, api *tgbotapi.BotAPI, service *Service, chatID, principalID int64, session LoginSession, messageID int) error {
	if !service.EnsureCredentials(principalID) {
		if _, err := sendTextMessage(api, chatID, "Iltimos, avval /login qiling."); err != nil {
			return fmt.Errorf("telegram send failed: %w", err)
		}
		return nil
	}
	targetWarehouse, _, _ := service.Defaults()
	if strings.TrimSpace(targetWarehouse) == "" {
		if err := editMessageText(api, chatID, messageID, "Default ombor sozlanmagan. /settings orqali omborni kiriting."); err != nil {
			return fmt.Errorf("telegram edit failed: %w", err)
		}
		return nil
	}

	creds, _ := service.creds.Get(principalID)
	draft, err := service.erp.CreateDraftPurchaseReceipt(ctx, creds.BaseURL, creds.APIKey, creds.APISecret, erpnext.CreatePurchaseReceiptInput{
		Supplier:      session.UserName,
		SupplierPhone: session.UserPhone,
		ItemCode:      session.DispatchItemCode,
		Qty:           session.DispatchQty,
		UOM:           session.DispatchUOM,
		Warehouse:     targetWarehouse,
	})
	if err != nil {
		if err := editMessageText(api, chatID, messageID, "Draft Purchase Receipt yaratilmadi. Qayta urinib ko'ring."); err != nil {
			return fmt.Errorf("telegram edit failed: %w", err)
		}
		return nil
	}

	clearDispatchState(&session)
	service.sessions.Upsert(principalID, session)

	if err := editMessageText(api, chatID, messageID, fmt.Sprintf("Jo'natish saqlandi.\nDraft Purchase Receipt: %s", draft.Name)); err != nil {
		return fmt.Errorf("telegram edit failed: %w", err)
	}

	notifyWerkaAboutDraft(api, service, draft)
	return nil
}

func handleDispatchCancelCallback(api *tgbotapi.BotAPI, service *Service, chatID, principalID int64, session LoginSession, messageID int) error {
	clearDispatchState(&session)
	service.sessions.Upsert(principalID, session)
	if err := editMessageText(api, chatID, messageID, "Jo'natish bekor qilindi."); err != nil {
		return fmt.Errorf("telegram edit failed: %w", err)
	}
	return nil
}

func handleNoticeOpenCallback(ctx context.Context, api *tgbotapi.BotAPI, service *Service, chatID, principalID int64, session LoginSession, receiptName string, messageID int) error {
	if !service.EnsureCredentials(principalID) {
		if _, err := sendTextMessage(api, chatID, "Iltimos, avval /login qiling."); err != nil {
			return fmt.Errorf("telegram send failed: %w", err)
		}
		return nil
	}

	creds, _ := service.creds.Get(principalID)
	draft, err := service.erp.GetPurchaseReceipt(ctx, creds.BaseURL, creds.APIKey, creds.APISecret, receiptName)
	if err != nil {
		if err := editMessageText(api, chatID, messageID, "Purchase Receipt topilmadi yoki allaqachon submit bo'lgan."); err != nil {
			return fmt.Errorf("telegram edit failed: %w", err)
		}
		return nil
	}

	clearNoticeState(&session)
	session.PromptMessageID = messageID
	session.WarehouseNoticeListActive = false
	session.WarehouseNoticeStep = WarehouseNoticeStepAwaitingAcceptedQty
	session.NoticeReceiptName = draft.Name
	session.NoticeSupplierPhone = parseTelegramReceiptPhone(draft.SupplierDeliveryNote)
	session.NoticeSupplierName = draft.Supplier
	session.NoticeItemCode = draft.ItemCode
	session.NoticeItemName = draft.ItemName
	session.NoticeUOM = draft.UOM
	session.NoticeSentQty = draft.Qty
	service.sessions.Upsert(principalID, session)

	text := fmt.Sprintf(
		"Qabul qilish:\nSupplier: %s\nMahsulot: %s\nJo'natilgan: %.2f %s\nQabul qilingan miqdorni kiriting.",
		draft.Supplier,
		draft.ItemCode,
		draft.Qty,
		draft.UOM,
	)
	if err := editMessageText(api, chatID, messageID, text); err != nil {
		return fmt.Errorf("telegram edit failed: %w", err)
	}
	return nil
}

func notifyWerkaAboutDraft(api *tgbotapi.BotAPI, service *Service, draft erpnext.PurchaseReceiptDraft) {
	chatID := service.WerkaTelegramID()
	if chatID == 0 {
		return
	}

	text := fmt.Sprintf(
		"Yangi bildirishnoma.\nSupplier: %s\nMahsulot: %s\nMiqdor: %.2f %s\n/not",
		draft.Supplier,
		draft.ItemCode,
		draft.Qty,
		draft.UOM,
	)
	if _, err := sendTextMessage(api, chatID, text); err != nil {
		log.Printf("failed to notify werka about draft %s: %v", draft.Name, err)
	}
}

func notifySupplierAboutReceipt(ctx context.Context, api *tgbotapi.BotAPI, service *Service, result erpnext.PurchaseReceiptSubmissionResult) {
	phone := parseTelegramReceiptPhone(result.SupplierDeliveryNote)
	if phone == "" {
		return
	}

	chatID, found := service.FindSupplierChatIDByPhone(phone)
	if !found || chatID == 0 {
		return
	}

	text := fmt.Sprintf(
		"Omborchi mahsulotni qabul qildi.\nHujjat: %s\nMahsulot: %s\nJo'natilgan: %.2f %s\nQabul qilingan: %.2f %s",
		result.Name,
		result.ItemCode,
		result.SentQty,
		result.UOM,
		result.AcceptedQty,
		result.UOM,
	)
	if _, err := sendTextMessage(api, chatID, text); err != nil {
		log.Printf("failed to notify supplier about receipt %s: %v", result.Name, err)
	}
}

func clearDispatchState(session *LoginSession) {
	session.SupplierDispatchStep = SupplierDispatchStepNone
	session.DispatchItemCode = ""
	session.DispatchItemName = ""
	session.DispatchUOM = ""
	session.DispatchQty = 0
}

func clearNoticeState(session *LoginSession) {
	session.WarehouseNoticeStep = WarehouseNoticeStepNone
	session.WarehouseNoticeListActive = false
	session.NoticeReceiptName = ""
	session.NoticeSupplierPhone = ""
	session.NoticeSupplierName = ""
	session.NoticeItemCode = ""
	session.NoticeItemName = ""
	session.NoticeUOM = ""
	session.NoticeSentQty = 0
}
