package store

import "time"

type Credentials struct {
	BaseURL   string
	APIKey    string
	APISecret string
	Username  string
	Roles     []string
	UpdatedAt time.Time
}

type CredentialStore interface {
	Save(chatID int64, creds Credentials)
	Get(chatID int64) (Credentials, bool)
}
