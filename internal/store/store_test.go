package store

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

// testStore, geçici dizinde migration uygulanmış bir Store döndürür.
func testStore(t *testing.T) *Store {
	t.Helper()
	st, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	if err := st.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return st
}

// Migration idempotent olmalı: ikinci kez çalıştırmak hata vermemeli.
func TestMigrateIdempotent(t *testing.T) {
	st := testStore(t)
	if err := st.Migrate(); err != nil {
		t.Fatalf("ikinci Migrate: %v", err)
	}
}

// Şema v1'deki tüm tablolar kurulmuş olmalı (ARCHITECTURE §4.1).
func TestSchemaTablesExist(t *testing.T) {
	st := testStore(t)
	want := []string{
		"users", "roles", "sessions", "sites", "domains", "ssl_certificates",
		"php_versions", "php_pools", "databases", "database_users", "sftp_accounts",
		"cron_jobs", "backups", "audit_logs", "security_profiles", "system_settings",
		"drift_events", "metrics",
	}
	for _, table := range want {
		var n int
		if err := st.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Errorf("tablo %q kurulmamış", table)
		}
	}
}

// Seed veriler: roller ve güvenlik profilleri.
func TestSeedData(t *testing.T) {
	st := testStore(t)
	for _, q := range []string{
		`SELECT COUNT(*) FROM roles WHERE name IN ('admin','user')`,
		`SELECT COUNT(*) FROM security_profiles WHERE name IN ('compatibility','balanced','hardened')`,
	} {
		var n int
		if err := st.db.QueryRow(q).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n == 0 {
			t.Errorf("seed eksik: %s", q)
		}
	}
}

// Audit log append-only: UPDATE ve DELETE trigger'larla reddedilmeli (İlke 13).
func TestAuditAppendOnly(t *testing.T) {
	ctx := context.Background()
	st := testStore(t)

	id, err := st.InsertAuditLog(ctx, AuditLog{
		Timestamp: "2026-08-14T00:00:00Z",
		User:      "tester",
		Action:    "test.write",
		Result:    "success",
		RequestID: "r1",
	})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	if _, err := st.db.ExecContext(ctx, `UPDATE audit_logs SET action='hack' WHERE id=?`, id); err == nil {
		t.Fatal("UPDATE yapılabildi; append-only ihlali")
	} else if !strings.Contains(err.Error(), "append-only") {
		t.Fatalf("beklenmeyen hata: %v", err)
	}
	if _, err := st.db.ExecContext(ctx, `DELETE FROM audit_logs WHERE id=?`, id); err == nil {
		t.Fatal("DELETE yapılabildi; append-only ihlali")
	}
}

// Yazma + listeleme akışı: en yeni kayıt önce gelir, limit uygulanır.
func TestInsertAndListAudit(t *testing.T) {
	ctx := context.Background()
	st := testStore(t)

	for i := 0; i < 3; i++ {
		action := "test.write." + string(rune('0'+i))
		if _, err := st.InsertAuditLog(ctx, AuditLog{
			Timestamp: "2026-08-14T00:00:0" + string(rune('0'+i)) + "Z",
			User:      "tester",
			Action:    action,
			Result:    "success",
			RequestID: "r" + string(rune('0'+i)),
		}); err != nil {
			t.Fatalf("insert %d: %v", i, err)
		}
	}

	got, err := st.ListAuditLogs(ctx, 2)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("limit=2 bekleniyordu, %d kayıt geldi", len(got))
	}
	if got[0].Action != "test.write.2" {
		t.Errorf("en yeni kayıt önce gelmeli, ilk kayıt %q", got[0].Action)
	}
}

// Foreign key zorunluluğu aktif olmalı.
func TestForeignKeyEnforced(t *testing.T) {
	st := testStore(t)

	if _, err := st.db.Exec(`INSERT INTO sites (id, name, linux_user, home_dir)
		VALUES ('site001', 'example.com', 'www-site001', '/srv/aurapanel/sites/example.com/home')`); err != nil {
		t.Fatalf("site insert: %v", err)
	}
	if _, err := st.db.Exec(`INSERT INTO domains (site_id, domain) VALUES ('yok', 'a.com')`); err == nil {
		t.Fatal("olmayan site_id'ye domain eklendi; FK ihlali yakalanmadı")
	}
}
