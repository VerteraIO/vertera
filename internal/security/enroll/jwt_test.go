package enroll

import (
	"testing"
	"time"
)

func TestIssueAndVerifyToken(t *testing.T) {
	secret := []byte("test-secret")
	tok, err := IssueToken(secret, 2*time.Minute)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}
	claims, err := VerifyToken(secret, tok)
	if err != nil {
		t.Fatalf("VerifyToken: %v", err)
	}
	if claims.ExpiresAt == nil {
		t.Fatalf("expected ExpiresAt to be set")
	}
	if time.Until(claims.ExpiresAt.Time) <= 0 {
		t.Fatalf("token already expired")
	}
}
