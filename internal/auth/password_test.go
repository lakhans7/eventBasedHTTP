package auth

import "testing"

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hash == "correct horse battery staple" {
		t.Fatal("password was not hashed")
	}
	if !VerifyPassword(hash, "correct horse battery staple") {
		t.Fatal("VerifyPassword should accept the correct password")
	}
	if VerifyPassword(hash, "wrong password") {
		t.Fatal("VerifyPassword should reject an incorrect password")
	}
}

func TestVerifyPasswordRejectsEmptyHash(t *testing.T) {
	// Guards against a Google-only account (no local password) accidentally
	// accepting an empty-string password.
	if VerifyPassword("", "anything") {
		t.Fatal("VerifyPassword must reject an empty stored hash")
	}
}
