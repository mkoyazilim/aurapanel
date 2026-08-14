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

// apr1Encode, Apache to64 kodlaması — bayt sırası SIRALI DEĞİL,
// permütasyonludur: (0,6,12) (1,7,13) (2,8,14) (3,9,15) (4,10,5) + (11).
func apr1Encode(b []byte) string {
	to64 := func(v uint32, n int) string {
		s := ""
		for i := 0; i < n; i++ {
			s += string(apr1Alphabet[v&0x3f])
			v >>= 6
		}
		return s
	}
	group := func(i, j, k int) uint32 {
		return uint32(b[i])<<16 | uint32(b[j])<<8 | uint32(b[k])
	}
	out := to64(group(0, 6, 12), 4)
	out += to64(group(1, 7, 13), 4)
	out += to64(group(2, 8, 14), 4)
	out += to64(group(3, 9, 15), 4)
	out += to64(group(4, 10, 5), 4)
	out += to64(uint32(b[11]), 2)
	return out
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
