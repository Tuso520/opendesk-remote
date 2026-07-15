package tokens

import "testing"

func TestHashToken(t *testing.T) {
	hash := HashToken("secret", "token")
	if hash == "token" || len(hash) != 64 {
		t.Fatalf("unexpected token hash: %q", hash)
	}
	if !EqualHash(hash, HashToken("secret", "token")) {
		t.Fatal("expected equal hashes")
	}
}
