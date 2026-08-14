package priv

import (
	"encoding/json"
	"path"
	"strings"
	"testing"
	"time"
)

func testCfg() *runtimeCfg {
	return &runtimeCfg{
		panelUser:  "aurapanel",
		panelUID:   1001,
		panelGID:   1001,
		quotaFS:    "/",
		opTimeout:  30 * time.Second,
		sitesRoot:  "/srv/aurapanel/sites",
		stageDir:   "/var/lib/aurapanel/stage",
		nftDir:     "/etc/aurapanel/nftables",
		cgroupBase: "/sys/fs/cgroup/aurapanel",
	}
}

// registry'deki op sayısı, ARCHITECTURE §3.2 allowlist'iyle birebir eşleşmeli.
func TestRegistryAllowlist(t *testing.T) {
	reg := newRegistry(testCfg())
	want := []string{
		"priv.ping", "user.create", "user.delete", "user.exists",
		"cgroup.bootstrap", "cgroup.limits", "quota.set",
		"firewall.apply", "sshd.install_config", "logrotate.install_config",
		"ols.test", "ols.read_bundle", "ols.install_bundle", "ols.remove_bundle", "ols.reload",
	}
	if len(reg) != len(want) {
		t.Fatalf("op sayısı beklenmiyor: %d (beklenen %d)", len(reg), len(want))
	}
	for _, op := range want {
		if _, ok := reg[op]; !ok {
			t.Errorf("allowlist op'u eksik: %s", op)
		}
	}
}

// plan'daki TÜM exec bin'leri binPaths haritasından gelmeli.
// Kullanıcı girdisinden binary adı üretilemez (İlke 3).
func assertPlanBins(t *testing.T, p *plan) {
	t.Helper()
	for _, a := range p.actions {
		if a.kind != actExec {
			continue
		}
		allowed := false
		for _, path := range binPaths {
			if a.exec.bin == path {
				allowed = true
				break
			}
		}
		if !allowed {
			t.Errorf("allowlist dışı binary: %s", a.exec.bin)
		}
	}
}

func TestUserCreatePlan(t *testing.T) {
	cfg := testCfg()
	fn := newRegistry(cfg)["user.create"]
	raw, _ := json.Marshal(map[string]any{
		"name": "www-site001",
		"home": "/srv/aurapanel/sites/site001/home",
	})
	p, data, err := fn(cfg, raw)
	if err != nil {
		t.Fatalf("user.create: %v", err)
	}
	assertPlanBins(t, p)
	if len(p.actions) != 3 {
		t.Fatalf("3 eylem bekleniyordu (mkdir+useradd+chown), %d geldi", len(p.actions))
	}
	if p.actions[0].kind != actMkdir || p.actions[0].mkdir != "/srv/aurapanel/sites/site001/home" {
		t.Fatal("ilk eylem mkdir(home) olmalı")
	}
	ua := p.actions[1].exec
	if ua.bin != binPaths["useradd"] {
		t.Fatalf("yanlış binary: %s", ua.bin)
	}
	want := []string{"-U", "-d", "/srv/aurapanel/sites/site001/home", "-s", "/usr/sbin/nologin", "www-site001"}
	if strings.Join(ua.args, "|") != strings.Join(want, "|") {
		t.Fatalf("useradd argümanları: %v (beklenen %v)", ua.args, want)
	}
	if data.(map[string]any)["name"] != "www-site001" {
		t.Fatal("data eksik")
	}
}

func TestUserCreateRejects(t *testing.T) {
	cfg := testCfg()
	fn := newRegistry(cfg)["user.create"]
	bad := []map[string]any{
		{"name": "BAD", "home": "/srv/aurapanel/sites/site001/home"},                       // büyük harf
		{"name": "../evil", "home": "/srv/aurapanel/sites/site001/home"},                  // traversal
		{"name": "www-site001", "home": "/etc/passwd"},                                    // home kaçışı
		{"name": "www-site001", "home": "/srv/aurapanel/sites/site002/home"},              // başka site
		{"name": "www-site001", "home": "/srv/aurapanel/sites/site001/home", "shell": "/bin/bash"}, // shell yasak
		{"name": "www-site001", "home": "/srv/aurapanel/sites/site001/home", "extra": 1}, // bilinmeyen alan
	}
	for i, args := range bad {
		raw, _ := json.Marshal(args)
		if _, _, err := fn(cfg, raw); err == nil {
			t.Errorf("durum %d reddedilmedi: %v", i, args)
		}
	}
}

