// ops.go: op kayıt defteri ve doğrulama/planlama mantığı.
// ÇAPRAZ PLATFORM: bu dosyada hiçbir komut yürütülmez, yalnızca plan üretilir.
// Yürütme exec_linux.go'dadır ve yalnızca Linux'ta derlenir.
package priv

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// NOT: Yol doğrulaması bilinçli olarak `path` (POSIX) paketiyle yapılır —
// helper yalnızca Linux'ta yürütülür ve Linux yol semantiği platformdan
// bağımsız doğrulanmalıdır. Gerçek dosya işlemleri (exec_linux.go)
// `filepath` kullanır ve zaten yalnızca Linux'ta derlenir.

// runtimeCfg, op doğrulamasında kullanılan sabitler (flag'lerden doldurulur).
type runtimeCfg struct {
	panelUser  string
	panelUID   uint32
	panelGID   uint32
	quotaFS    string
	opTimeout  time.Duration
	sitesRoot  string
	stageDir   string
	nftDir     string
	cgroupBase string
}

// binPaths, yürütülebilecek TEK binary kümesi (mutlak yollar).
// Bu harita dışında hiçbir binary çalıştırılamaz — fuzz testi bunu kanıtlar.
var binPaths = map[string]string{
	"useradd":   "/usr/sbin/useradd",
	"userdel":   "/usr/sbin/userdel",
	"chown":     "/usr/bin/chown",
	"nft":       "/usr/sbin/nft",
	"sshd":      "/usr/sbin/sshd",
	"logrotate": "/usr/sbin/logrotate",
	"systemctl": "/usr/bin/systemctl",
	"setquota":  "/usr/sbin/setquota",
	"rsync":     "/usr/bin/rsync",
	"postmap":   "/usr/sbin/postmap",
	"postconf":  "/usr/sbin/postconf",
	"dovecot":   "/usr/sbin/dovecot",
	"opendkim":  "/usr/sbin/opendkim",
	"lswsctrl":  "/usr/local/lsws/bin/lswsctrl",
	"lshttpd":   "/usr/local/lsws/bin/lshttpd",
}

func bin(name string) (string, error) {
	p, ok := binPaths[name]
	if !ok {
		return "", fmt.Errorf("izin verilmeyen binary: %q", name)
	}
	return p, nil
}

// Doğrulama kalıpları.
var (
	reUserName = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)
	reSiteID   = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}$`)
	reFileName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,63}$`)
	reCPUValue = regexp.MustCompile(`^[0-9]{1,16}$`)
)

// allowedShells: site kullanıcılarına shell verilmez (İlke/§11.2).
var allowedShells = map[string]struct{}{
	"/usr/sbin/nologin": {},
	"/bin/false":        {},
}

// Sınır sabitleri (titiz aralık kontrolleri).
const (
	minMemoryBytes = 8 << 20       // 8 MiB
	maxMemoryBytes = 1 << 40       // 1 TiB
	minPIDs        = 16
	maxPIDs        = 1 << 22       // ~4.2M
	minDiskMB      = 1
	maxDiskMB      = 10_000_000    // ~10 TiB
	minInodes      = 100
	maxInodes      = 1_000_000_000
)

// cleanUnder, path'in base altında kaldığını doğrular ve temizlenmiş hâlini döndürür.
func cleanUnder(base, p string) (string, error) {
	if !path.IsAbs(p) {
		return "", errors.New("mutlak yol gerekli")
	}
	clean := path.Clean(p)
	baseClean := path.Clean(base)
	if clean == baseClean || strings.HasPrefix(clean, baseClean+"/") {
		return clean, nil
	}
	return "", errors.New("izin verilen dizin dışına çıkış")
}

// strictDecode, bilinmeyen alanları ve artık veriyi reddeden JSON çözümleme.
func strictDecode(raw json.RawMessage, v any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if dec.More() {
		return errors.New("args'ta artık veri var")
	}
	return nil
}

// Plan modeli: yürütülecek eylemlerin sıralı listesi.
type actionKind int

const (
	actMkdir actionKind = iota
	actWrite
	actCopy
	actRemove
	actRemoveAll
	actExec
)

type action struct {
	kind      actionKind
	mkdir     string
	mkdirMode os.FileMode
	write     fileWrite
	copy      fileCopy
	remove    string
	removeAll string
	exec      execSpec
}

type fileWrite struct {
	path    string
	content string
	mode    os.FileMode
}

type fileCopy struct {
	src  string
	dst  string
	mode os.FileMode
}

type execSpec struct {
	bin   string
	args  []string
	stdin string // stdin'e yazılacak metin
	// tolerateWarn: çıkış kodu != 0 ama çıktıda "[ERROR]" YOKSA başarılı
	// say (lshttpd -t yalnızca WARN içeren sağlıklı config'de 1 döner).
	tolerateWarn bool
}

type plan struct {
	actions []action
}

func (p *plan) mkdir(path string)      { p.mkdirMode(path, 0o755) }
func (p *plan) mkdirMode(path string, mode os.FileMode) {
	p.actions = append(p.actions, action{kind: actMkdir, mkdir: path, mkdirMode: mode})
}
func (p *plan) write(f fileWrite)      { p.actions = append(p.actions, action{kind: actWrite, write: f}) }
func (p *plan) copy(c fileCopy)        { p.actions = append(p.actions, action{kind: actCopy, copy: c}) }
func (p *plan) remove(path string)     { p.actions = append(p.actions, action{kind: actRemove, remove: path}) }
func (p *plan) removeAll(path string)  { p.actions = append(p.actions, action{kind: actRemoveAll, removeAll: path}) }
func (p *plan) exec(bin string, args ...string) {
	p.actions = append(p.actions, action{kind: actExec, exec: execSpec{bin: bin, args: args}})
}

// opFunc, argümanları doğrular ve bir plan üretir; asla yürütmez.
type opFunc func(cfg *runtimeCfg, args json.RawMessage) (*plan, any, error)

// newRegistry, allowlist'teki op kümesini döndürür (ARCHITECTURE §3.2).
// Bu liste dışında hiçbir op YOKTUR.
func newRegistry(cfg *runtimeCfg) map[string]opFunc {
	return map[string]opFunc{
		"priv.ping":                 opPing,
		"user.create":               opUserCreate,
		"user.delete":               opUserDelete,
		"user.exists":               opUserExists,
		"cgroup.bootstrap":          opCgroupBootstrap,
		"server.services":           opServerServices,
		"server.action":             opServerAction,
		"cgroup.limits":             opCgroupLimits,
		"quota.set":                 opQuotaSet,
		"firewall.apply":            opFirewallApply,
		"sshd.install_config":       opSshdInstall,
		"logrotate.install_config":  opLogrotateInstall,
		"ols.test":                  opOlsTest,
		"ols.read_bundle":           opOlsReadBundle,
		"ols.install_bundle":        opOlsInstallBundle,
		"ols.remove_bundle":         opOlsRemoveBundle,
		"ols.reload":                opOlsReload,
		"site.prepare":              opSitePrepare,
		"site.teardown":             opSiteTeardown,
		"cgroup.cleanup":            opCgroupCleanup,
		"cgroup.read":               opCgroupRead,
		"site.status":               opSiteStatus,
		"quota.get":                 opQuotaGet,
		"php.detect":                opPHPDetect,
		"php.install_ini":           opPHPInstallIni,
		"php.read_ini":              opPHPReadIni,
		"ols.webadmin_credentials":  opOlsWebadminCredentials,
		"cron.apply":                opCronApply,
		"node.apply":                opNodeApply,
		"node.remove":               opNodeRemove,
		"site.clone_files":          opSiteCloneFiles,
		"firewall.list":             opFirewallList,
		"firewall.rule_add":         opFirewallRuleAdd,
		"firewall.rule_delete":      opFirewallRuleDelete,
		"firewall.ssh_port":         opSSHPortChange,
		"firewall.panel_port":       opPanelPortChange,
		"mail.setup":                opMailSetup,
		"mail.provision":            opMailProvision,
		"mail.dkim_generate":        opMailDKIMGenerate,
	}
}

// opNodeApply creates or updates a systemd service for a Node.js app
func opNodeApply(cfg *runtimeCfg, raw json.RawMessage) (*plan, any, error) {
	var a struct {
		Site    string `json:"site"`
		AppID   string `json:"app_id"`
		Content string `json:"content"`
	}
	if err := strictDecode(raw, &a); err != nil {
		return nil, nil, fmt.Errorf("node.apply: %w", err)
	}
	if !reSiteID.MatchString(a.Site) {
		return nil, nil, errors.New("node.apply: site kimliği geçersiz")
	}
	if !reSiteID.MatchString(a.AppID) {
		return nil, nil, errors.New("node.apply: app kimliği geçersiz")
	}
	
	serviceName := fmt.Sprintf("ap-node-%s-%s.service", a.Site, a.AppID)
	servicePath := "/etc/systemd/system/" + serviceName
	
	systemctl, _ := bin("systemctl")
	p := &plan{}
	p.write(fileWrite{path: servicePath, content: a.Content, mode: 0o644})
	p.exec(systemctl, "daemon-reload")
	p.exec(systemctl, "enable", "--now", serviceName)
	p.exec(systemctl, "restart", serviceName)
	return p, map[string]any{"service": serviceName}, nil
}

