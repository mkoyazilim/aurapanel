// ops.go: op kayıt defteri ve doğrulama/planlama mantığı.
// ÇAPRAZ PLATFORM: bu dosyada hiçbir komut yürütülmez, yalnızca plan üretilir.
// Yürütme exec_linux.go'dadır ve yalnızca Linux'ta derlenir.
package priv

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
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
	actExec
)

type action struct {
	kind  actionKind
	mkdir string
	write fileWrite
	copy  fileCopy
	exec  execSpec
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
	bin  string
	args []string
}

type plan struct {
	actions []action
}

func (p *plan) mkdir(path string)      { p.actions = append(p.actions, action{kind: actMkdir, mkdir: path}) }
func (p *plan) write(f fileWrite)      { p.actions = append(p.actions, action{kind: actWrite, write: f}) }
func (p *plan) copy(c fileCopy)        { p.actions = append(p.actions, action{kind: actCopy, copy: c}) }
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
		"cgroup.limits":             opCgroupLimits,
		"quota.set":                 opQuotaSet,
		"firewall.apply":            opFirewallApply,
		"sshd.install_config":       opSshdInstall,
		"logrotate.install_config":  opLogrotateInstall,
	}
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
	p.mkdir(home)
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
	chown, _ := bin("chown")
	p := &plan{}
	p.mkdir(base)
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
		Config string `json:"config"`
	}
	if err := strictDecode(raw, &a); err != nil {
		return nil, nil, fmt.Errorf("sshd.install_config: %w", err)
	}
	src, err := cleanUnder(cfg.stageDir, a.Config)
	if err != nil {
		return nil, nil, fmt.Errorf("sshd.install_config: %w", err)
	}
	if !reFileName.MatchString(path.Base(src)) {
		return nil, nil, errors.New("sshd.install_config: dosya adı geçersiz")
	}

	sshd, _ := bin("sshd")
	systemctl, _ := bin("systemctl")
	dst := "/etc/ssh/sshd_config.d/aurapanel-sites.conf"
	p := &plan{}
	// Önce kaynak doğrulanır, sonra kopyalanır, sonra reload — sıra önemli.
	p.exec(sshd, "-t", "-f", src)
	p.copy(fileCopy{src: src, dst: dst, mode: 0o600})
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
