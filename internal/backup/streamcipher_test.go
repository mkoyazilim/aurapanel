package backup

import (
	"bytes"
	"crypto/rand"
	"io"
	"testing"
)

func TestCipherRoundtripBuffer(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)

	var buf bytes.Buffer
	ew, err := EncryptWriter(key, &buf)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ew.Write([]byte("hello world")); err != nil {
		t.Fatal(err)
	}
	ew.Close()

	if !bytes.HasPrefix(buf.Bytes(), magic) {
		t.Fatal("imza yazılmadı")
	}
	if bytes.Contains(buf.Bytes(), []byte("hello world")) {
		t.Fatal("şifreli akışta düz metin var")
	}

	dr, err := DecryptReader(key, &buf)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := io.ReadAll(dr)
	if err != nil {
		t.Fatal(err)
	}
	if string(plain) != "hello world" {
		t.Fatalf("roundtrip bozuk: %q", plain)
	}
}
