package ssl

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
)

// validatePair, sertifika ile özel anahtarın eşleştiğini doğrular ve
// sertifika nesnesini döndürür. Eşleşmeyen/gayri geçerli çiftler
// KURULUMDAN ÖNCE reddedilir — OLS asla bozuk TLS yapılandırması almaz.
func validatePair(certPEM, keyPEM []byte) (*x509.Certificate, error) {
	cert, err := parseCert(certPEM)
	if err != nil {
		return nil, err
	}
	key, err := parseKey(keyPEM)
	if err != nil {
		return nil, err
	}

	switch k := key.(type) {
	case *rsa.PrivateKey:
		if !k.PublicKey.Equal(cert.PublicKey) {
			return nil, errors.New("özel anahtar sertifikayla eşleşmiyor")
		}
	case *ecdsa.PrivateKey:
		if !k.PublicKey.Equal(cert.PublicKey) {
			return nil, errors.New("özel anahtar sertifikayla eşleşmiyor")
		}
	case ed25519.PrivateKey:
		if !k.Public().(ed25519.PublicKey).Equal(cert.PublicKey) {
			return nil, errors.New("özel anahtar sertifikayla eşleşmiyor")
		}
	default:
		return nil, fmt.Errorf("desteklenmeyen anahtar türü: %T", key)
	}
	return cert, nil
}

func parseCert(pemBytes []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, errors.New("sertifika PEM çözümlenemedi")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("sertifika geçersiz: %w", err)
	}
	return cert, nil
}

// parseKey, PKCS8 / PKCS1 / SEC1 / PKIX biçimlerini destekler.
func parseKey(pemBytes []byte) (any, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("anahtar PEM çözümlenemedi")
	}
	switch block.Type {
	case "PRIVATE KEY": // PKCS8
		if k, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
			return k, nil
		}
	case "RSA PRIVATE KEY": // PKCS1
		if k, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
			return k, nil
		}
	case "EC PRIVATE KEY": // SEC1
		if k, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
			return k, nil
		}
	case "PUBLIC KEY":
		return nil, errors.New("özel anahtar yerine genel anahtar verildi")
	}
	return nil, errors.New("anahtar çözümlenemedi (PKCS8/PKCS1/SEC1 bekleniyor)")
}