// opNodeRemove stops and removes a systemd service for a Node.js app
func opNodeRemove(cfg *runtimeCfg, raw json.RawMessage) (*plan, any, error) {
	var a struct {
		Site  string `json:"site"`
		AppID string `json:"app_id"`
	}
	if err := strictDecode(raw, &a); err != nil {
		return nil, nil, fmt.Errorf("node.remove: %w", err)
	}
	if !reSiteID.MatchString(a.Site) {
		return nil, nil, errors.New("node.remove: site kimliği geçersiz")
	}
	if !reSiteID.MatchString(a.AppID) {
		return nil, nil, errors.New("node.remove: app kimliği geçersiz")
	}
	
	serviceName := fmt.Sprintf("ap-node-%s-%s.service", a.Site, a.AppID)
	servicePath := "/etc/systemd/system/" + serviceName
	
	systemctl, _ := bin("systemctl")
	p := &plan{}
	p.exec(systemctl, "stop", serviceName)
	p.exec(systemctl, "disable", serviceName)
	p.remove(servicePath)
	p.exec(systemctl, "daemon-reload")
	return p, map[string]any{"service": serviceName}, nil
}

// opCronApply, site kullanıcısının crontab dosyasını güvenli biçimde yazar.
// İçerik panel tarafından hazırlanır (job'lar DB'den okunur); priv yalnızca
// doğrulama + yazma işlemi yapar.
func opCronApply(cfg *runtimeCfg, raw json.RawMessage) (*plan, any, error) {
	var a struct {
		Site    string `json:"site"`
		Content string `json:"content"`
	}
	if err := strictDecode(raw, &a); err != nil {
		return nil, nil, fmt.Errorf("cron.apply: %w", err)
	}
	if !reSiteID.MatchString(a.Site) {
		return nil, nil, errors.New("cron.apply: site kimliği geçersiz")
	}
	if len(a.Content) > 64<<10 {
		return nil, nil, errors.New("cron.apply: içerik 64 KiB sınırını aşıyor")
	}
	user := "www-" + a.Site
	crontabPath := "/var/spool/cron/crontabs/" + user
	p := &plan{}
	p.write(fileWrite{path: crontabPath, content: a.Content, mode: 0o600})
	return p, map[string]any{"site": a.Site, "user": user}, nil
}

// opOlsWebadminCredentials, OLS WebAdmin kimlik bilgilerini senkronlar
// (ARCHITECTURE §9.10: panel admin şifresiyle tek giriş çifti).
// apr1-MD5 hash'i SÜREÇ İÇİNDE hesaplanır ve htpasswd dosyasına yazılır —
// dış htpasswd binary'si YOKTUR (OLS yalnızca htpasswd.php taşır), parola
// hiçbir komut satırında görünmez.
func opOlsWebadminCredentials(cfg *runtimeCfg, raw json.RawMessage) (*plan, any, error) {
	var a struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := strictDecode(raw, &a); err != nil {
		return nil, nil, fmt.Errorf("ols.webadmin_credentials: %w", err)
	}
	if !reUserName.MatchString(a.Username) {
		return nil, nil, errors.New("ols.webadmin_credentials: kullanıcı adı geçersiz")
	}
	if len(a.Password) < 12 || len(a.Password) > 128 {
		return nil, nil, errors.New("ols.webadmin_credentials: parola 12..128 karakter olmalı")
	}
	line := a.Username + ":" + apr1Crypt(a.Password, apr1Salt()) + "\n"
	p := &plan{}
	p.write(fileWrite{path: "/usr/local/lsws/admin/conf/htpasswd", content: line, mode: 0o600})
	return p, map[string]any{"username": a.Username}, nil
}

func opPing(cfg *runtimeCfg, raw json.RawMessage) (*plan, any, error) {
	var a struct{}
	if err := strictDecode(raw, &a); err != nil {
		return nil, nil, fmt.Errorf("priv.ping: %w", err)
	}
	return &plan{}, map[string]any{"version": Version, "goos": runtime.GOOS}, nil
}

func opUserCreate(cfg *runtimeCfg, raw json.RawMessage) (*plan, any, error) {
	var a struct {
		Name  string `json:"name"`
		Home  string `json:"home"`
		Shell string `json:"shell"`
	}
	if err := strictDecode(raw, &a); err != nil {
		return nil, nil, fmt.Errorf("user.create: %w", err)
	}
	if !reUserName.MatchString(a.Name) {
		return nil, nil, errors.New("user.create: name geçersiz (^[a-z][a-z0-9_-]{0,31}$)")
	}
	// Değişmez: kullanıcı adı "www-<siteID>" biçimindedir ve home dizini
	// TAM olarak kendi site dizininde olmalıdır — panel hatasıyla bile
	// kullanıcı başka bir sitenin alanına yazılamaz.
	siteID := strings.TrimPrefix(a.Name, "www-")
	if siteID == a.Name || !reSiteID.MatchString(siteID) {
		return nil, nil, errors.New("user.create: name 'www-<siteID>' biçiminde olmalı")
	}
	home, err := cleanUnder(cfg.sitesRoot, a.Home)
	if err != nil {
		return nil, nil, fmt.Errorf("user.create: home: %w", err)
	}
	wantDir := path.Join(cfg.sitesRoot, siteID)
	if path.Dir(home) != wantDir {
		return nil, nil, fmt.Errorf("user.create: home %s altında olmalı", wantDir)
	}
	if path.Base(home) != "home" {
		return nil, nil, errors.New("user.create: home, site kökündeki 'home' dizini olmalı")
	}
	if a.Shell == "" {
		a.Shell = "/usr/sbin/nologin"
	}
	if _, ok := allowedShells[a.Shell]; !ok {
		return nil, nil, fmt.Errorf("user.create: shell izinli değil: %q", a.Shell)
	}

	useradd, _ := bin("useradd")
	chown, _ := bin("chown")
	p := &plan{}
	p.mkdirMode(home, 0o750)
	p.exec(useradd, "-U", "-d", home, "-s", a.Shell, a.Name)
	p.exec(chown, "-R", a.Name+":"+a.Name, home)

	return p, map[string]any{"name": a.Name, "home": home}, nil
}

func opUserDelete(cfg *runtimeCfg, raw json.RawMessage) (*plan, any, error) {
	var a struct {
		Name string `json:"name"`
	}
	if err := strictDecode(raw, &a); err != nil {
		return nil, nil, fmt.Errorf("user.delete: %w", err)
	}
	if !reUserName.MatchString(a.Name) {
		return nil, nil, errors.New("user.delete: name geçersiz")
	}
	userdel, _ := bin("userdel")
	p := &plan{}
	p.exec(userdel, a.Name)
	return p, map[string]any{"name": a.Name}, nil
}

// opUserExists süreç içi os/user.Lookup kullanır — exec yok.
func opUserExists(cfg *runtimeCfg, raw json.RawMessage) (*plan, any, error) {
	var a struct {
		Name string `json:"name"`
	}
	if err := strictDecode(raw, &a); err != nil {
		return nil, nil, fmt.Errorf("user.exists: %w", err)
	}
	if !reUserName.MatchString(a.Name) {
		return nil, nil, errors.New("user.exists: name geçersiz")
	}
	_, err := user.Lookup(a.Name)
	exists := err == nil
	if err != nil {
		var unknown user.UnknownUserError
		switch {
		case errors.As(err, &unknown):
			exists = false // Linux: kullanıcı gerçekten yok
		case runtime.GOOS != "linux":
			exists = false // Windows geliştirme ortamı: hata tipi farklıdır
		default:
			return nil, nil, fmt.Errorf("user.exists: %w", err)
		}
	}
	return &plan{}, map[string]any{"exists": exists}, nil
}

func opCgroupBootstrap(cfg *runtimeCfg, raw json.RawMessage) (*plan, any, error) {
	var a struct{}
	if err := strictDecode(raw, &a); err != nil {
		return nil, nil, fmt.Errorf("cgroup.bootstrap: %w", err)
	}
	base, err := cleanUnder("/sys/fs/cgroup", cfg.cgroupBase)
	if err != nil {
		return nil, nil, fmt.Errorf("cgroup.bootstrap: %w", err)
	}
	sites := path.Join(base, "sites")
	chown, _ := bin("chown")
	p := &plan{}
	p.mkdir(base)
	// Enable controllers in root before creating child (sites)
	p.write(fileWrite{path: path.Join(base, "cgroup.subtree_control"), content: "+cpu +memory +pids\n", mode: 0o644})
	
	p.mkdir(sites)
	// Enable controllers in sites so that site001, site002 etc. get them
	p.write(fileWrite{path: path.Join(sites, "cgroup.subtree_control"), content: "+cpu +memory +pids\n", mode: 0o644})
	
	// Delegation: alt ağacın sahipliğini panel kullanıcısına devret
	// (panel, site cgroup'larını root olmadan yönetebilsin — §3).
	p.exec(chown, "-R", cfg.panelUser+":"+cfg.panelUser, base)
	return p, map[string]any{"base": base}, nil
}