func TestUserExistsNoExec(t *testing.T) {
	cfg := testCfg()
	fn := newRegistry(cfg)["user.exists"]
	raw, _ := json.Marshal(map[string]any{"name": "kesinlikle-yok-xyz123"})
	p, data, err := fn(cfg, raw)
	if err != nil {
		t.Fatalf("user.exists: %v", err)
	}
	if len(p.actions) != 0 {
		t.Fatal("user.exists exec üretmemeli")
	}
	if data.(map[string]any)["exists"] != false {
		t.Fatal("olmayan kullanıcı exists=false olmalı")
	}
}

func TestCgroupLimitsPlan(t *testing.T) {
	cfg := testCfg()
	fn := newRegistry(cfg)["cgroup.limits"]
	raw, _ := json.Marshal(map[string]any{
		"site": "site001", "cpu_max": "50000", "memory_max": 268435456, "memory_high": 134217728, "pids_max": 512,
	})
	p, _, err := fn(cfg, raw)
	if err != nil {
		t.Fatalf("cgroup.limits: %v", err)
	}
	dir := path.Join(cfg.cgroupBase, "sites", "site001")
	if p.actions[0].kind != actMkdir || p.actions[0].mkdir != dir {
		t.Fatal("ilk eylem mkdir(site cgroup) olmalı")
	}
	got := map[string]string{}
	for _, a := range p.actions[1:] {
		if a.kind != actWrite {
			t.Fatalf("yazma dışı eylem: %d", a.kind)
		}
		got[a.write.path] = a.write.content
	}
	if got[path.Join(dir, "cpu.max")] != "50000 100000" {
		t.Errorf("cpu.max: %q", got[path.Join(dir, "cpu.max")])
	}
	if got[path.Join(dir, "memory.max")] != "268435456" {
		t.Errorf("memory.max: %q", got[path.Join(dir, "memory.max")])
	}
	if got[path.Join(dir, "pids.max")] != "512" {
		t.Errorf("pids.max: %q", got[path.Join(dir, "pids.max")])
	}
}

func TestCgroupLimitsRejects(t *testing.T) {
	cfg := testCfg()
	fn := newRegistry(cfg)["cgroup.limits"]
	bad := []map[string]any{
		{"site": "../x"},
		{"site": "site001", "cpu_max": "rm -rf"},
		{"site": "site001", "memory_max": 1024},                       // 8 MiB altı
		{"site": "site001", "memory_max": 1 << 41},                    // 1 TiB üstü
		{"site": "site001", "memory_max": 100, "memory_high": 200},    // high > max
		{"site": "site001", "pids_max": 1},                            // 16 altı
		{"site": "site001", "memory_max": -5},                         // negatif
	}
	for i, args := range bad {
		raw, _ := json.Marshal(args)
		if _, _, err := fn(cfg, raw); err == nil {
			t.Errorf("durum %d reddedilmedi: %v", i, args)
		}
	}
}

func TestQuotaSetPlan(t *testing.T) {
	cfg := testCfg()
	fn := newRegistry(cfg)["quota.set"]
	raw, _ := json.Marshal(map[string]any{"user": "www-site001", "disk_mb": 5120, "inodes": 200000})
	p, _, err := fn(cfg, raw)
	if err != nil {
		t.Fatalf("quota.set: %v", err)
	}
	assertPlanBins(t, p)
	ex := p.actions[0].exec
	if ex.bin != binPaths["setquota"] {
		t.Fatalf("yanlış binary: %s", ex.bin)
	}
	// ext4 blokları 1 KiB: 5120 MiB = 5242880 blok.
	want := []string{"-u", "www-site001", "5242880", "5242880", "200000", "200000", "/"}
	if strings.Join(ex.args, "|") != strings.Join(want, "|") {
		t.Fatalf("setquota argümanları: %v (beklenen %v)", ex.args, want)
	}
}

func TestFirewallApplyRejects(t *testing.T) {
	cfg := testCfg()
	fn := newRegistry(cfg)["firewall.apply"]
	bad := []string{"../rules.nft", "/etc/passwd", "rules.nft", "/etc/aurapanel/nftables/../../shadow"}
	for i, r := range bad {
		raw, _ := json.Marshal(map[string]any{"ruleset": r})
		if _, _, err := fn(cfg, raw); err == nil {
			t.Errorf("durum %d reddedilmedi: %s", i, r)
		}
	}
	ok, _ := json.Marshal(map[string]any{"ruleset": "/etc/aurapanel/nftables/rules.nft"})
	if _, _, err := fn(cfg, ok); err != nil {
		t.Errorf("geçerli ruleset reddedildi: %v", err)
	}
}

