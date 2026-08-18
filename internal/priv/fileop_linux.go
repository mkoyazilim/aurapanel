//go:build linux

package priv

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// runFileOp, file.op'u yürütür: yardımcı kendini AYNI binary'nin worker
// modunda, site kullanıcısının kimliğiyle (setuid) yeniden başlatır.
// Worker, site cgroup'una kendini ekler ve doğrulanmış işlemi yapar.
func runFileOp(ctx context.Context, raw json.RawMessage) (map[string]any, error) {
	args, content, err := validateFileOp(raw)
	if err != nil {
		return nil, err
	}

	// Site kullanıcısının UID/GID'i (değişmez: www-<siteID>).
	pu, err := user.Lookup("www-" + args.Site)
	if err != nil {
		return nil, fmt.Errorf("file.op: site kullanıcısı yok: %w", err)
	}
	uid, err1 := strconv.Atoi(pu.Uid)
	gid, err2 := strconv.Atoi(pu.Gid)
	if err1 != nil || err2 != nil {
		return nil, errors2(err1, err2)
	}

	self, err := os.Executable()
	if err != nil {
		return nil, err
	}
	// Worker modu ENV ile tetiklenir — argv[0] fiildir. read parçalı akış
	// için offset/limit'i, write ekleme için offset'i konum argümanı taşır.
	argv := append([]string{args.Verb, args.Site}, args.Paths...)
	if args.Verb == "read" || args.Verb == "write" {
		argv = append(argv, strconv.FormatInt(args.Offset, 10))
	}
	if args.Verb == "read" {
		argv = append(argv, strconv.FormatInt(args.Limit, 10))
	}
	cmd := exec.CommandContext(ctx, self, argv...)
	cmd.Env = append(os.Environ(), "AURAPANEL_FILE_WORKER=1")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)},
		// Çocuk, yardımcı ölürse ölsün (yetim işlem kalmasın).
		Pdeathsig: syscall.SIGTERM,
	}
	if args.Verb == "write" {
		cmd.Stdin = bytes.NewReader(content)
	}

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("file.op worker: %v: %s", err, truncateOutput(out.Bytes()))
	}

	var resp struct {
		OK    bool            `json:"ok"`
		Data  json.RawMessage `json:"data"`
		Error string          `json:"error"`
	}
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		return nil, fmt.Errorf("file.op yanıtı: %w", err)
	}
	if !resp.OK {
		return nil, fmt.Errorf("file.op: %s", resp.Error)
	}
	var data map[string]any
	if len(resp.Data) > 0 {
		if err := json.Unmarshal(resp.Data, &data); err != nil {
			return nil, err
		}
	}
	return data, nil
}

func errors2(e1, e2 error) error {
	if e1 != nil {
		return e1
	}
	return e2
}

// WorkerMain, worker modunun dışa açılan giriş noktası.
func WorkerMain(argv []string) int { return fileWorkerMain(argv) }

// fileWorkerMain, worker modunun giriş noktası (site UID'siyle çalışır).
// Çıktı: tek satır JSON {ok, data?, error?}.
func fileWorkerMain(argv []string) int {
	worker := &fileWorker{}
	if len(argv) < 2 {
		worker.fail("eksik argüman")
		return 1
	}
	verb, siteID := argv[0], argv[1]
	paths := argv[2:]

	// Site cgroup'una katıl (delegasyon sayesinde site kullanıcısı yazabilir).
	worker.joinCgroup(siteID)

	var data map[string]any
	var err error
	switch verb {
	case "list":
		data, err = worker.doList(siteID, paths[0])
	case "read":
		var off, lim int64
		if len(paths) > 1 {
			off, _ = strconv.ParseInt(paths[1], 10, 64)
		}
		if len(paths) > 2 {
			lim, _ = strconv.ParseInt(paths[2], 10, 64)
		}
		data, err = worker.doRead(siteID, paths[0], off, lim)
	case "write":
		var off int64
		if len(paths) > 1 {
			off, _ = strconv.ParseInt(paths[1], 10, 64)
		}
		data, err = worker.doWrite(siteID, paths[0], off)
	case "mkdir":
		data, err = worker.doMkdir(siteID, paths[0])
	case "rename":
		data, err = worker.doRename(siteID, paths[0], paths[1])
	case "remove":
		data, err = worker.doRemove(siteID, paths[0])
	case "stat":
		data, err = worker.doStat(siteID, paths[0])
	case "symlink":
		data, err = worker.doSymlink(siteID, paths[0], paths[1])
	case "eval":
		data, err = worker.doEval(siteID, paths[0])
	default:
		err = fmt.Errorf("bilinmeyen fiil: %q", verb)
	}
	if err != nil {
		worker.fail(err.Error())
		return 1
	}
	worker.ok(data)
	return 0
}

type fileWorker struct{}

func (w *fileWorker) ok(data map[string]any) {
	json.NewEncoder(os.Stdout).Encode(map[string]any{"ok": true, "data": data})
}

func (w *fileWorker) fail(msg string) {
	json.NewEncoder(os.Stdout).Encode(map[string]any{"ok": false, "error": msg})
}

// joinCgroup, kendi PID'ini site cgroup'una ekler (best-effort: cgroup
// kurulmamışsa dosya işlemi yine UID kimliğiyle güvenlidir).
func (w *fileWorker) joinCgroup(siteID string) {
	pid := strconv.Itoa(os.Getpid())
	procs := fmt.Sprintf("/sys/fs/cgroup/aurapanel/sites/%s/cgroup.procs", siteID)
	os.WriteFile(procs, []byte(pid), 0o644)
}

