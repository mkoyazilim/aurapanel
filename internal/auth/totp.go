package auth

import (
	"errors"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// TOTP ile ilgili yardımcılar (ARCHITECTURE §9.2: MFA = TOTP + WebAuthn;
// WebAuthn kayıt akışı sunucu fazında bağlanacak).

// GenerateTOTP, yeni TOTP sırrı üretir: (secret base32, otpauth URL).
func GenerateTOTP(issuer, account string) (string, string, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: account,
		Algorithm:   otp.AlgorithmSHA1,
	})
	if err != nil {
		return "", "", err
	}
	return key.Secret(), key.URL(), nil
}

// VerifyTOTP, TOTP kodunu doğrular (±1 zaman penceresi toleransı).
func VerifyTOTP(secret, code string) (bool, error) {
	if secret == "" || code == "" {
		return false, errors.New("TOTP sırrı veya kodu eksik")
	}
	return totp.ValidateCustom(code, secret, time.Now().UTC(),
		totp.ValidateOpts{Period: 30, Skew: 1, Digits: 6, Algorithm: otp.AlgorithmSHA1})
}