func opCgroupLimits(cfg *runtimeCfg, raw json.RawMessage) (*plan, any, error) {
	var a struct {
		Site       string `json:"site"`
		CPU        string `json:"cpu_max"`
		MemoryMax  uint64 `json:"memory_max"`
		MemoryHigh uint64 `json:"memory_high"`
		PIDsMax    uint64 `json:"pids_max"`
	}
	if err := strictDecode(raw, &a); err != nil {
		return nil, nil, fmt.Errorf("cgroup.limits: %w", err)
	}
	if !reSiteID.MatchString(a.Site) {
		return nil, nil, errors.New("cgroup.limits: site kimliği geçersiz")
	}
	if a.CPU != "" && a.CPU != "max" && !reCPUValue.MatchString(a.CPU) {
		return nil, nil, errors.New("cgroup.limits: cpu_max 'max' veya sayı olmalı")
	}
	if a.MemoryMax != 0 && (a.MemoryMax < minMemoryBytes || a.MemoryMax > maxMemoryBytes) {
		return nil, nil, fmt.Errorf("cgroup.limits: memory_max aralık dışı (%d..%d)", minMemoryBytes, maxMemoryBytes)
	}
	if a.MemoryHigh != 0 && (a.MemoryHigh < minMemoryBytes || a.MemoryHigh > maxMemoryBytes) {
		return nil, nil, fmt.Errorf("cgroup.limits: memory_high aralık dışı")
	}
	if a.MemoryMax != 0 && a.MemoryHigh != 0 && a.MemoryHigh > a.MemoryMax {
		return nil, nil, errors.New("cgroup.limits: memory_high, memory_max'tan büyük olamaz")
	}
	if a.PIDsMax != 0 && (a.PIDsMax < minPIDs || a.PIDsMax > maxPIDs) {
		return nil, nil, fmt.Errorf("cgroup.limits: pids_max aralık dışı (%d..%d)", minPIDs, maxPIDs)
	}

	dir := path.Join(cfg.cgroupBase, "sites", a.Site)
	p := &plan{}
	p.mkdir(dir)
	applied := map[string]any{}
	if a.CPU != "" {
		v := a.CPU
		if v != "max" {
			v += " 100000" // cgroup v2 cpu.max: "quota period"
		}
		p.write(fileWrite{path: path.Join(dir, "cpu.max"), content: v, mode: 0o644})
		applied["cpu_max"] = a.CPU
	}
	if a.MemoryMax != 0 {
		p.write(fileWrite{path: path.Join(dir, "memory.max"), content: strconv.FormatUint(a.MemoryMax, 10), mode: 0o644})
		applied["memory_max"] = a.MemoryMax
	}
	if a.MemoryHigh != 0 {
		p.write(fileWrite{path: path.Join(dir, "memory.high"), content: strconv.FormatUint(a.MemoryHigh, 10), mode: 0o644})
		applied["memory_high"] = a.MemoryHigh
	}
	if a.PIDsMax != 0 {
		p.write(fileWrite{path: path.Join(dir, "pids.max"), content: strconv.FormatUint(a.PIDsMax, 10), mode: 0o644})
		applied["pids_max"] = a.PIDsMax
	}
	return p, map[string]any{"site": a.Site, "applied": applied}, nil
}

func opQuotaSet(cfg *runtimeCfg, raw json.RawMessage) (*plan, any, error) {
	var a struct {
		User   string `json:"user"`
		DiskMB uint64 `json:"disk_mb"`
		Inodes uint64 `json:"inodes"`
	}
	if err := strictDecode(raw, &a); err != nil {
		return nil, nil, fmt.Errorf("quota.set: %w", err)
	}
	if !reUserName.MatchString(a.User) {
		return nil, nil, errors.New("quota.set: user geçersiz")
	}
	if a.DiskMB == 0 && a.Inodes == 0 {
		return nil, nil, errors.New("quota.set: disk_mb veya inodes verilmeli")
	}
	if a.DiskMB != 0 && (a.DiskMB < minDiskMB || a.DiskMB > maxDiskMB) {
		return nil, nil, fmt.Errorf("quota.set: disk_mb aralık dışı (%d..%d)", minDiskMB, maxDiskMB)
	}
	if a.Inodes != 0 && (a.Inodes < minInodes || a.Inodes > maxInodes) {
		return nil, nil, fmt.Errorf("quota.set: inodes aralık dışı (%d..%d)", minInodes, maxInodes)
	}

	// setquota (ext4): blok birimi 1 KiB'tir → MiB * 1024.
	blocks := a.DiskMB * 1024
	setquota, _ := bin("setquota")
	p := &plan{}
	p.exec(setquota,
		"-u", a.User,
		strconv.FormatUint(blocks, 10), strconv.FormatUint(blocks, 10),
		strconv.FormatUint(a.Inodes, 10), strconv.FormatUint(a.Inodes, 10),
		cfg.quotaFS)
	return p, map[string]any{"user": a.User, "disk_mb": a.DiskMB, "inodes": a.Inodes, "filesystem": cfg.quotaFS}, nil
}

func opFirewallApply(cfg *runtimeCfg, raw json.RawMessage) (*plan, any, error) {
	var a struct {
		Ruleset string `json:"ruleset"`
	}
	if err := strictDecode(raw, &a); err != nil {
		return nil, nil, fmt.Errorf("firewall.apply: %w", err)
	}
	ruleset, err := cleanUnder(cfg.nftDir, a.Ruleset)
	if err != nil {
		return nil, nil, fmt.Errorf("firewall.apply: %w", err)
	}
	if !reFileName.MatchString(path.Base(ruleset)) {
		return nil, nil, errors.New("firewall.apply: dosya adı geçersiz")
	}
	nft, _ := bin("nft")
	p := &plan{}
	p.exec(nft, "-f", ruleset)
	return p, map[string]any{"ruleset": ruleset}, nil
}

func opSshdInstall(cfg *runtimeCfg, raw json.RawMessage) (*plan, any, error) {
	var a struct {
		Content string `json:"content"`
	}
	if err := strictDecode(raw, &a); err != nil {
		return nil, nil, fmt.Errorf("sshd.install_config: %w", err)
	}
	tmp := path.Join(cfg.stageDir, "sshd-sites.conf.tmp")

	sshd, _ := bin("sshd")
	systemctl, _ := bin("systemctl")
	dst := "/etc/ssh/sshd_config.d/aurapanel-sites.conf"
	p := &plan{}
	// Önce staging dizini oluşturulur, içerik temp'e yazılır, doğrulanır, kopyalanır.
	p.mkdirMode(cfg.stageDir, 0o755)
	p.write(fileWrite{path: tmp, content: a.Content, mode: 0o600})
	p.exec(sshd, "-t", "-f", tmp)
	p.copy(fileCopy{src: tmp, dst: dst, mode: 0o600})
	p.exec(systemctl, "reload", "ssh")
	return p, map[string]any{"config": dst}, nil
}

func opLogrotateInstall(cfg *runtimeCfg, raw json.RawMessage) (*plan, any, error) {
	var a struct {
		Config string `json:"config"`
	}
	if err := strictDecode(raw, &a); err != nil {
		return nil, nil, fmt.Errorf("logrotate.install_config: %w", err)
	}
	src, err := cleanUnder(cfg.stageDir, a.Config)
	if err != nil {
		return nil, nil, fmt.Errorf("logrotate.install_config: %w", err)
	}
	if !reFileName.MatchString(path.Base(src)) {
		return nil, nil, errors.New("logrotate.install_config: dosya adı geçersiz")
	}

	logrotate, _ := bin("logrotate")
	dst := "/etc/logrotate.d/aurapanel-sites"
	p := &plan{}
	p.exec(logrotate, "-d", src) // kaynağı doğrula
	p.copy(fileCopy{src: src, dst: dst, mode: 0o644})
	return p, map[string]any{"config": dst}, nil
}

// --- OLS operasyonları (ARCHITECTURE §3.2, W3) ---

// olsVhostsDir, OLS native vhost kökü (OLS 1.7+).
const olsVhostsDir = "/usr/local/lsws/conf/vhosts"

// olsFileAllowlist: bir site bundle'ında bulunabilecek dosya adları.
var olsFileAllowlist = map[string]struct{}{
	"main.conf":   {},
	"vhconf.conf": {},
}

