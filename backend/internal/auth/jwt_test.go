package auth

import "testing"

func TestIssueAndVerifyToken(t *testing.T) {
	issuer := NewTokenIssuer("test-secret")

	token, err := issuer.IssueToken("user-123")
	if err != nil {
		t.Fatalf("IssueToken returned error: %v", err)
	}

	userID, err := issuer.VerifyToken(token)
	if err != nil {
		t.Fatalf("VerifyToken returned error: %v", err)
	}
	if userID != "user-123" {
		t.Errorf("VerifyToken returned userID %q, want %q", userID, "user-123")
	}
}

func TestVerifyTokenRejectsWrongSecret(t *testing.T) {
	token, err := NewTokenIssuer("secret-a").IssueToken("user-123")
	if err != nil {
		t.Fatalf("IssueToken returned error: %v", err)
	}

	if _, err := NewTokenIssuer("secret-b").VerifyToken(token); err == nil {
		t.Error("VerifyToken accepted a token signed with a different secret")
	}
}

func TestVerifyTokenRejectsGarbage(t *testing.T) {
	issuer := NewTokenIssuer("test-secret")
	if _, err := issuer.VerifyToken("not-a-jwt"); err == nil {
		t.Error("VerifyToken accepted a malformed token")
	}
}
