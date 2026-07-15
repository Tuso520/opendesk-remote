package auth

import "testing"

func TestPasswordHashAndVerify(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("hash failed: %v", err)
	}
	if hash == "correct horse battery staple" {
		t.Fatal("password hash must not equal plaintext")
	}
	if !VerifyPassword(hash, "correct horse battery staple") {
		t.Fatal("expected password verification to pass")
	}
	if VerifyPassword(hash, "wrong") {
		t.Fatal("expected wrong password to fail")
	}
}