const (
	olsBundleFileLimit    = 8
	olsBundleContentLimit = 256 << 10 // 256 KiB
)

func opOlsTest(cfg *runtimeCfg, raw json.RawMessage) (*plan, any, error) {
	var a struct{}
	if err := strictDecode(raw, &a); err != nil {
		return nil, nil, fmt.Errorf("ols.test: %w", err)
	}
	lshttpd, _ := bin("lshttpd")
	p := &plan{}
	p.exec(lshttpd, "-t")
	p.actions[len(p.actions)-1].exec.tolerateWarn = true
	return p, map[string]any{"tested": true}, nil
}

// opOlsReadBundle: snapshot için mevcut bundle dosyalarını okur (süreç içi).
func opOlsReadBundle(cfg *runtimeCfg, raw json.RawMessage) (*plan, any, error) {
	var a struct {
		Site string `json:"site"`
	}
	if err := strictDecode(raw, &a); err != nil {
		return nil, nil, fmt.Errorf("ols.read_bundle: %w", err)
	}
	if !reSiteID.MatchString(a.Site) {
		return nil, nil, errors.New("ols.read_bundle: site kimliği geçersiz")
	}
	dir := path.Join(olsVhostsDir, a.Site)
	files := []map[string]any{}
	for name := range olsFileAllowlist {
		full := path.Join(dir, name)
		b, err := os.ReadFile(full)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, nil, fmt.Errorf("ols.read_bundle: %w", err)
		}
		if len(b) > olsBundleContentLimit {
			return nil, nil, fmt.Errorf("ols.read_bundle: %s beklenmeyen boyutta (%d)", name, len(b))
		}
		st, err := os.Stat(full)
		if err != nil {
			return nil, nil, fmt.Errorf("ols.read_bundle: %w", err)
		}
		files = append(files, map[string]any{
			"name":    name,
			"content": string(b),
			"mode":    int(st.Mode().Perm()),
		})
	}
	return &plan{}, map[string]any{"site": a.Site, "files": files}, nil
}

type bundleFile struct {
	Name    string `json:"name"`
	Content string `json:"content"`
	Mode    int    `json:"mode"`
}

func validateBundle(a struct {
	Site  string       `json:"site"`
	Files []bundleFile `json:"files"`
}, opName string) (string, []bundleFile, error) {
	if !reSiteID.MatchString(a.Site) {
		return "", nil, errors.New(opName + ": site kimliği geçersiz")
	}
	if len(a.Files) == 0 || len(a.Files) > olsBundleFileLimit {
		return "", nil, fmt.Errorf("%s: dosya sayısı 1..%d olmalı", opName, olsBundleFileLimit)
	}
	seen := map[string]bool{}
	for _, f := range a.Files {
		if _, ok := olsFileAllowlist[f.Name]; !ok {
			return "", nil, fmt.Errorf("%s: dosya izinli değil: %q", opName, f.Name)
		}
		if seen[f.Name] {
			return "", nil, fmt.Errorf("%s: tekrarlanan dosya: %q", opName, f.Name)
		}
		seen[f.Name] = true
		if len(f.Content) > olsBundleContentLimit {
			return "", nil, fmt.Errorf("%s: %s içerik sınırını aşıyor", opName, f.Name)
		}
		if strings.ContainsRune(f.Content, '\x00') {
			return "", nil, fmt.Errorf("%s: %s NUL bayt içeriyor", opName, f.Name)
		}
		if f.Mode != 0 && (f.Mode < 0o600 || f.Mode > 0o644) {
			return "", nil, fmt.Errorf("%s: %s mode 0600..0644 olmalı", opName, f.Name)
		}
	}
	dir := path.Join(olsVhostsDir, a.Site)
	return dir, a.Files, nil
}

func opOlsInstallBundle(cfg *runtimeCfg, raw json.RawMessage) (*plan, any, error) {
	var a struct {
		Site  string       `json:"site"`
		Files []bundleFile `json:"files"`
	}
	if err := strictDecode(raw, &a); err != nil {
		return nil, nil, fmt.Errorf("ols.install_bundle: %w", err)
	}
	dir, files, err := validateBundle(a, "ols.install_bundle")
	if err != nil {
		return nil, nil, err
	}
	p := &plan{}
	p.mkdir(dir)
	installed := []string{}
	for _, f := range files {
		mode := os.FileMode(f.Mode)
		if mode == 0 {
			mode = 0o644
		}
		p.write(fileWrite{path: path.Join(dir, f.Name), content: f.Content, mode: mode})
		installed = append(installed, f.Name)
	}
	return p, map[string]any{"site": a.Site, "installed": installed}, nil
}

func opOlsRemoveBundle(cfg *runtimeCfg, raw json.RawMessage) (*plan, any, error) {
	var a struct {
		Site  string   `json:"site"`
		Names []string `json:"names"`
	}
	if err := strictDecode(raw, &a); err != nil {
		return nil, nil, fmt.Errorf("ols.remove_bundle: %w", err)
	}
	if !reSiteID.MatchString(a.Site) {
		return nil, nil, errors.New("ols.remove_bundle: site kimliği geçersiz")
	}
	if len(a.Names) == 0 || len(a.Names) > olsBundleFileLimit {
		return nil, nil, fmt.Errorf("ols.remove_bundle: ad sayısı 1..%d olmalı", olsBundleFileLimit)
	}
	seen := map[string]bool{}
	p := &plan{}
	removed := []string{}
	for _, n := range a.Names {
		if _, ok := olsFileAllowlist[n]; !ok {
			return nil, nil, fmt.Errorf("ols.remove_bundle: dosya izinli değil: %q", n)
		}
		if seen[n] {
			return nil, nil, fmt.Errorf("ols.remove_bundle: tekrarlanan dosya: %q", n)
		}
		seen[n] = true
		p.remove(path.Join(olsVhostsDir, a.Site, n))
		removed = append(removed, n)
	}
	// Tüm bundle dosyaları silinince vhost dizinini de kaldır.
	// OLS'nin "include conf/vhosts/*/main.conf" wildcard'ı bu dizini
	// görmemeli; aksi hâlde eksik vhconf.conf nedeniyle config testi bozulur.
	if len(seen) == len(olsFileAllowlist) {
		p.removeAll(path.Join(olsVhostsDir, a.Site))
	}
	return p, map[string]any{"site": a.Site, "removed": removed}, nil
}

func opOlsReload(cfg *runtimeCfg, raw json.RawMessage) (*plan, any, error) {
	var a struct{}
	if err := strictDecode(raw, &a); err != nil {
		return nil, nil, fmt.Errorf("ols.reload: %w", err)
	}
	lswsctrl, _ := bin("lswsctrl")
	p := &plan{}
	p.exec(lswsctrl, "reload")
	return p, map[string]any{"reloaded": true}, nil
}

// --- Site yaşam döngüsü operasyonları (W4) ---

// opSitePrepare, site dizin iskeletini kurar: logs (0750), tmp (0700)
// ve site kökünün tamamını site kullanıcısına devreder (chown -R).
func opSitePrepare(cfg *runtimeCfg, raw json.RawMessage) (*plan, any, error) {
	var a struct {
		Site string `json:"site"`
		User string `json:"user"`
	}
	if err := strictDecode(raw, &a); err != nil {
		return nil, nil, fmt.Errorf("site.prepare: %w", err)
	}
	if !reSiteID.MatchString(a.Site) {
		return nil, nil, errors.New("site.prepare: site kimliği geçersiz")
	}
	if a.User != "www-"+a.Site {
		return nil, nil, errors.New("site.prepare: user 'www-<site>' biçiminde olmalı")
	}
	root := path.Join(cfg.sitesRoot, a.Site)
	chown, _ := bin("chown")
	p := &plan{}
	p.mkdirMode(path.Join(root, "logs"), 0o750)
	p.mkdirMode(path.Join(root, "tmp"), 0o700)
	p.exec(chown, "-R", a.User+":"+a.User, root)
	return p, map[string]any{"site": a.Site, "root": root}, nil
}

// opSiteTeardown, site dizin ağacını kaldırır. RemoveAll yalnızca TAM
// site köküne uygulanır — başka hiçbir yol bu op'ta geçemez.
func opSiteTeardown(cfg *runtimeCfg, raw json.RawMessage) (*plan, any, error) {
	var a struct {
		Site string `json:"site"`
		User string `json:"user"`
	}
	if err := strictDecode(raw, &a); err != nil {
		return nil, nil, fmt.Errorf("site.teardown: %w", err)
	}
	if !reSiteID.MatchString(a.Site) {
		return nil, nil, errors.New("site.teardown: site kimliği geçersiz")
	}
	if a.User != "www-"+a.Site {
		return nil, nil, errors.New("site.teardown: user 'www-<site>' biçiminde olmalı")
	}
	root := path.Join(cfg.sitesRoot, a.Site)
	p := &plan{}
	p.removeAll(root)
	return p, map[string]any{"site": a.Site, "root": root}, nil
}

