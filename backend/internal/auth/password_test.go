package auth

import "testing"

func TestHashAndCheckPassword(t *testing.T) {
	hash, err := HashPassword("correct-horse-battery-staple")
	if err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}
	if hash == "correct-horse-battery-staple" {
		t.Fatal("HashPassword did not hash the password")
	}
	if !CheckPassword(hash, "correct-horse-battery-staple") {
		t.Error("CheckPassword rejected the correct password")
	}
	if CheckPassword(hash, "wrong-password") {
		t.Error("CheckPassword accepted an incorrect password")
	}
}
