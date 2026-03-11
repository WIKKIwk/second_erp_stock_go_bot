package mobileapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"golang.org/x/oauth2"
)

type fakeTokenSource struct{}

func (fakeTokenSource) Token() (*oauth2.Token, error) {
	return &oauth2.Token{AccessToken: "token"}, nil
}

func TestFCMSenderSendToKey(t *testing.T) {
	store := NewPushTokenStore(t.TempDir() + "/push_tokens.json")
	if err := store.Put("supplier:SUP-001", "device-token", "android"); err != nil {
		t.Fatalf("seed push token: %v", err)
	}

	requestSeen := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestSeen = true
		if got := r.Header.Get("Authorization"); got != "Bearer token" {
			t.Fatalf("unexpected auth header: %s", got)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sender := &fcmSender{
		store:      store,
		httpClient: server.Client(),
		tokenSrc:   fakeTokenSource{},
		projectID:  "demo",
		endpoint:   server.URL,
	}

	if err := sender.SendToKey(context.Background(), "supplier:SUP-001", "Title", "Body", map[string]string{"id": "1"}); err != nil {
		t.Fatalf("SendToKey() error = %v", err)
	}
	if !requestSeen {
		t.Fatal("expected request to be sent")
	}
}

func TestDiscoverServiceAccountPathReturnsFirebaseAdminSDKFile(t *testing.T) {
	dir := t.TempDir()
	file := dir + "/demo-firebase-adminsdk.json"
	if err := os.WriteFile(file, []byte("{}"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	prev, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(prev)

	got := discoverServiceAccountPath()
	if !strings.Contains(got, "firebase-adminsdk") {
		t.Fatalf("unexpected path: %s", got)
	}
}