// opSiteCloneFiles securely copies files from one site to another (for staging).
func opSiteCloneFiles(cfg *runtimeCfg, raw json.RawMessage) (*plan, any, error) {
	var a struct {
		SrcSite string `json:"src_site"`
		DstSite string `json:"dst_site"`
	}
	if err := strictDecode(raw, &a); err != nil {
		return nil, nil, fmt.Errorf("site.clone_files: %w", err)
	}
	if !reSiteID.MatchString(a.SrcSite) || !reSiteID.MatchString(a.DstSite) {
		return nil, nil, errors.New("site.clone_files: site kimliği geçersiz")
	}
	
	srcHome := path.Join(cfg.sitesRoot, a.SrcSite, "home") + "/" // Trailing slash is important for rsync
	dstHome := path.Join(cfg.sitesRoot, a.DstSite, "home") + "/"
	
	dstUser := "www-" + a.DstSite
	
	// Ensure we only copy from valid sites
	// Since we execute this as root, we can use rsync and then chown the dst
	// Actually, instead of rsync, we can just use cp -a and chown, but rsync is better
	rsyncPath, _ := bin("rsync")
	chownPath, _ := bin("chown")
	
	p := &plan{}
	// -a: archive mode (recursive, preserve perms, owner, times)
	// --delete: delete extraneous files from destination
	p.exec(rsyncPath, "-a", "--delete", srcHome, dstHome)
	// chown -R the destination to the new user
	p.exec(chownPath, "-R", dstUser+":"+dstUser, dstHome)
	
	return p, map[string]any{"src": srcHome, "dst": dstHome}, nil
}

// opCgroupCleanup, site cgroup alt dizinini kaldırır. Cgroup içinde
// yaşayan süreçler varsa çekirdek reddeder (EBUSY) — hata panelde
// "deleting" durumunda kalır ve admin yeniden dener.
func opCgroupCleanup(cfg *runtimeCfg, raw json.RawMessage) (*plan, any, error) {
	var a struct {
		Site string `json:"site"`
	}
	if err := strictDecode(raw, &a); err != nil {
		return nil, nil, fmt.Errorf("cgroup.cleanup: %w", err)
	}
	if !reSiteID.MatchString(a.Site) {
		return nil, nil, errors.New("cgroup.cleanup: site kimliği geçersiz")
	}
	dir := path.Join(cfg.cgroupBase, "sites", a.Site)
	p := &plan{}
	p.removeAll(dir)
	return p, map[string]any{"site": a.Site}, nil
}

// opCgroupRead, site cgroup limitlerinin GERÇEK değerlerini okur (drift).
// Eksik dosyalar sessizce atlanır — "limit yok" demektir.
func opCgroupRead(cfg *runtimeCfg, raw json.RawMessage) (*plan, any, error) {
	var a struct {
		Site string `json:"site"`
	}
	if err := strictDecode(raw, &a); err != nil {
		return nil, nil, fmt.Errorf("cgroup.read: %w", err)
	}
	if !reSiteID.MatchString(a.Site) {
		return nil, nil, errors.New("cgroup.read: site kimliği geçersiz")
	}
	dir := path.Join(cfg.cgroupBase, "sites", a.Site)
	values := map[string]any{}
	for _, f := range []string{"cpu.max", "memory.max", "memory.high", "pids.max"} {
		b, err := os.ReadFile(path.Join(dir, f))
		if err != nil {
			continue // dosya yok → değer yok
		}
		values[f] = strings.TrimSpace(string(b))
	}
	return &plan{}, map[string]any{"site": a.Site, "values": values}, nil
}

// opSiteStatus, site dizinlerinin (home/logs/tmp) varlığını ve modunu okur (drift).
func opSiteStatus(cfg *runtimeCfg, raw json.RawMessage) (*plan, any, error) {
	var a struct {
		Site string `json:"site"`
		User string `json:"user"`
	}
	if err := strictDecode(raw, &a); err != nil {
		return nil, nil, fmt.Errorf("site.status: %w", err)
	}
	if !reSiteID.MatchString(a.Site) {
		return nil, nil, errors.New("site.status: site kimliği geçersiz")
	}
	if a.User != "www-"+a.Site {
		return nil, nil, errors.New("site.status: user 'www-<site>' biçiminde olmalı")
	}
	root := path.Join(cfg.sitesRoot, a.Site)
	dirs := map[string]any{}
	for _, d := range []string{"home", "logs", "tmp"} {
		st, err := os.Stat(path.Join(root, d))
		if err != nil {
			dirs[d] = map[string]any{"exists": false}
			continue
		}
		dirs[d] = map[string]any{"exists": true, "mode": int(st.Mode().Perm())}
	}
	return &plan{}, map[string]any{"site": a.Site, "dirs": dirs}, nil
}

// opQuotaGet, kullanıcının quota hard limitlerini okur (drift).
// Quota etkin değilse hata değil "available:false" döner — bu bir drift
// değil kurulum (W13) sorunudur.
func opQuotaGet(cfg *runtimeCfg, raw json.RawMessage) (*plan, any, error) {
	var a struct {
		User string `json:"user"`
	}
	if err := strictDecode(raw, &a); err != nil {
		return nil, nil, fmt.Errorf("quota.get: %w", err)
	}
	if !reUserName.MatchString(a.User) {
		return nil, nil, errors.New("quota.get: user geçersiz")
	}
	u, err := user.Lookup(a.User)
	if err != nil {
		if runtime.GOOS != "linux" {
			// Windows geliştirme ortamında kullanıcı veritabanı farklıdır;
			// user.exists ile aynı davranış: sorgulanamayan = yok.
			return &plan{}, map[string]any{"user": a.User, "available": false, "reason": "kullanıcı bulunamadı"}, nil
		}
		return nil, nil, fmt.Errorf("quota.get: kullanıcı bulunamadı: %w", err)
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return nil, nil, fmt.Errorf("quota.get: uid: %w", err)
	}
	blocks, inodes, err := readQuota(cfg.quotaFS, uint32(uid))
	if err != nil {
		return &plan{}, map[string]any{"user": a.User, "available": false, "reason": err.Error()}, nil
	}
	return &plan{}, map[string]any{
		"user": a.User, "available": true,
		"disk_blocks": blocks, "inodes": inodes,
	}, nil
}

// --- PHP operasyonları (W6) ---

var reLSPhpDir = regexp.MustCompile(`^lsphp[0-9]{2}$`)

const phpIniLimit = 64 << 10 // 64 KiB

// opPHPDetect, kurulu LSPHP sürümlerini /usr/local/lsws altından tarar
// (lsphpNN/bin/lsphp var mı). Hiçbir kullanıcı girdisi yoktur.
func opPHPDetect(cfg *runtimeCfg, raw json.RawMessage) (*plan, any, error) {
	var a struct{}
	if err := strictDecode(raw, &a); err != nil {
		return nil, nil, fmt.Errorf("php.detect: %w", err)
	}
	versions := map[string]any{}
	entries, err := os.ReadDir("/usr/local/lsws")
	if err == nil {
		for _, e := range entries {
			if !e.IsDir() || !reLSPhpDir.MatchString(e.Name()) {
				continue
			}
			if _, err := os.Stat(path.Join("/usr/local/lsws", e.Name(), "bin", "lsphp")); err == nil {
				versions[e.Name()[5:6] + "." + e.Name()[6:]] = true
			}
		}
	}
	return &plan{}, map[string]any{"versions": versions}, nil
}

// reIniLine, php.ini satırlarının izin verilen biçimi:
//   yönerge[boşluk]=[boşluk]değer
// Değer karakter kümesi DAR tutulur (harf/rakam/./ /:+_- ve boşluk):
// allowlist anahtarlarımızın (boyutlar, tam sayılar, On/Off, zaman dilimi)
// ihtiyaç duyduğu küme budur; `;`, tırnak, parantez vb. YASAKTIR.
var reIniLine = regexp.MustCompile(`^[a-zA-Z0-9_.\[\]-]+\s*=\s*[a-zA-Z0-9./:+_\-, ]{0,255}$`)

// validateIniContent, php.ini içeriğini satır satır denetler. Yorum (#)
// ve boş satırlar serbesttir; diğer her satır reIniLine'a uymalıdır.
func validateIniContent(content string) error {
	if len(content) > phpIniLimit {
		return errors.New("php.ini boyutu 64 KiB sınırını aşıyor")
	}
	for i, line := range strings.Split(content, "\n") {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "#") || strings.HasPrefix(t, ";") {
			continue
		}
		if !reIniLine.MatchString(t) {
			return fmt.Errorf("php.ini satır %d geçersiz", i+1)
		}
	}
	return nil
}

func iniPathFor(cfg *runtimeCfg, siteID string) string {
	return path.Join(cfg.sitesRoot, siteID, "conf", "php.ini")
}

