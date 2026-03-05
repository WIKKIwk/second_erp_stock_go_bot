package bot

import (
	"context"
	"fmt"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func RunTelegramLoop(ctx context.Context, token string, service *Service) error {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return fmt.Errorf("telegram bot init failed: %w", err)
	}

	updates := api.GetUpdatesChan(tgbotapi.NewUpdate(0))

	for {
		select {
		case <-ctx.Done():
			return nil
		case update := <-updates:
			if update.Message == nil {
				continue
			}

			chatID := update.Message.Chat.ID
			messages := handleIncomingMessage(ctx, service, chatID, update.Message)
			for _, text := range messages {
				if strings.TrimSpace(text) == "" {
					continue
				}
				msg := tgbotapi.NewMessage(chatID, text)
				if _, err := api.Send(msg); err != nil {
					return fmt.Errorf("telegram send failed: %w", err)
				}
			}
		}
	}
}

func handleIncomingMessage(ctx context.Context, service *Service, chatID int64, message *tgbotapi.Message) []string {
	if message.IsCommand() {
		switch message.Command() {
		case "start":
			return service.HandleStart(chatID)
		case "login":
			return service.HandleLoginCommand(chatID)
		default:
			return []string{"Noma'lum buyruq. Mavjud buyruqlar: /start, /login"}
		}
	}

	return service.HandleText(ctx, chatID, message.Text)
}
