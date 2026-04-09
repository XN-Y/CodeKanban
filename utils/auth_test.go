package utils

import (
	"errors"
	"testing"
	"time"
)

func TestHashClientSecretRoundTrip(t *testing.T) {
	encoded, err := HashClientSecret("client-hash-value")
	if err != nil {
		t.Fatalf("HashClientSecret returned error: %v", err)
	}

	ok, err := VerifyClientSecret("client-hash-value", encoded)
	if err != nil {
		t.Fatalf("VerifyClientSecret returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected VerifyClientSecret to accept the original client hash")
	}

	ok, err = VerifyClientSecret("different-client-hash", encoded)
	if err != nil {
		t.Fatalf("VerifyClientSecret returned error for mismatched hash: %v", err)
	}
	if ok {
		t.Fatal("expected VerifyClientSecret to reject a different client hash")
	}
}

func TestAuthSessionTokenLifecycle(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	token, err := IssueAuthSessionToken("secret-token-key", 5*time.Minute, now)
	if err != nil {
		t.Fatalf("IssueAuthSessionToken returned error: %v", err)
	}

	claims, err := VerifyAuthSessionToken(token, "secret-token-key", now.Add(4*time.Minute))
	if err != nil {
		t.Fatalf("VerifyAuthSessionToken returned error: %v", err)
	}
	if claims.ExpiresAt != now.Add(5*time.Minute).Unix() {
		t.Fatalf("unexpected expiry: got %d want %d", claims.ExpiresAt, now.Add(5*time.Minute).Unix())
	}

	if _, err := VerifyAuthSessionToken(token, "secret-token-key", now.Add(6*time.Minute)); !errors.Is(err, ErrExpiredSessionToken) {
		t.Fatalf("expected ErrExpiredSessionToken, got %v", err)
	}
}