// opPHPInstallIni, site php.ini dosyasını doğrulayıp yazar (site conf dizininde,
// site kullanıcısına ait).
func opPHPInstallIni(cfg *runtimeCfg, raw json.RawMessage) (*plan, any, error) {
	var a struct {
		Site    string `json:"site"`
		Content string `json:"content"`
	}
	if err := strictDecode(raw, &a); err != nil {
		return nil, nil, fmt.Errorf("php.install_ini: %w", err)
	}
	if !reSiteID.MatchString(a.Site) {
		return nil, nil, errors.New("php.install_ini: site kimliği geçersiz")
	}
	if err := validateIniContent(a.Content); err != nil {
		return nil, nil, fmt.Errorf("php.install_ini: %w", err)
	}
	target := iniPathFor(cfg, a.Site)
	user := "www-" + a.Site
	chown, _ := bin("chown")
	p := &plan{}
	p.mkdirMode(path.Dir(target), 0o750)
	p.write(fileWrite{path: target, content: a.Content, mode: 0o644})
	p.exec(chown, "-R", user+":"+user, path.Dir(target))
	return p, map[string]any{"site": a.Site, "path": target}, nil
}

// opPHPReadIni, site php.ini içeriğini okur (editör için).
func opPHPReadIni(cfg *runtimeCfg, raw json.RawMessage) (*plan, any, error) {
	var a struct {
		Site string `json:"site"`
	}
	if err := strictDecode(raw, &a); err != nil {
		return nil, nil, fmt.Errorf("php.read_ini: %w", err)
	}
	if !reSiteID.MatchString(a.Site) {
		return nil, nil, errors.New("php.read_ini: site kimliği geçersiz")
	}
	b, err := os.ReadFile(iniPathFor(cfg, a.Site))
	if errors.Is(err, os.ErrNotExist) {
		return &plan{}, map[string]any{"site": a.Site, "content": ""}, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("php.read_ini: %w", err)
	}
	return &plan{}, map[string]any{"site": a.Site, "content": string(b)}, nil
}

func opServerServices(cfg *runtimeCfg, raw json.RawMessage) (*plan, any, error) {
	p := &plan{}
	out := map[string]string{}
	services := []string{"lsws", "mariadb", "fail2ban", "sshd"}
	for _, svc := range services {
		cmd := exec.Command("systemctl", "is-active", svc)
		if err := cmd.Run(); err == nil {
			out[svc] = "active"
		} else {
			out[svc] = "inactive"
		}
	}
	return p, out, nil
}

func opServerAction(cfg *runtimeCfg, raw json.RawMessage) (*plan, any, error) {
	var a struct {
		Action string `json:"action"`
		Target string `json:"target"`
	}
	if err := strictDecode(raw, &a); err != nil {
		return nil, nil, err
	}
	p := &plan{}
	if a.Action == "restart" || a.Action == "stop" || a.Action == "start" {
		if a.Target == "fail2ban" || a.Target == "lsws" || a.Target == "mariadb" {
			systemctl, _ := bin("systemctl")
			p.exec(systemctl, a.Action, a.Target)
		}
	}
	return p, map[string]string{"status": "ok"}, nil
}
// --- Güvenlik Duvarı Yönetimi (firewall.list / firewall.rule_add / firewall.rule_delete / firewall.ssh_port / firewall.panel_port) ---

// fwPortRange: 1-65535, rezerve portlar (0, 1024'ün altı sistem portlarına dikkat)
// panelde zaten UI doğrulaması var; burada sadece sayısal aralık kontrol.
var rePort = regexp.MustCompile(`^[0-9]{1,5}$`)

func validatePort(p int) error {
	if p < 1 || p > 65535 {
		return fmt.Errorf("geçersiz port: %d (1-65535 arası olmalı)", p)
	}
	return nil
}

// nftableConfig, mevcut aurapanel nftables kuralsetini döndürür (proses içi — exec yok).
func opFirewallList(cfg *runtimeCfg, raw json.RawMessage) (*plan, any, error) {
	data, err := os.ReadFile("/etc/nftables.d/aurapanel.nft")
	if err != nil {
		// Dosya yoksa boş kural seti döndür — henüz oluşturulmamış
		return &plan{}, map[string]any{"rules": []any{}, "raw": ""}, nil
	}
	content := string(data)
	// Basit parser: "tcp dport NNN accept" satırlarını yakala
	type Rule struct {
		Port    int    `json:"port"`
		Proto   string `json:"proto"`
		Comment string `json:"comment"`
	}
	var rules []Rule
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		// Örn: tcp dport 8080 accept # panel
		re := regexp.MustCompile(`(tcp|udp)\s+dport\s+(\d+)\s+accept(?:\s+#\s*(.+))?`)
		m := re.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		port := 0
		fmt.Sscanf(m[2], "%d", &port)
		// 22, 80, 443 temel kurallar — bunları da listele
		rules = append(rules, Rule{Port: port, Proto: m[1], Comment: strings.TrimSpace(m[3])})
	}
	return &plan{}, map[string]any{"rules": rules, "raw": content}, nil
}

// opFirewallRuleAdd: tek port açar (tcp veya udp).
func opFirewallRuleAdd(cfg *runtimeCfg, raw json.RawMessage) (*plan, any, error) {
	var a struct {
		Port    int    `json:"port"`
		Proto   string `json:"proto"`   // "tcp" | "udp"
		Comment string `json:"comment"` // kısa açıklama (opsiyonel)
	}
	if err := strictDecode(raw, &a); err != nil {
		return nil, nil, err
	}
	if err := validatePort(a.Port); err != nil {
		return nil, nil, err
	}
	if a.Proto != "tcp" && a.Proto != "udp" {
		return nil, nil, fmt.Errorf("geçersiz protokol: %q (tcp|udp)", a.Proto)
	}
	comment := strings.TrimSpace(a.Comment)
	if len(comment) > 64 {
		return nil, nil, errors.New("yorum 64 karakteri geçemez")
	}
	// Sadece harf, rakam, boşluk ve tire
	if matched, _ := regexp.MatchString(`^[a-zA-Z0-9 _\-]*$`, comment); !matched {
		return nil, nil, errors.New("yorum geçersiz karakter içeriyor")
	}

	nft, _ := bin("nft")
	rule := fmt.Sprintf("%s dport %d accept", a.Proto, a.Port)
	if comment != "" {
		rule += " # " + comment
	}
	p := &plan{}
	// nft table'a dinamik kural ekle (çalışan ruleset'e — persist için dosya da güncellenir)
	p.exec(nft, "add", "rule", "inet", "aurapanel", "input", a.Proto, "dport", fmt.Sprintf("%d", a.Port), "accept")
	// Kalıcılık için nftables.d dosyasını yeniden oluştur (aşağıdaki helper kullanılır exec_linux'ta)
	return p, map[string]any{"added": a.Port, "proto": a.Proto}, nil
}

// opFirewallRuleDelete: açık bir portu kapatır.
// Güvenlik: 22 (SSH) ve 80, 443 silinemez.
func opFirewallRuleDelete(cfg *runtimeCfg, raw json.RawMessage) (*plan, any, error) {
	var a struct {
		Port  int    `json:"port"`
		Proto string `json:"proto"`
	}
	if err := strictDecode(raw, &a); err != nil {
		return nil, nil, err
	}
	if err := validatePort(a.Port); err != nil {
		return nil, nil, err
	}
	if a.Proto != "tcp" && a.Proto != "udp" {
		return nil, nil, fmt.Errorf("geçersiz protokol: %q", a.Proto)
	}
	// Çekirdek portları koruması
	protected := map[int]bool{22: true, 80: true, 443: true}
	if protected[a.Port] {
		return nil, nil, fmt.Errorf("port %d silinemez: sistem tarafından korunan port", a.Port)
	}
	nft, _ := bin("nft")
	p := &plan{}
	p.exec(nft, "delete", "rule", "inet", "aurapanel", "input",
		"handle", fmt.Sprintf("$(nft -a list chain inet aurapanel input | awk '/%s dport %d accept/ {print $NF}')", a.Proto, a.Port))
	return p, map[string]any{"deleted": a.Port}, nil
}

