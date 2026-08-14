package priv

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestDecodeValid(t *testing.T) {
	req, err := decodeRequest(bytes.NewBufferString(`{"op":"priv.ping","args":{"x":1},"request_id":"r1"}`))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if req.Op != "priv.ping" || req.RequestID != "r1" {
		t.Fatalf("alanlar hatalı: %+v", req)
	}
}

// args verilmediğinde {} kabul edilmeli.
func TestDecodeMissingArgs(t *testing.T) {
	req, err := decodeRequest(bytes.NewBufferString(`{"op":"priv.ping"}`))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if string(req.Args) != `{}` {
		t.Fatalf("args varsayılanı bekleniyordu: %s", req.Args)
	}
}

func TestDecodeRejectsUnknownFields(t *testing.T) {
	if _, err := decodeRequest(bytes.NewBufferString(`{"op":"priv.ping","shell_cmd":"rm -rf /"}`)); err == nil {
		t.Fatal("bilinmeyen alan kabul edildi")
	}
}

func TestDecodeRejectsTrailingData(t *testing.T) {
	if _, err := decodeRequest(bytes.NewBufferString(`{"op":"priv.ping"}{"op":"x"}`)); err == nil {
		t.Fatal("artık veri kabul edildi")
	}
}

func TestDecodeRejectsEmptyOp(t *testing.T) {
	if _, err := decodeRequest(bytes.NewBufferString(`{"op":""}`)); err == nil {
		t.Fatal("boş op kabul edildi")
	}
}

func TestDecodeRejectsOversize(t *testing.T) {
	big := `{"op":"priv.ping","args":{"pad":"` + strings.Repeat("a", maxRequestSize) + `"}}`
	if _, err := decodeRequest(bytes.NewBufferString(big)); err == nil {
		t.Fatal("64 KiB üstü istek kabul edildi")
	}
}

func TestDecodeRejectsGarbage(t *testing.T) {
	for _, in := range []string{``, `{`, `"x"`, `{"op":1}`, `{"args":"string değil"}`} {
		if _, err := decodeRequest(bytes.NewBufferString(in)); err == nil {
			t.Fatalf("bozuk girdi kabul edildi: %q", in)
		}
	}
}

func TestResponseRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	if err := writeResponse(&buf, Response{OK: true, Data: map[string]any{"exists": true}, RequestID: "r9"}); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !strings.HasSuffix(got, "\n") {
		t.Fatal("yanıt satır sonu ile bitmeli")
	}
	var resp Response
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("yanıt çözümlenemedi: %v", err)
	}
	if !resp.OK || resp.RequestID != "r9" {
		t.Fatalf("yanıt alanları hatalı: %+v", resp)
	}
}
