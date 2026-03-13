package core

import (
	"testing"

	"cid_gio_gio/internal/config"
)

func TestChangeAccountNumberAppliesConfiguredRangeAndCodeMap(t *testing.T) {
	rules := config.CidRulesConfig{
		RequiredPrefix: "5",
		ValidLength:    20,
		AccNumAdd:      2100,
		AccountRanges:  []config.AccountRange{{From: 2000, To: 2200, Delta: 2100}},
		TestCodeMap:    map[string]string{"E603": "E602"},
	}
	input := []byte("50000002000E60301050")

	out, err := ChangeAccountNumber(input, rules)
	if err != nil {
		t.Fatalf("ChangeAccountNumber() error = %v", err)
	}

	got := string(out)
	want := "50000004100E60201050" + string(byte(0x14))
	if got != want {
		t.Fatalf("ChangeAccountNumber() = %q, want %q", got, want)
	}
}

func TestIsHeartbeatRecognizesExpectedPayload(t *testing.T) {
	if !IsHeartbeat("1000           @    ") {
		t.Fatal("expected heartbeat payload to be recognized")
	}
	if IsHeartbeat("50000002000E60301050") {
		t.Fatal("unexpected normal CID payload recognized as heartbeat")
	}
}

func TestByteValidationHelpers(t *testing.T) {
	rules := config.CidRulesConfig{
		RequiredPrefix: "5",
		ValidLength:    20,
	}
	if !IsMessageValidBytes([]byte("50000002000E60301050"), rules) {
		t.Fatal("expected bytes message to be valid")
	}
	if !IsHeartbeatBytes([]byte("1000           @    ")) {
		t.Fatal("expected heartbeat bytes to be recognized")
	}
}

func TestChangeAccountNumberFallbackForWideMapping(t *testing.T) {
	rules := config.CidRulesConfig{
		RequiredPrefix: "5",
		ValidLength:    20,
		AccNumAdd:      0,
		AccountRanges:  []config.AccountRange{{From: 2000, To: 2200, Delta: 2100}},
		TestCodeMap:    map[string]string{"E603": "E603X"},
	}
	input := []byte("50000002000E60301050")

	out, err := ChangeAccountNumber(input, rules)
	if err != nil {
		t.Fatalf("ChangeAccountNumber() error = %v", err)
	}
	want := "50000004100E603X01050" + string(byte(0x14))
	if string(out) != want {
		t.Fatalf("ChangeAccountNumber() = %q, want %q", string(out), want)
	}
}

func TestChangeAccountNumberRejectsZeroAccount(t *testing.T) {
	rules := config.CidRulesConfig{
		RequiredPrefix: "5",
		ValidLength:    20,
		AccNumAdd:      2100,
		AccountRanges:  []config.AccountRange{{From: 2000, To: 2200, Delta: 2100}},
	}
	input := []byte("50000000000E60301050")

	_, err := ChangeAccountNumber(input, rules)
	if err == nil {
		t.Fatal("expected error for account 0000")
	}
}

func TestChangeAccountNumberRejectsOutOfRangeAfterDelta(t *testing.T) {
	rules := config.CidRulesConfig{
		RequiredPrefix: "5",
		ValidLength:    20,
		AccountRanges:  []config.AccountRange{{From: 2000, To: 2200, Delta: 8000}},
	}
	input := []byte("50000002000E60301050")

	_, err := ChangeAccountNumber(input, rules)
	if err == nil {
		t.Fatal("expected error for account overflow after delta")
	}
}