// opSSHPortChange: SSH portunu değiştirir.
// Güvenlik sırası: 1) yeni port aç  2) sshd_config yaz  3) sshd reload  4) eski port kapat
// Bu sıra lockout'u önler.
func opSSHPortChange(cfg *runtimeCfg, raw json.RawMessage) (*plan, any, error) {
	var a struct {
		NewPort int `json:"new_port"`
		OldPort int `json:"old_port"`
	}
	if err := strictDecode(raw, &a); err != nil {
		return nil, nil, err
	}
	if err := validatePort(a.NewPort); err != nil {
		return nil, nil, fmt.Errorf("yeni port: %w", err)
	}
	if err := validatePort(a.OldPort); err != nil {
		return nil, nil, fmt.Errorf("eski port: %w", err)
	}
	if a.NewPort == a.OldPort {
		return nil, nil, errors.New("yeni port eski portla aynı")
	}
	// 80 ve 443'e SSH konulamaz
	if a.NewPort == 80 || a.NewPort == 443 {
		return nil, nil, fmt.Errorf("port %d HTTP/HTTPS için ayrılmış, SSH için kullanılamaz", a.NewPort)
	}

	nft, _ := bin("nft")
	sshd, _ := bin("sshd")
	systemctl, _ := bin("systemctl")

	p := &plan{}

	// Adım 1: Yeni SSH portunu güvenlik duvarında AÇ (henüz eski port da açık kalıyor)
	p.exec(nft, "add", "rule", "inet", "aurapanel", "input", "tcp", "dport", fmt.Sprintf("%d", a.NewPort), "accept")

	// Adım 2: sshd_config dosyasını güncelle
	sshdConfig := fmt.Sprintf(`# AuraPanel tarafından yönetilir — elle düzenleme önerilmez.
Port %d
PermitRootLogin prohibit-password
PubkeyAuthentication yes
PasswordAuthentication yes
ChallengeResponseAuthentication no
UsePAM yes
PrintMotd no
AcceptEnv LANG LC_*
Subsystem sftp internal-sftp
`, a.NewPort)
	p.write(fileWrite{path: "/etc/ssh/sshd_config", content: sshdConfig, mode: 0o600})

	// Adım 3: Konfigürasyon geçerlilik testi
	p.exec(sshd, "-t")

	// Adım 4: sshd reload (yeni port etkinleşir, mevcut bağlantılar kesilmez)
	p.exec(systemctl, "reload", "sshd")

	// Adım 5: nftables kalıcı config dosyasını güncelle (exec_linux.go bunu file rewrite ile yapar)

	return p, map[string]any{"new_port": a.NewPort, "old_port": a.OldPort}, nil
}

// opPanelPortChange: AuraPanel dinleme adresini değiştirir (aurapanel.yaml listen.address).
func opPanelPortChange(cfg *runtimeCfg, raw json.RawMessage) (*plan, any, error) {
	var a struct {
		NewPort int `json:"new_port"`
	}
	if err := strictDecode(raw, &a); err != nil {
		return nil, nil, err
	}
	if err := validatePort(a.NewPort); err != nil {
		return nil, nil, fmt.Errorf("panel port: %w", err)
	}
	if a.NewPort == 80 || a.NewPort == 443 {
		return nil, nil, fmt.Errorf("port %d HTTP/HTTPS için ayrılmış", a.NewPort)
	}

	nft, _ := bin("nft")
	systemctl, _ := bin("systemctl")

	p := &plan{}
	// Yeni portu aç
	p.exec(nft, "add", "rule", "inet", "aurapanel", "input", "tcp", "dport", fmt.Sprintf("%d", a.NewPort), "accept")

	// aurapanel.yaml listen.address satırını güncelle (sed yerine doğrudan yaz)
	yamlPath := "/etc/aurapanel/aurapanel.yaml"
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		return nil, nil, fmt.Errorf("config okunamadı: %w", err)
	}
	updated := regexp.MustCompile(`address:\s*"[^"]*"`).
		ReplaceAll(data, []byte(fmt.Sprintf(`address: "127.0.0.1:%d"`, a.NewPort)))
	p.write(fileWrite{path: yamlPath, content: string(updated), mode: 0o640})

	// Servisi yeniden başlat (yeni port ile dinler)
	p.exec(systemctl, "restart", "aurapanel")

	return p, map[string]any{"new_port": a.NewPort}, nil
}

// --- Mail operasyonları (Postfix + Dovecot + OpenDKIM) ---

// reMailDomain: basit domain doğrulaması (RFC 952 alt kümesi).
var reMailDomain = regexp.MustCompile(`^[a-z0-9]([a-z0-9.-]{0,253}[a-z0-9])?$`)

// reMailLocal: yerel kısım (@ öncesi).
var reMailLocal = regexp.MustCompile(`^[a-z0-9][a-z0-9._+-]{0,63}$`)

// opMailSetup: tek seferlik Postfix/Dovecot/OpenDKIM yapılandırması.
// Args: {} (boş JSON).
func opMailSetup(_ *runtimeCfg, raw json.RawMessage) (*plan, any, error) {
	var a struct{}
	if err := strictDecode(raw, &a); err != nil {
		return nil, nil, fmt.Errorf("mail.setup: %w", err)
	}

	// hostname oku (süreç içi)
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "localhost"
	}

	systemctl, _ := bin("systemctl")
	chown, _ := bin("chown")

	p := &plan{}

	// --- /var/vmail ---
	p.mkdirMode("/var/vmail", 0o755)
	p.exec(chown, "5000:5000", "/var/vmail")

	// --- Postfix ---
	postfixMainCf := `# AuraPanel managed - virtual mailbox config
smtpd_tls_cert_file = /etc/ssl/certs/ssl-cert-snakeoil.pem
smtpd_tls_key_file = /etc/ssl/private/ssl-cert-snakeoil.key
smtpd_tls_security_level = may
smtp_tls_security_level = may
smtpd_sasl_type = dovecot
smtpd_sasl_path = private/auth
smtpd_sasl_auth_enable = yes
smtpd_recipient_restrictions = permit_sasl_authenticated, permit_mynetworks, reject_unauth_destination
virtual_mailbox_domains = /etc/postfix/virtual_domains
virtual_mailbox_base = /var/vmail
virtual_mailbox_maps = hash:/etc/postfix/virtual_mailboxes
virtual_uid_maps = static:5000
virtual_gid_maps = static:5000
milter_protocol = 6
milter_default_action = accept
smtpd_milters = unix:/run/opendkim/opendkim.sock
non_smtpd_milters = unix:/run/opendkim/opendkim.sock
myhostname = ` + hostname + "\n"

	p.write(fileWrite{path: "/etc/postfix/main.cf", content: postfixMainCf, mode: 0o644})
	p.write(fileWrite{path: "/etc/postfix/virtual_domains", content: "", mode: 0o644})
	p.write(fileWrite{path: "/etc/postfix/virtual_mailboxes", content: "", mode: 0o644})

	// --- Dovecot ---
	dovecotMailConf := `# AuraPanel managed
mail_location = maildir:/var/vmail/%d/%n
mail_uid = 5000
mail_gid = 5000
mail_privileged_group = mail
first_valid_uid = 5000
last_valid_uid = 5000
`
	dovecotAuthConf := `# AuraPanel managed
disable_plaintext_auth = yes
auth_mechanisms = plain login

passdb {
  driver = passwd-file
  args = scheme=BLF-CRYPT username_format=%u /etc/dovecot/users
}

userdb {
  driver = passwd-file
  args = username_format=%u /etc/dovecot/users
}

service auth {
  unix_listener /var/spool/postfix/private/auth {
    mode = 0660
    user = postfix
    group = postfix
  }
}
`
	dovecotSSLConf := `# AuraPanel managed
ssl = yes
ssl_cert = </etc/ssl/certs/ssl-cert-snakeoil.pem
ssl_key = </etc/ssl/private/ssl-cert-snakeoil.key
ssl_min_protocol = TLSv1.2
`
	dovecotLMTPConf := `# AuraPanel managed
protocol lmtp {
  mail_plugins = $mail_plugins
}

service lmtp {
  unix_listener /var/spool/postfix/private/dovecot-lmtp {
    mode = 0600
    user = postfix
    group = postfix
  }
}
`

	p.mkdir("/etc/dovecot/conf.d")
	p.write(fileWrite{path: "/etc/dovecot/conf.d/10-mail.conf", content: dovecotMailConf, mode: 0o644})
	p.write(fileWrite{path: "/etc/dovecot/conf.d/10-auth.conf", content: dovecotAuthConf, mode: 0o644})
	p.write(fileWrite{path: "/etc/dovecot/conf.d/10-ssl.conf", content: dovecotSSLConf, mode: 0o644})
	p.write(fileWrite{path: "/etc/dovecot/conf.d/20-lmtp.conf", content: dovecotLMTPConf, mode: 0o644})
	p.write(fileWrite{path: "/etc/dovecot/users", content: "", mode: 0o640})

	// --- OpenDKIM ---
	opendkimConf := `# AuraPanel managed
Syslog          yes
SyslogSuccess   yes
LogWhy          yes
Canonicalization relaxed/simple
Mode            sv
SubDomains      no
AutoRestart     yes
AutoRestartRate 10/1M
Background      yes
DNSTimeout      5
SignatureAlgorithm rsa-sha256

KeyTable        /etc/opendkim/KeyTable
SigningTable    refile:/etc/opendkim/SigningTable
ExternalIgnoreList /etc/opendkim/TrustedHosts
InternalHosts   /etc/opendkim/TrustedHosts

OversignHeaders From
Socket          local:/run/opendkim/opendkim.sock
PidFile         /run/opendkim/opendkim.pid
UMask           007
UserID          opendkim
`

	p.mkdir("/etc/opendkim")
	p.mkdir("/etc/opendkim/keys")
	p.write(fileWrite{path: "/etc/opendkim.conf", content: opendkimConf, mode: 0o644})
	p.write(fileWrite{path: "/etc/opendkim/KeyTable", content: "", mode: 0o644})
	p.write(fileWrite{path: "/etc/opendkim/SigningTable", content: "", mode: 0o644})
	p.write(fileWrite{path: "/etc/opendkim/TrustedHosts", content: "127.0.0.1\nlocalhost\n", mode: 0o644})

	// --- Servisleri etkinleştir ---
	p.exec(systemctl, "enable", "--now", "postfix")
	p.exec(systemctl, "enable", "--now", "dovecot")
	p.exec(systemctl, "enable", "--now", "opendkim")

	return p, map[string]any{"hostname": hostname}, nil
}

