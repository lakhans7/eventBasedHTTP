package auth

import (
	"testing"
	"time"
)

func TestJWTIssueAndParse(t *testing.T) {
	issuer := NewJWTIssuer("test-secret", time.Minute)
	token, err := issuer.Issue("user-1", "session-1")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	claims, err := issuer.Parse(token)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if claims.UserID != "user-1" || claims.SessionID != "session-1" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestJWTRejectsExpiredToken(t *testing.T) {
	issuer := NewJWTIssuer("test-secret", -time.Minute) // already expired
	token, err := issuer.Issue("user-1", "session-1")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if _, err := issuer.Parse(token); err != ErrInvalidToken {
		t.Fatalf("expected ErrInvalidToken for expired token, got %v", err)
	}
}

func TestJWTRejectsTokenSignedWithDifferentSecret(t *testing.T) {
	a := NewJWTIssuer("secret-a", time.Minute)
	b := NewJWTIssuer("secret-b", time.Minute)

	token, err := a.Issue("user-1", "session-1")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if _, err := b.Parse(token); err != ErrInvalidToken {
		t.Fatalf("expected ErrInvalidToken for token signed with a different secret, got %v", err)
	}
}
