package marketplace

import "testing"

func TestTokenCipherRoundTrip(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	cipher, err := NewTokenCipher(key)
	if err != nil {
		t.Fatalf("NewTokenCipher: %v", err)
	}

	ciphertext, err := cipher.Encrypt("super-secret-token")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if string(ciphertext) == "super-secret-token" {
		t.Fatal("Encrypt must not return the plaintext")
	}

	plaintext, err := cipher.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if plaintext != "super-secret-token" {
		t.Fatalf("expected round-tripped plaintext, got %q", plaintext)
	}
}

func TestTokenCipherRejectsShortKey(t *testing.T) {
	if _, err := NewTokenCipher([]byte("too-short")); err == nil {
		t.Fatal("expected an error for a non-32-byte key")
	}
}

func TestTokenCipherRejectsTamperedCiphertext(t *testing.T) {
	key := make([]byte, 32)
	cipher, _ := NewTokenCipher(key)
	ciphertext, _ := cipher.Encrypt("value")
	ciphertext[len(ciphertext)-1] ^= 0xFF // flip a bit

	if _, err := cipher.Decrypt(ciphertext); err == nil {
		t.Fatal("expected Decrypt to reject a tampered ciphertext (GCM authentication should fail)")
	}
}
