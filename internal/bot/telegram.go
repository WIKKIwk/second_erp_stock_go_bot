package bot

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
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
	responseText := service.HandleText(ctx, principalID, message.Text)

	if inLoginFlow {
		deleteMessageBestEffort(api, chatID, message.MessageID)
		if strings.TrimSpace(responseText) == "" {
			return nil
		}

		if session.PromptMessageID > 0 {
			if err := editMessageText(api, chatID, session.PromptMessageID, responseText); err == nil {
				return nil
			}
			log.Printf("prompt edit failed for user %d, sending fallback message", principalID)
		}

		newPromptID, err := sendTextMessage(api, chatID, responseText)
		if err != nil {
			return fmt.Errorf("telegram send failed: %w", err)
		}

		if updated, exists := service.sessions.Get(principalID); exists && updated.Step != LoginStepNone {
			updated.PromptMessageID = newPromptID
			service.sessions.Upsert(principalID, updated)
		}
		return nil
	}

	if strings.TrimSpace(responseText) == "" {
		return nil
	}
	if _, err := sendTextMessage(api, chatID, responseText); err != nil {
		return fmt.Errorf("telegram send failed: %w", err)
	}
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

func sendTextMessage(api *tgbotapi.BotAPI, chatID int64, text string) (int, error) {
	msg := tgbotapi.NewMessage(chatID, text)
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

func deleteMessageBestEffort(api *tgbotapi.BotAPI, chatID int64, messageID int) {
	if messageID <= 0 {
		return
	}
	del := tgbotapi.NewDeleteMessage(chatID, messageID)
	if _, err := api.Request(del); err != nil {
		log.Printf("failed to delete message %d in chat %d: %v", messageID, chatID, err)
	}
}
