package priv

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// maxRequestSize, tek isteğin üst sınırı: en büyük file.op write isteğini
// (16 MiB içerik, base64 ≈ 22.4 MB) karşılayacak kadar geniş.
const maxRequestSize = 24 << 20

// Request, helper'a gönderilen tek satırlık JSON istek.
type Request struct {
	Op        string          `json:"op"`
	Args      json.RawMessage `json:"args"`
	RequestID string          `json:"request_id"`
}

// Response, helper'ın her durumda döndürdüğü yanıt.
type Response struct {
	OK        bool   `json:"ok"`
	Data      any    `json:"data,omitempty"`
	Error     string `json:"error,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

// decodeRequest, bağlantıdan TEK bir JSON isteğini sıkı kurallarla okur:
// bilinmeyen alan reddi, 64 KiB boyut sınırı, artık veri reddi, boş op reddi.
//
// NOT: Akıştan tek JSON değeri okunur — asla EOF beklenmez. İstemci,
// isteği gönderdikten sonra yazma tarafını kapatmadan yanıt bekler;
// EOF beklemek her isteği zaman aşımına sokardı (sunucu smoke testinde
// yakalanan gerçek hata — regresyon testi: TestDecodeStreamingWithoutHalfClose).
func decodeRequest(r io.Reader) (*Request, error) {
	dec := json.NewDecoder(io.LimitReader(r, maxRequestSize+1))
	dec.DisallowUnknownFields()

	var req Request
	if err := dec.Decode(&req); err != nil {
		if err == io.EOF {
			return nil, errors.New("boş istek")
		}
		return nil, fmt.Errorf("json çözümleme: %w", err)
	}
	// Artık veri kontrolü YALNIZCA decoder'ın tamponunda kalana bakar —
	// canlı sokette More()/yeni okuma BLOKE OLUR (regresyon testi:
	// TestDecodeStreamingWithoutHalfClose). Tampondaki beyaz boşluk dışı
	// artık baytlar reddedilir; tampon dışı artıklar bağlantı kapanışında
	// atılır ve hiçbir işlem görmez.
	if br := dec.Buffered(); br != nil {
		leftover, err := io.ReadAll(br)
		if err == nil && len(bytes.TrimSpace(leftover)) > 0 {
			return nil, errors.New("istekte artık veri var")
		}
	}
	if req.Op == "" {
		return nil, errors.New("op boş olamaz")
	}
	if req.Args == nil {
		req.Args = json.RawMessage(`{}`)
	}
	return &req, nil
}

// writeResponse, yanıtı tek satır JSON olarak yazar.
func writeResponse(w io.Writer, resp Response) error {
	b, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	_, err = w.Write(append(b, '\n'))
	return err
}
