package bot

import "testing"

func TestResetSessionForCommandClearsTransientFlow(t *testing.T) {
	session := LoginSession{
		Step:             LoginStepAwaitingAPISecret,
		BaseURL:          "https://erp.example.com",
		APIKey:           "key",
		WelcomeMessageID: 10,
		PromptMessageID:  11,
		ActionStep:       ActionStepAwaitingQty,
		ActionType:       ActionTypeReceipt,
		SelectedItemCode: "ITEM-001",
		SelectedUOM:      "Kg",
		RequireUOMFirst:  true,
		LastActionType:   ActionTypeIssue,
		LastItemCode:     "ITEM-002",
		LastUOM:          "Nos",
		SettingsStep:     SettingsStepAwaitingPassword,
		SettingsAuthed:   true,
		SettingsPanelID:  12,
		SettingsSelect:   SettingsSelectionWarehouse,
	}

	got := resetSessionForCommand(session, "stock")

	if got != (LoginSession{}) {
		t.Fatalf("expected fully reset session, got %+v", got)
	}
}

func TestResetSessionForWerPreservesSettingsContext(t *testing.T) {
	session := LoginSession{
		Step:            LoginStepAwaitingAPIKey,
		ActionStep:      ActionStepAwaitingItem,
		SettingsAuthed:  true,
		SettingsPanelID: 55,
		SettingsSelect:  SettingsSelectionWarehouse,
	}

	got := resetSessionForCommand(session, "wer")

	if !got.SettingsAuthed {
		t.Fatal("expected settings auth to be preserved")
	}
	if got.SettingsPanelID != 55 {
		t.Fatalf("expected settings panel id 55, got %d", got.SettingsPanelID)
	}
	if got.Step != LoginStepNone || got.ActionStep != ActionStepNone {
		t.Fatalf("expected login/action flow to be cleared, got %+v", got)
	}
	if got.SettingsSelect != SettingsSelectionNone {
		t.Fatalf("expected settings selection to be cleared, got %+v", got)
	}
}
