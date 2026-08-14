// apr1.go: Apache MD5 crypt (htpasswd -m) uygulaması — OLS WebAdmin
// htpasswd dosyası için dış binary bağımlılığı olmadan (OLS yalnızca
// htpasswd.php taşır, binary yoktur).
package priv

import (
	"crypto/md5"
	"crypto/rand"
)

// apr1Alphabet, Apache MD5 crypt özel base64 alfabesi.
const apr1Alphabet = "./0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// apr1Crypt, "$apr1$<salt>$<hash>" üretir.
func apr1Crypt(password, salt string) string {
	ctx := md5.New()
	ctx.Write([]byte(password + "$apr1$" + salt))

	alt := md5.New()
	alt.Write([]byte(password + salt + password))
	altSum := alt.Sum(nil)

	for i := len(password); i > 0; i -= 16 {
		end := i
		if end > 16 {
			end = 16
		}
		ctx.Write(altSum[:end])
	}
	for i := len(password); i > 0; i >>= 1 {
		if i&1 == 1 {
			ctx.Write([]byte{0})
		} else {
			ctx.Write([]byte{password[0]})
		}
	}
	final := ctx.Sum(nil)

	// 1000 tur finalizasyonu.
	for i := 0; i < 1000; i++ {
		round := md5.New()
		if i&1 == 1 {
			round.Write([]byte(password))
		} else {
			round.Write(final)
		}
		if i%3 != 0 {
			round.Write([]byte(salt))
		}
		if i%7 != 0 {
			round.Write([]byte(password))
		}
		if i&1 == 1 {
			round.Write(final)
		} else {
			round.Write([]byte(password))
		}
		final = round.Sum(nil)
	}

	return "$apr1$" + salt + "$" + apr1Encode(final)
}

// apr1Encode, özel alfabeyle kodlama (4 bayt → 4×6 bit).
func apr1Encode(b []byte) string {
	out := ""
	for i := 0; i < 16; i += 3 {
		chunk := uint32(b[i])<<16
		if i+1 < len(b) {
			chunk |= uint32(b[i+1]) << 8
		}
		if i+2 < len(b) {
			chunk |= uint32(b[i+2])
		}
		for j := 0; j < 4; j++ {
			out += string(apr1Alphabet[chunk&0x3f])
			chunk >>= 6
		}
	}
	return out[:22] // 64+1 bit → 22 karakter
}

// apr1Salt, 8 karakterlik rastgele salt üretir.
func apr1Salt() string {
	const allowed = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789./"
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "saltsalt"
	}
	for i := range b {
		b[i] = allowed[int(b[i])%len(allowed)]
	}
	return string(b)
}
