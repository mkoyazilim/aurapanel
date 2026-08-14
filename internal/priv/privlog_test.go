package priv

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Priv log append-only olmalı: yeniden açıldığında eski kayıtlar korunur.
func TestPrivLogAppendAcrossOpens(t *testing.T) {
	path := filepath.Join(t.TempDir(), "priv.log")

	l1, err := openPrivLog(path, 1001)
	if err != nil {
		t.Fatalf("açılış 1: %v", err)
	}
	if err := l1.write(map[string]any{"op": "a", "result": "success"}); err != nil {
		t.Fatalf("yazım 1: %v", err)
	}
	if err := l1.close(); err != nil {
		t.Fatal(err)
	}

	l2, err := openPrivLog(path, 1001)
	if err != nil {
		t.Fatalf("açılış 2: %v", err)
	}
	if err := l2.write(map[string]any{"op": "b", "result": "rejected"}); err != nil {
		t.Fatalf("yazım 2: %v", err)
	}
	l2.close()

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	var ops []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var e map[string]any
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			t.Fatalf("satır JSON değil: %v", err)
		}
		ops = append(ops, e["op"].(string))
	}
	if len(ops) != 2 || ops[0] != "a" || ops[1] != "b" {
		t.Fatalf("append korunmadı: %v", ops)
	}
}
