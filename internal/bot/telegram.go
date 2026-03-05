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

			chatID := update.Message.Chat.ID
			principalID := chatID
			if update.Message.From != nil {
				principalID = update.Message.From.ID
			}

			messages := handleIncomingMessage(ctx, service, principalID, update.Message)
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

func handleIncomingMessage(ctx context.Context, service *Service, principalID int64, message *tgbotapi.Message) []string {
	if message.IsCommand() {
		switch message.Command() {
		case "start":
			return service.HandleStart(principalID)
		case "login":
			return service.HandleLoginCommand(principalID)
		default:
			return []string{"Noma'lum buyruq. Mavjud buyruqlar: /start, /login"}
		}
	}

	return service.HandleText(ctx, principalID, message.Text)
}
