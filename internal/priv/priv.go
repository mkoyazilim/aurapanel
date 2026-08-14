// Package priv, AuraPanel'in ayrıcalıklı yardımcı sürecini uygular
// (ARCHITECTURE §3): allowlist tabanlı op'lar, Unix socket + SO_PEERCRED
// kimlik doğrulaması, append-only priv log.
//
// Güvenlik modeli:
//   - Panel process'i root olarak ÇALIŞMAZ; privileged işlemler yalnızca burada.
//   - Kullanıcı girdisi ASLA shell'e aktarılmaz; her op sabit binary + argüman
//     dizisi üretir ve binary'ler binPaths haritasındaki mutlak yollardan gelir.
//   - Yürütme (executePlan) yalnızca Linux'ta derlenir; doğrulama/planlama
//     mantığı (ops.go) çapraz platformdur ve fuzz testlerine açıktır.
package priv

import (
	"context"
	"flag"
	"fmt"
	logpkg "log"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"time"
)

// Version, -ldflags ile doldurulabilir (priv.ping yanıtında döner).
var Version = "dev"

const requestTimeout = 10 * time.Second

// Main, aurapanel-priv giriş noktasıdır; çıkış kodunu döndürür.
func Main(argv []string) int {
	// file.op worker modu: helper kendini site UID'siyle yeniden başlatır
	// (FILE_MANAGER §14 Tier-1) — bayrak yalnızca helper TARAFINDAN konur.
	if os.Getenv("AURAPANEL_FILE_WORKER") == "1" {
		return fileWorkerMain(argv)
	}

	fs := flag.NewFlagSet("aurapanel-priv", flag.ContinueOnError)
	socketPath := fs.String("socket", "/run/aurapanel/priv.sock", "helper Unix socket yolu")
	panelUser := fs.String("panel-user", "aurapanel", "sokete bağlanmasına izin verilen panel kullanıcısı")
	privLogPath := fs.String("priv-log", "/var/log/aurapanel/priv.log", "append-only priv log yolu")
	quotaFS := fs.String("quota-fs", "/", "filesystem quota uygulanacak dosya sistemi")
	opTimeout := fs.Duration("op-timeout", 30*time.Second, "tek op için zaman aşımı")
	check := fs.Bool("check", false, "ortam doğrulaması yap ve çık")
	if err := fs.Parse(argv); err != nil {
		return 2
	}

	log := stdLogger()

	if err := requireRoot(); err != nil {
		log.Printf("FATAL: %v", err)
		return 1
	}
	pu, err := user.Lookup(*panelUser)
	if err != nil {
		log.Printf("FATAL: panel kullanıcısı bulunamadı %q: %v", *panelUser, err)
		return 1
	}
	panelUID, err1 := strconv.Atoi(pu.Uid)
	panelGID, err2 := strconv.Atoi(pu.Gid)
	if err1 != nil || err2 != nil {
		log.Printf("FATAL: uid/gid çözümleme: %v %v", err1, err2)
		return 1
	}

	cfg := &runtimeCfg{
		panelUser:  *panelUser,
		panelUID:   uint32(panelUID),
		panelGID:   uint32(panelGID),
		quotaFS:    *quotaFS,
		opTimeout:  *opTimeout,
		sitesRoot:  "/srv/aurapanel/sites",
		stageDir:   "/var/lib/aurapanel/stage",
		nftDir:     "/etc/aurapanel/nftables",
		cgroupBase: "/sys/fs/cgroup/aurapanel",
	}

	pl, err := openPrivLog(*privLogPath, cfg.panelGID)
	if err != nil {
		log.Printf("FATAL: priv log: %v", err)
		return 1
	}
	defer pl.close()

	if *check {
		return runCheck(cfg, pl, *socketPath)
	}
	if err := serve(cfg, pl, *socketPath); err != nil {
		log.Printf("FATAL: %v", err)
		return 1
	}
	return 0
}

func stdLogger() *logpkg.Logger {
	return logpkg.New(os.Stderr, "aurapanel-priv: ", logpkg.LstdFlags)
}

// sdListenEnabled, systemd socket activation'ı algılar (sd_listen_fds
// protokolü): LISTEN_FDS=1 ve LISTEN_PID kendimiz ise fd 3 hazır dinleyen
// sokettir. LISTEN_PID boşsa kendi pid'imiz sayılır (sd_listen_fds davranışı).
func sdListenEnabled() bool {
	if runtime.GOOS == "windows" {
		return false
	}
	if os.Getenv("LISTEN_FDS") != "1" {
		return false
	}
	pid := os.Getenv("LISTEN_PID")
	return pid == "" || pid == strconv.Itoa(os.Getpid())
}

// openListener, dinleme soketini açar. systemd socket activation varsa soketi
// devralır (systemdListener — socket dosyası systemd'e aittir, dokunulmaz);
// yoksa dosyayı kendisi oluşturur (0660 root:panel-user).
func openListener(socketPath string, panelGID uint32) (net.Listener, error) {
	dir := filepath.Dir(socketPath)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("socket dizini: %w", err)
	}
	if runtime.GOOS != "windows" {
		os.Chown(dir, 0, int(panelGID))
	}
	if ln, ok := systemdListener(); ok {
		return ln, nil
	}
	if runtime.GOOS != "windows" {
		os.Remove(socketPath) // bayat socket dosyası
	}
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("dinleme %s: %w", socketPath, err)
	}
	if runtime.GOOS != "windows" {
		os.Chmod(socketPath, 0o660)
		os.Chown(socketPath, 0, int(panelGID))
	}
	return ln, nil
}

