package audit

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mkoyazilim/aurapanel/internal/store"
)

func TestWriteAndList(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "audit.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer st.Close()
	if err := st.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	svc := New(st)
	ctx := context.Background()

	if err := svc.Write(ctx, Event{
		User:   "admin",
		IP:     "127.0.0.1",
		Action: "site.create",
		Target: "site001",
		Extra:  map[string]any{"domain": "example.com"},
	}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	evs, err := svc.List(ctx, 1)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(evs) != 1 {
		t.Fatalf("1 kayıt bekleniyordu, %d geldi", len(evs))
	}
	e := evs[0]
	if e.Action != "site.create" || e.User != "admin" || e.IP != "127.0.0.1" || e.Target != "site001" {
		t.Errorf("alanlar eksik: %+v", e)
	}
	if e.RequestID == "" {
		t.Error("request_id otomatik üretilmeli")
	}
	if e.Result != "success" {
		t.Errorf("varsayılan result 'success' olmalı, %q geldi", e.Result)
	}
	if !strings.Contains(e.Extra, "example.com") {
		t.Errorf("extra JSON'a yazılmadı: %q", e.Extra)
	}
}
