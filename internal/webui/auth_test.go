package webui

import (
	"path/filepath"
	"testing"
)

func TestAuthBootstrapAndLogin(t *testing.T) {
	path := filepath.Join(t.TempDir(), "webauth.json")
	a, err := loadAuthStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if a.hasAccount() {
		t.Fatal("fresh store must have no account")
	}
	code := a.issueSetupCode()
	if code == "" || code != a.issueSetupCode() {
		t.Fatal("setup code must be stable until used")
	}

	if err := a.register("wrong-code", "me", "longenough"); err == nil {
		t.Fatal("register must require the setup code")
	}
	if err := a.register(code, "me", "short"); err == nil {
		t.Fatal("register must enforce password length")
	}
	if err := a.register(code, "me", "longenough"); err != nil {
		t.Fatal(err)
	}
	if err := a.register(code, "again", "longenough"); err == nil {
		t.Fatal("second account must be rejected")
	}

	// Reload from disk: account persists, setup code does not.
	b, err := loadAuthStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if !b.hasAccount() {
		t.Fatal("account must persist")
	}
	if b.login("me", "wrongpassword") {
		t.Fatal("wrong password accepted")
	}
	if !b.login("me", "longenough") {
		t.Fatal("correct login rejected")
	}

	tok := b.newSession()
	if !b.validSession(tok) {
		t.Fatal("fresh session invalid")
	}
	b.dropSession(tok)
	if b.validSession(tok) {
		t.Fatal("dropped session still valid")
	}
}

func TestIsLoopbackAddr(t *testing.T) {
	cases := map[string]bool{
		"localhost:8737": true,
		"127.0.0.1:80":   true,
		"[::1]:8737":     true,
		":8737":          false,
		"0.0.0.0:8737":   false,
		"192.168.1.5:80": false,
	}
	for addr, want := range cases {
		if got := IsLoopbackAddr(addr); got != want {
			t.Errorf("IsLoopbackAddr(%q) = %v, want %v", addr, got, want)
		}
	}
}
