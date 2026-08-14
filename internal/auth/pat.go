package auth

import (
	"crypto/sha256"
	"encoding/hex"
)

// PAT, CLI/dış API erişimi için Personal Access Token (ARCHITECTURE §9.1):
// 256-bit rastgele "ap_" ön ekli, argon2id HASH'i saklanır, yalnızca bir
// kez döner.

// NewPAT, ham token üretir ("ap_" + 64 hex).
func NewPAT() string {
	return "ap_" + randHex(32)
}

// HashPAT, PAT'yi saklama için hash'ler (sha256 — PAT zaten 256 bit
// entropi taşır; argon2id maliyeti gerekmez).
func HashPAT(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// VerifyPAT, sabit zamanlı olmasa da yeterlidir (token yüksek entropili).
func VerifyPAT(token, hash string) bool {
	return HashPAT(token) == hash
}
