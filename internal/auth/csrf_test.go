package auth

import "testing"

func TestValidCSRF(t *testing.T) {
	if !ValidCSRF("token-123", "token-123") {
		t.Fatal("matching tokens should be valid")
	}
	if ValidCSRF("token-123", "token-456") {
		t.Fatal("mismatched tokens should be invalid")
	}
	if ValidCSRF("", "") {
		t.Fatal("empty tokens must never be considered valid (an attacker page also sends no header)")
	}
	if ValidCSRF("token-123", "") {
		t.Fatal("a missing header must be rejected even if the cookie is present")
	}
}