// serve, Unix socket'i açar ve istekleri kabul eder.
// Socket 0660 root:panel-user olarak oluşturulur; yalnızca panel kullanıcısı bağlanabilir.
func serve(cfg *runtimeCfg, pl *privLog, socketPath string) error {
	ln, err := openListener(socketPath, cfg.panelGID)
	if err != nil {
		return err
	}
	defer ln.Close()

	log := stdLogger()
	log.Printf("dinleniyor %s (panel kullanıcısı: %s uid=%d)", socketPath, cfg.panelUser, cfg.panelUID)

	reg := newRegistry(cfg)
	for {
		conn, err := ln.Accept()
		if err != nil {
			return fmt.Errorf("accept: %w", err)
		}
		go handleConn(cfg, reg, pl, conn)
	}
}

// handleConn, tek bağlantıyı işler: peer kimliği → sıkı decode → op dispatch →
// plan yürütme → yanıt. Her aşama priv log'a yazılır.
func handleConn(cfg *runtimeCfg, reg map[string]opFunc, pl *privLog, conn net.Conn) {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(requestTimeout))

	uid, pid, err := peerCred(conn)
	entry := map[string]any{
		"ts":       time.Now().UTC().Format(time.RFC3339Nano),
		"peer_uid": uid,
		"peer_pid": pid,
	}
	if err != nil {
		writeResponse(conn, Response{OK: false, Error: "peer kimliği alınamadı"})
		entry["result"] = "rejected"
		entry["error"] = err.Error()
		pl.write(entry)
		return
	}
	if uid != cfg.panelUID {
		writeResponse(conn, Response{OK: false, Error: "yetkisiz peer"})
		entry["result"] = "rejected"
		entry["error"] = "yetkisiz peer"
		pl.write(entry)
		return
	}

	req, err := decodeRequest(conn)
	if err != nil {
		writeResponse(conn, Response{OK: false, Error: err.Error()})
		entry["result"] = "rejected"
		entry["error"] = err.Error()
		pl.write(entry)
		return
	}
	entry["op"] = req.Op
	entry["request_id"] = req.RequestID

	// file.op: plan modeli dışında — worker spawn'ı (site UID + cgroup).
	if req.Op == "file.op" {
		opCtx, cancel := context.WithTimeout(context.Background(), cfg.opTimeout)
		data, err := runFileOp(opCtx, req.Args)
		cancel()
		if err != nil {
			writeResponse(conn, Response{OK: false, Error: err.Error(), RequestID: req.RequestID})
			entry["result"] = "failed"
			entry["error"] = err.Error()
			pl.write(entry)
			return
		}
		writeResponse(conn, Response{OK: true, Data: data, RequestID: req.RequestID})
		entry["result"] = "success"
		pl.write(entry)
		return
	}

	fn, ok := reg[req.Op]
	if !ok {
		writeResponse(conn, Response{OK: false, Error: "bilinmeyen op", RequestID: req.RequestID})
		entry["result"] = "rejected"
		entry["error"] = "bilinmeyen op"
		pl.write(entry)
		return
	}

	plan, data, err := fn(cfg, req.Args)
	if err != nil {
		writeResponse(conn, Response{OK: false, Error: err.Error(), RequestID: req.RequestID})
		entry["result"] = "failed"
		entry["error"] = err.Error()
		pl.write(entry)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.opTimeout)
	defer cancel()
	if err := executePlan(ctx, plan); err != nil {
		writeResponse(conn, Response{OK: false, Error: err.Error(), RequestID: req.RequestID})
		entry["result"] = "failed"
		entry["error"] = err.Error()
		pl.write(entry)
		return
	}

	writeResponse(conn, Response{OK: true, Data: data, RequestID: req.RequestID})
	entry["result"] = "success"
	pl.write(entry)
}

// runCheck, ortamı doğrular ve çıkar: root mu, binary'ler yerinde mi,
// socket dizini açılabilir mi, priv log yazılabilir mi.
func runCheck(cfg *runtimeCfg, pl *privLog, socketPath string) int {
	log := stdLogger()
	fails := 0

	for name, path := range binPaths {
		st, err := os.Stat(path)
		if err != nil || st.IsDir() {
			log.Printf("HATA: binary yok: %s (%s)", path, name)
			fails++
		}
	}

	dir := filepath.Dir(socketPath)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		log.Printf("HATA: socket dizini %s: %v", dir, err)
		fails++
	}

	if err := pl.write(map[string]any{
		"ts":     time.Now().UTC().Format(time.RFC3339Nano),
		"op":     "check",
		"result": "success",
	}); err != nil {
		log.Printf("HATA: priv log yazılamadı: %v", err)
		fails++
	}

	if fails > 0 {
		log.Printf("CHECK FAILED: %d sorun", fails)
		return 1
	}
	log.Printf("CHECK OK: panel_user=%s uid=%d socket=%s", cfg.panelUser, cfg.panelUID, socketPath)
	return 0
}