func TestSshdInstallOrder(t *testing.T) {
	cfg := testCfg()
	fn := newRegistry(cfg)["sshd.install_config"]
	raw, _ := json.Marshal(map[string]any{"config": "/var/lib/aurapanel/stage/sshd-sites.conf"})
	p, _, err := fn(cfg, raw)
	if err != nil {
		t.Fatalf("sshd.install_config: %v", err)
	}
	assertPlanBins(t, p)
	// Sıra kritik: sshd -t (doğrula) → copy → systemctl reload.
	if p.actions[0].kind != actExec || p.actions[0].exec.bin != binPaths["sshd"] {
		t.Fatal("ilk eylem sshd -t doğrulaması olmalı")
	}
	if p.actions[1].kind != actCopy || p.actions[1].copy.dst != "/etc/ssh/sshd_config.d/aurapanel-sites.conf" {
		t.Fatal("ikinci eylem hedefe kopyalama olmalı")
	}
	if p.actions[2].kind != actExec || p.actions[2].exec.bin != binPaths["systemctl"] {
		t.Fatal("üçüncü eylem systemctl reload olmalı")
	}
	// Stage dışı kaynak reddedilmeli.
	bad, _ := json.Marshal(map[string]any{"config": "/etc/ssh/sshd_config"})
	if _, _, err := fn(cfg, bad); err == nil {
		t.Fatal("stage dışı config kabul edildi")
	}
}

// OLS bundle doğrulaması: site/dosya adı/NUL/oversize/tekrar/mode redleri.
func TestOlsBundleValidation(t *testing.T) {
	cfg := testCfg()
	fn := newRegistry(cfg)["ols.install_bundle"]
	ok := map[string]any{"name": "vhconf.conf", "content": "# vhost"}
	bad := []map[string]any{
		{"site": "../x", "files": []map[string]any{ok}},
		{"site": "site001", "files": []map[string]any{}},
		{"site": "site001", "files": []map[string]any{{"name": "evil.sh", "content": "# x"}}},
		{"site": "site001", "files": []map[string]any{{"name": "vhconf.conf", "content": "# a\x00b"}}},
		{"site": "site001", "files": []map[string]any{{"name": "vhconf.conf", "content": strings.Repeat("#", olsBundleContentLimit+1)}}},
		{"site": "site001", "files": []map[string]any{ok, {"name": "vhconf.conf", "content": "# dup"}}},
		{"site": "site001", "files": []map[string]any{{"name": "vhconf.conf", "content": "# x", "mode": 0o777}}},
	}
	for i, args := range bad {
		raw, _ := json.Marshal(args)
		if _, _, err := fn(cfg, raw); err == nil {
			t.Errorf("ols bundle durumu %d reddedilmedi", i)
		}
	}
	// Geçerli bundle: write eylemleri doğru hedefe, mode varsayılan 0644.
	raw, _ := json.Marshal(map[string]any{"site": "site001", "files": []map[string]any{ok}})
	p, _, err := fn(cfg, raw)
	if err != nil {
		t.Fatalf("geçerli bundle reddedildi: %v", err)
	}
	if p.actions[0].kind != actMkdir {
		t.Fatal("ilk eylem mkdir olmalı")
	}
	w := p.actions[1].write
	if w.path != "/usr/local/lsws/conf/vhosts/site001/vhconf.conf" || w.mode != 0o644 {
		t.Fatalf("write hedefi/mode hatalı: %s %v", w.path, w.mode)
	}
}

// Her op'un mutlu-yol planı bin allowlist'ine uygun olmalı.
func TestAllOpsHappyPathBins(t *testing.T) {
	cfg := testCfg()
	reg := newRegistry(cfg)
	happy := map[string]map[string]any{
		"priv.ping":                 {},
		"user.create":               {"name": "www-site001", "home": "/srv/aurapanel/sites/site001/home"},
		"user.delete":               {"name": "www-site001"},
		"user.exists":               {"name": "www-site001"},
		"cgroup.bootstrap":          {},
		"cgroup.limits":             {"site": "site001", "cpu_max": "max"},
		"quota.set":                 {"user": "www-site001", "disk_mb": 1024},
		"firewall.apply":            {"ruleset": "/etc/aurapanel/nftables/rules.nft"},
		"sshd.install_config":       {"config": "/var/lib/aurapanel/stage/sshd-sites.conf"},
		"logrotate.install_config":  {"config": "/var/lib/aurapanel/stage/logrotate-sites.conf"},
		"ols.test":                  {},
		"ols.read_bundle":           {"site": "site001"},
		"ols.install_bundle":        {"site": "site001", "files": []map[string]any{{"name": "vhconf.conf", "content": "# test"}}},
		"ols.remove_bundle":         {"site": "site001", "names": []string{"vhconf.conf"}},
		"ols.reload":                {},
	}
	for op, args := range happy {
		raw, _ := json.Marshal(args)
		p, _, err := reg[op](cfg, raw)
		if err != nil {
			t.Errorf("%s mutlu yol: %v", op, err)
			continue
		}
		assertPlanBins(t, p)
	}
}
