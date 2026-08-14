package priv

import (
	"strings"
	"testing"
)

// Bilinen htpasswd vektörü: "password" + "salt" → bilinen apr1 değeri.
func TestApr1KnownVector(t *testing.T) {
	got := apr1Crypt("password", "salt")
	want := "$apr1$salt$Xxd1irWT9ycqoYxGFn4cb."
	if got != want {
		t.Fatalf("apr1: %s (beklenen %s)", got, want)
	}
}

func TestApr1SaltAndLength(t *testing.T) {
	h := apr1Crypt("güçlü-parola", apr1Salt())
	if !strings.HasPrefix(h, "$apr1$") {
		t.Fatalf("ön ek yok: %s", h)
	}
	if len(h) != len("$apr1$")+8+1+22 {
		t.Fatalf("uzunluk hatalı: %d (%s)", len(h), h)
	}
	// Aynı parola farklı salt → farklı hash.
	h2 := apr1Crypt("güçlü-parola", "basKaslt")
	if h == h2 {
		t.Fatal("salt farklıyken aynı hash")
	}
}
