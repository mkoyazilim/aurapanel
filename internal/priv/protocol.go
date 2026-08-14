package priv

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// maxRequestSize, tek isteğin üst sınırı (64 KiB). Daha büyük girdiler reddedilir.
const maxRequestSize = 64 << 10

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

// decodeRequest, tek bir JSON isteğini sıkı kurallarla okur:
// bilinmeyen alan reddi, 64 KiB boyut sınırı, artık veri reddi, boş op reddi.
func decodeRequest(r io.Reader) (*Request, error) {
	b, err := io.ReadAll(io.LimitReader(r, maxRequestSize+1))
	if err != nil {
		return nil, fmt.Errorf("okuma: %w", err)
	}
	if len(b) > maxRequestSize {
		return nil, errors.New("istek boyutu sınırı aşıldı (64 KiB)")
	}

	var req Request
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		return nil, fmt.Errorf("json çözümleme: %w", err)
	}
	if dec.More() {
		return nil, errors.New("istekte artık veri var")
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