// opMailProvision: sanal haritaları ve dovecot kullanıcılarını yeniden üretir.
func opMailProvision(_ *runtimeCfg, raw json.RawMessage) (*plan, any, error) {
	var a struct {
		Domains  []string `json:"domains"`
		Accounts []struct {
			Email        string `json:"email"`
			PasswordHash string `json:"password_hash"`
			QuotaMB      int    `json:"quota_mb"`
			Domain       string `json:"domain"`
		} `json:"accounts"`
	}
	if err := strictDecode(raw, &a); err != nil {
		return nil, nil, fmt.Errorf("mail.provision: %w", err)
	}

	// Doğrulama
	for _, d := range a.Domains {
		if !reMailDomain.MatchString(d) {
			return nil, nil, fmt.Errorf("mail.provision: geçersiz domain: %q", d)
		}
	}

	domainSet := make(map[string]struct{}, len(a.Domains))
	for _, d := range a.Domains {
		domainSet[d] = struct{}{}
	}

	for _, acc := range a.Accounts {
		parts := strings.SplitN(acc.Email, "@", 2)
		if len(parts) != 2 {
			return nil, nil, fmt.Errorf("mail.provision: geçersiz e-posta: %q", acc.Email)
		}
		local, domain := parts[0], parts[1]
		if !reMailLocal.MatchString(local) {
			return nil, nil, fmt.Errorf("mail.provision: geçersiz yerel kısım: %q", local)
		}
		if !reMailDomain.MatchString(domain) {
			return nil, nil, fmt.Errorf("mail.provision: geçersiz domain: %q", domain)
		}
		if _, ok := domainSet[domain]; !ok {
			return nil, nil, fmt.Errorf("mail.provision: hesap domain'i listede yok: %q", domain)
		}
		if acc.Domain != domain {
			return nil, nil, fmt.Errorf("mail.provision: domain alanı uyumsuz: %q != %q", acc.Domain, domain)
		}
		if acc.PasswordHash == "" {
			return nil, nil, fmt.Errorf("mail.provision: boş parola hash'i: %q", acc.Email)
		}
		if acc.QuotaMB < 0 {
			return nil, nil, fmt.Errorf("mail.provision: geçersiz kota: %d", acc.QuotaMB)
		}
	}

	postmap, _ := bin("postmap")
	systemctl, _ := bin("systemctl")
	chown, _ := bin("chown")

	p := &plan{}

	// 1. virtual_domains
	var domBuf strings.Builder
	for _, d := range a.Domains {
		domBuf.WriteString(d)
		domBuf.WriteByte('\n')
	}
	p.write(fileWrite{path: "/etc/postfix/virtual_domains", content: domBuf.String(), mode: 0o644})

	// 2. virtual_mailboxes
	var mboxBuf strings.Builder
	for _, acc := range a.Accounts {
		parts := strings.SplitN(acc.Email, "@", 2)
		local, domain := parts[0], parts[1]
		fmt.Fprintf(&mboxBuf, "%s\t%s/%s/\n", acc.Email, domain, local)
	}
	p.write(fileWrite{path: "/etc/postfix/virtual_mailboxes", content: mboxBuf.String(), mode: 0o644})

	// 3. postmap
	p.exec(postmap, "/etc/postfix/virtual_mailboxes")

	// 4. dovecot users
	var usersBuf strings.Builder
	for _, acc := range a.Accounts {
		parts := strings.SplitN(acc.Email, "@", 2)
		local, domain := parts[0], parts[1]
		quotaRule := ""
		if acc.QuotaMB > 0 {
			quotaRule = fmt.Sprintf("userdb_quota_rule=*:storage=%dM", acc.QuotaMB)
		}
		fmt.Fprintf(&usersBuf, "%s:{BLF-CRYPT}%s:5000:5000::/var/vmail/%s/%s::%s\n",
			acc.Email, acc.PasswordHash, domain, local, quotaRule)
	}
	p.write(fileWrite{path: "/etc/dovecot/users", content: usersBuf.String(), mode: 0o640})

	// 5. maildir dizinlerini oluştur
	for _, acc := range a.Accounts {
		parts := strings.SplitN(acc.Email, "@", 2)
		local, domain := parts[0], parts[1]
		maildir := fmt.Sprintf("/var/vmail/%s/%s", domain, local)
		p.mkdirMode(maildir, 0o700)
		p.exec(chown, "5000:5000", maildir)
	}
	// domain dizinlerinin sahipliği
	for _, d := range a.Domains {
		domDir := "/var/vmail/" + d
		p.mkdirMode(domDir, 0o755)
		p.exec(chown, "5000:5000", domDir)
	}

	// 6-7. servisleri yeniden yükle
	p.exec(systemctl, "reload", "postfix")
	p.exec(systemctl, "reload", "dovecot")

	return p, map[string]any{"domains": len(a.Domains), "accounts": len(a.Accounts)}, nil
}

// opMailDKIMGenerate: domain için DKIM anahtar çifti üretir (Go crypto, dış binary yok).
// KeyTable/SigningTable/TrustedHosts dosyaları süreç içi okunup güncellenir.
func opMailDKIMGenerate(_ *runtimeCfg, raw json.RawMessage) (*plan, any, error) {
	var a struct {
		Domain string `json:"domain"`
	}
	if err := strictDecode(raw, &a); err != nil {
		return nil, nil, fmt.Errorf("mail.dkim_generate: %w", err)
	}
	if !reMailDomain.MatchString(a.Domain) {
		return nil, nil, fmt.Errorf("mail.dkim_generate: geçersiz domain: %q", a.Domain)
	}

	// RSA 2048-bit anahtar çifti üret (süreç içi)
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, fmt.Errorf("mail.dkim_generate: anahtar üretme hatası: %w", err)
	}

	privPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privKey),
	})

	pubDER, err := x509.MarshalPKIXPublicKey(&privKey.PublicKey)
	if err != nil {
		return nil, nil, fmt.Errorf("mail.dkim_generate: public key marshal hatası: %w", err)
	}
	pubB64 := base64.StdEncoding.EncodeToString(pubDER)

	chown, _ := bin("chown")
	systemctl, _ := bin("systemctl")

	keyDir := "/etc/opendkim/keys/" + a.Domain

	// Mevcut tablo dosyalarını oku (süreç içi) ve yeni satırları ekle
	keyTableLine := fmt.Sprintf("mail._domainkey.%s %s:mail:%s/mail.private\n",
		a.Domain, a.Domain, keyDir)
	signingTableLine := fmt.Sprintf("*@%s mail._domainkey.%s\n", a.Domain, a.Domain)
	trustedLine := a.Domain + "\n"

	keyTable, _ := os.ReadFile("/etc/opendkim/KeyTable")
	signingTable, _ := os.ReadFile("/etc/opendkim/SigningTable")
	trustedHosts, _ := os.ReadFile("/etc/opendkim/TrustedHosts")

	newKeyTable := string(keyTable) + keyTableLine
	newSigningTable := string(signingTable) + signingTableLine
	newTrustedHosts := string(trustedHosts)
	if !strings.Contains(newTrustedHosts, trustedLine) {
		newTrustedHosts += trustedLine
	}

	p := &plan{}

	// Anahtar dizini ve dosyaları
	p.mkdirMode(keyDir, 0o700)
	p.write(fileWrite{path: keyDir + "/mail.private", content: string(privPEM), mode: 0o600})
	p.exec(chown, "-R", "opendkim:opendkim", keyDir)

	// Tablo dosyalarını güncelle
	p.write(fileWrite{path: "/etc/opendkim/KeyTable", content: newKeyTable, mode: 0o644})
	p.write(fileWrite{path: "/etc/opendkim/SigningTable", content: newSigningTable, mode: 0o644})
	p.write(fileWrite{path: "/etc/opendkim/TrustedHosts", content: newTrustedHosts, mode: 0o644})

	// opendkim yeniden yükle
	p.exec(systemctl, "restart", "opendkim")

	return p, map[string]any{"public_key": pubB64, "domain": a.Domain}, nil
}
