package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// ComputeHMAC вычисляет HMAC-SHA256 для данных с ключом
func ComputeHMAC(data []byte, key string) string {
	if key == "" {
		return ""
	}

	h := hmac.New(sha256.New, []byte(key))
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

// VerifyHMAC проверяет, что HMAC корректен
func VerifyHMAC(data []byte, key string, receivedMAC string) bool {
	if key == "" {
		// Если ключ не задан, не проверяем
		return true
	}

	expectedMAC := ComputeHMAC(data, key)
	return hmac.Equal([]byte(expectedMAC), []byte(receivedMAC))
}
