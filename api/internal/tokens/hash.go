package tokens

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

func HashToken(secret, token string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(token))
	return hex.EncodeToString(mac.Sum(nil))
}

func EqualHash(a, b string) bool {
	return hmac.Equal([]byte(a), []byte(b))
}