// resolveRel, worker'ın savunma katmanı: yolu site root içinde KANITLAR
// (panel fm.resolve ile aynı sözleşme — bilinçli çift katman).
func (w *fileWorker) resolveRel(siteID, rel string) (string, error) {
	home := "/srv/aurapanel/sites/" + siteID + "/home"
	return resolveWorker(evalSymlinks, home, rel)
}

// resolveWorker, normalize → temizle (".." reddi) → join → kanonik
// (symlink'ler) → site root doğrulaması.
func resolveWorker(canon func(string) (string, error), siteHome, rel string) (string, error) {
	for _, r := range rel {
		if r == 0 || r < 0x20 {
			return "", errors.New("geçersiz yol")
		}
	}
	if path.IsAbs(rel) {
		return "", errors.New("geçersiz yol")
	}
	clean := path.Clean(rel)
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", errors.New("geçersiz yol")
	}
	full := path.Join(siteHome, clean)
	resolved, err := canon(full)
	if err != nil {
		// Hedef yok: en yakın var olan atadan çöz.
		missing := ""
		cur := full
		for {
			if _, err2 := canon(cur); err2 == nil {
				break
			}
			parent := path.Dir(cur)
			if parent == cur {
				return "", errors.New("yol çözümlenemedi")
			}
			missing = path.Join(path.Base(cur), missing)
			cur = parent
		}
		base, err := canon(cur)
		if err != nil {
			return "", err
		}
		resolved = path.Join(base, path.Clean(missing))
	}
	root := path.Clean(siteHome)
	if resolved != root && !strings.HasPrefix(resolved, root+"/") {
		return "", errors.New("site root dışına erişim engellendi")
	}
	return resolved, nil
}

func evalSymlinks(p string) (string, error) {
	resolved, err := filepathEval(p)
	if err != nil {
		return "", err
	}
	return resolved, nil
}

func (w *fileWorker) doList(siteID, rel string) (map[string]any, error) {
	abs, err := w.resolveRel(siteID, rel)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return nil, err
	}
	out := []map[string]any{}
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, map[string]any{
			"name": e.Name(), "size": info.Size(), "mode": int64(info.Mode()),
			"is_dir": info.IsDir(), "mtime": info.ModTime().UTC().Format(time.RFC3339Nano),
		})
	}
	return map[string]any{"entries": out}, nil
}

func (w *fileWorker) doRead(siteID, rel string, offset, limit int64) (map[string]any, error) {
	abs, err := w.resolveRel(siteID, rel)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(abs)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if offset > 0 {
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			return nil, err
		}
	}
	if limit <= 0 || limit > fileOpContentLimit {
		limit = fileOpContentLimit
	}
	b, err := io.ReadAll(io.LimitReader(f, limit))
	if err != nil {
		return nil, err
	}
	// Boş parça = dosya sonu (akış okuyucusu EOF olarak yorumlar).
	return map[string]any{"content_b64": b64Encode(b)}, nil
}

func (w *fileWorker) doWrite(siteID, rel string, offset int64) (map[string]any, error) {
	abs, err := w.resolveRel(siteID, rel)
	if err != nil {
		return nil, err
	}
	b, err := ioReadAllLimit(os.Stdin, fileOpContentLimit)
	if err != nil {
		return nil, err
	}
	if offset > 0 {
		// Ekleme modu (parçalı yükleme): mevcut dosyaya offset'ten yaz.
		f, err := os.OpenFile(abs, os.O_WRONLY, 0)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			return nil, err
		}
		if _, err := f.Write(b); err != nil {
			return nil, err
		}
		return map[string]any{"size": len(b)}, nil
	}
	if err := atomicWrite(abs, b, 0o644); err != nil {
		return nil, err
	}
	return map[string]any{"size": len(b)}, nil
}

func (w *fileWorker) doMkdir(siteID, rel string) (map[string]any, error) {
	abs, err := w.resolveRel(siteID, rel)
	if err != nil {
		return nil, err
	}
	return nil, os.MkdirAll(abs, 0o755)
}

func (w *fileWorker) doRename(siteID, from, to string) (map[string]any, error) {
	absFrom, err := w.resolveRel(siteID, from)
	if err != nil {
		return nil, err
	}
	absTo, err := w.resolveRel(siteID, to)
	if err != nil {
		return nil, err
	}
	return nil, os.Rename(absFrom, absTo)
}

func (w *fileWorker) doRemove(siteID, rel string) (map[string]any, error) {
	abs, err := w.resolveRel(siteID, rel)
	if err != nil {
		return nil, err
	}
	home := "/srv/aurapanel/sites/" + siteID + "/home"
	if abs == home {
		return nil, fmt.Errorf("site root silinemez")
	}
	return nil, os.RemoveAll(abs)
}

func (w *fileWorker) doStat(siteID, rel string) (map[string]any, error) {
	abs, err := w.resolveRel(siteID, rel)
	if err != nil {
		return nil, err
	}
	st, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"name": st.Name(), "size": st.Size(), "mode": int64(st.Mode()),
		"is_dir": st.IsDir(), "mtime": st.ModTime().UTC().Format(time.RFC3339Nano),
	}, nil
}

func (w *fileWorker) doSymlink(siteID, target, link string) (map[string]any, error) {
	absTarget, err := w.resolveRel(siteID, target)
	if err != nil {
		return nil, err
	}
	absLink, err := w.resolveRel(siteID, link)
	if err != nil {
		return nil, err
	}
	return nil, os.Symlink(absTarget, absLink)
}

func (w *fileWorker) doEval(siteID, rel string) (map[string]any, error) {
	abs, err := w.resolveRel(siteID, rel)
	if err != nil {
		return nil, err
	}
	return map[string]any{"path": abs}, nil
}
