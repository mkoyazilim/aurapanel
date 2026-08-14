package priv

import (
	"encoding/json"
	"errors"
	"fmt"
)

type scriptOpArgs struct {
	Site      string `json:"site"`
	Cwd       string `json:"cwd"`
	ScriptB64 string `json:"script_b64"`
}

func validateScriptOp(raw json.RawMessage) (scriptOpArgs, []byte, error) {
	var a scriptOpArgs
	if err := strictDecode(raw, &a); err != nil {
		return a, nil, fmt.Errorf("script.op: %w", err)
	}
	if !reSiteID.MatchString(a.Site) {
		return a, nil, errors.New("script.op: site kimliği geçersiz")
	}
	if a.ScriptB64 == "" {
		return a, nil, errors.New("script.op: script_b64 gerekli")
	}
	b, err := b64Decode(a.ScriptB64)
	if err != nil || len(b) > 4<<20 { // 4MB script limiti
		return a, nil, errors.New("script.op: script geçersiz veya çok büyük")
	}
	return a, b, nil
}
