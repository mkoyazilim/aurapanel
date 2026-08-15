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
		"site.prepare", "site.teardown", "cgroup.cleanup",
		"cgroup.read", "site.status", "quota.get",
		"php.detect", "php.install_ini", "php.read_ini",
		"ols.webadmin_credentials",
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
	raw, _ := json.Marshal(map[string]any{"content": "test content"})
	p, _, err := fn(cfg, raw)
	if err != nil {
		t.Fatalf("sshd.install_config: %v", err)
	}
	assertPlanBins(t, p)
	// Sıra kritik: mkdir -> write -> sshd -t (doğrula) → copy → systemctl reload.
	if len(p.actions) < 5 {
		t.Fatalf("yetersiz action")
	}
	if p.actions[2].kind != actExec || p.actions[2].exec.bin != binPaths["sshd"] {
		t.Fatal("üçüncü eylem sshd -t doğrulaması olmalı")
	}
	if p.actions[3].kind != actCopy || p.actions[3].copy.dst != "/etc/ssh/sshd_config.d/aurapanel-sites.conf" {
		t.Fatal("dördüncü eylem hedefe kopyalama olmalı")
	}
	if p.actions[4].kind != actExec || p.actions[4].exec.bin != binPaths["systemctl"] {
		t.Fatal("beşinci eylem systemctl reload olmalı")
	}
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

// site.prepare/teardown değişmezi: user www-<site> olmalı; teardown
// RemoveAll'ı yalnızca TAM site köküne uygular.
func TestSitePrepareTeardownValidation(t *testing.T) {
	cfg := testCfg()
	prep := newRegistry(cfg)["site.prepare"]
	tear := newRegistry(cfg)["site.teardown"]

	if _, _, err := prep(cfg, mustJSON(t, map[string]any{"site": "site001", "user": "www-site002"})); err == nil {
		t.Error("user değişmezi ihlali kabul edildi")
	}
	if _, _, err := prep(cfg, mustJSON(t, map[string]any{"site": "../x", "user": "www-../x"})); err == nil {
		t.Error("geçersiz site kabul edildi")
	}
	p, _, err := prep(cfg, mustJSON(t, map[string]any{"site": "site001", "user": "www-site001"}))
	if err != nil {
		t.Fatalf("geçerli prepare reddedildi: %v", err)
	}
	if p.actions[0].kind != actMkdir || p.actions[0].mkdir != "/srv/aurapanel/sites/site001/logs" {
		t.Fatal("ilk eylem mkdir(logs) olmalı")
	}
	if p.actions[1].mkdirMode != 0o700 {
		t.Errorf("tmp 0700 olmalı, %o geldi", p.actions[1].mkdirMode)
	}

	pt, _, err := tear(cfg, mustJSON(t, map[string]any{"site": "site001", "user": "www-site001"}))
	if err != nil {
		t.Fatalf("geçerli teardown reddedildi: %v", err)
	}
	if pt.actions[0].kind != actRemoveAll || pt.actions[0].removeAll != "/srv/aurapanel/sites/site001" {
		t.Fatalf("teardown TAM site kökünü hedeflemeli: %+v", pt.actions[0])
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// Okuma op'ları (drift): eksik kaynaklar sessizce boş döner, exec üretmez.
func TestReadOps(t *testing.T) {
	cfg := testCfg()
	reg := newRegistry(cfg)

	// cgroup.read: Windows'ta /sys/fs/cgroup yok → values boş; exec yok.
	p, data, err := reg["cgroup.read"](cfg, mustJSON(t, map[string]any{"site": "site001"}))
	if err != nil {
		t.Fatalf("cgroup.read: %v", err)
	}
	if len(p.actions) != 0 {
		t.Fatal("cgroup.read exec üretmemeli")
	}
	if len(data.(map[string]any)["values"].(map[string]any)) != 0 {
		t.Fatalf("eksik dosyalarda values boş olmalı: %v", data)
	}

	// site.status: dizinler yok → exists:false.
	_, data, err = reg["site.status"](cfg, mustJSON(t, map[string]any{"site": "site001", "user": "www-site001"}))
	if err != nil {
		t.Fatalf("site.status: %v", err)
	}
	dirs := data.(map[string]any)["dirs"].(map[string]any)
	if dirs["home"].(map[string]any)["exists"] != false {
		t.Fatalf("eksik home exists:true dönmemeli: %v", dirs)
	}

	// quota.get: Windows'ta readQuota stub hata döner → available:false (hata DEĞİL).
	_, data, err = reg["quota.get"](cfg, mustJSON(t, map[string]any{"user": "www-site001"}))
	if err != nil {
		t.Fatalf("quota.get: %v", err)
	}
	if data.(map[string]any)["available"] != false {
		t.Fatalf("quota kullanılamaz durumda available:false olmalı: %v", data)
	}

	// site.status user değişmezi.
	if _, _, err := reg["site.status"](cfg, mustJSON(t, map[string]any{"site": "site001", "user": "www-site999"})); err == nil {
		t.Fatal("site.status user değişmezi ihlali kabul edildi")
	}
}

// php.install_ini: satır biçimi sıkı doğrulamalı; şüpheli içerik reddedilmeli.
func TestPHPIniValidation(t *testing.T) {
	cfg := testCfg()
	fn := newRegistry(cfg)["php.install_ini"]

	good := "# yorum\nmemory_limit = 256M\nsession.gc_maxlifetime = 1440\n"
	p, _, err := fn(cfg, mustJSON(t, map[string]any{"site": "site001", "content": good}))
	if err != nil {
		t.Fatalf("geçerli ini reddedildi: %v", err)
	}
	if p.actions[0].kind != actMkdir || p.actions[0].mkdir != "/srv/aurapanel/sites/site001/conf" {
		t.Fatal("ilk eylem mkdir(conf) olmalı")
	}
	if p.actions[1].write.path != "/srv/aurapanel/sites/site001/conf/php.ini" {
		t.Fatalf("hedef yanlış: %s", p.actions[1].write.path)
	}

	bad := []string{
		"rm -rf /tmp\n",
		"system('id')\n",
		"memory_limit = 256M; rm -rf /\n",
		"memory_limit=256M\n\x00",
		"key\n",
		strings.Repeat("#", phpIniLimit+1),
	}
	for i, c := range bad {
		if _, _, err := fn(cfg, mustJSON(t, map[string]any{"site": "site001", "content": c})); err == nil {
			t.Errorf("şüpheli ini içeriği %d kabul edildi: %q", i, c)
		}
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
		"sshd.install_config":       {"content": "test content"},
		"logrotate.install_config":  {"config": "/var/lib/aurapanel/stage/logrotate-sites.conf"},
		"ols.test":                  {},
		"ols.read_bundle":           {"site": "site001"},
		"ols.install_bundle":        {"site": "site001", "files": []map[string]any{{"name": "vhconf.conf", "content": "# test"}}},
		"ols.remove_bundle":         {"site": "site001", "names": []string{"vhconf.conf"}},
		"ols.reload":                {},
		"site.prepare":              {"site": "site001", "user": "www-site001"},
		"site.teardown":             {"site": "site001", "user": "www-site001"},
		"cgroup.cleanup":            {"site": "site001"},
		"cgroup.read":               {"site": "site001"},
		"site.status":               {"site": "site001", "user": "www-site001"},
		"quota.get":                 {"user": "www-site001"},
		"php.detect":                {},
		"php.install_ini":           {"site": "site001", "content": "memory_limit = 256M\n"},
		"php.read_ini":              {"site": "site001"},
		"ols.webadmin_credentials":  {"username": "admin-abc", "password": "güçlü-parola-123"},
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
