//go:build linux

package priv

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"syscall"
)

// runScriptOp, script.op'u yürütür: yardımcı kendini AYNI binary'nin worker
// modunda (AURAPANEL_SCRIPT_WORKER=1) başlatır.
func runScriptOp(ctx context.Context, raw json.RawMessage) (map[string]any, error) {
	args, content, err := validateScriptOp(raw)
	if err != nil {
		return nil, err
	}

	pu, err := user.Lookup("www-" + args.Site)
	if err != nil {
		return nil, fmt.Errorf("script.op: site kullanıcısı yok: %w", err)
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
	
	argv := []string{args.Site, args.Cwd}
	cmd := exec.CommandContext(ctx, self, argv...)
	cmd.Env = append(os.Environ(), "AURAPANEL_SCRIPT_WORKER=1")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)},
		Pdeathsig:  syscall.SIGTERM,
	}
	cmd.Stdin = bytes.NewReader(content)

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("script.op worker: %v: %s", err, truncateOutput(out.Bytes()))
	}

	var resp struct {
		OK    bool            `json:"ok"`
		Data  json.RawMessage `json:"data"`
		Error string          `json:"error"`
	}
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		return nil, fmt.Errorf("script.op yanıtı: %w", err)
	}
	if !resp.OK {
		return nil, fmt.Errorf("script.op: %s", resp.Error)
	}
	var data map[string]any
	if len(resp.Data) > 0 {
		if err := json.Unmarshal(resp.Data, &data); err != nil {
			return nil, err
		}
	}
	return data, nil
}

func ScriptWorkerMain(argv []string) int {
	worker := &fileWorker{} // ok/fail metodlarını tekrar kullanabiliriz
	if len(argv) < 2 {
		worker.fail("eksik argüman")
		return 1
	}
	siteID := argv[0]
	cwd := argv[1]

	worker.joinCgroup(siteID)

	// Yolu site köküne kısıtla (güvenlik)
	absCwd, err := worker.resolveRel(siteID, cwd)
	if err != nil {
		worker.fail("geçersiz cwd: " + err.Error())
		return 1
	}

	scriptBytes, err := ioReadAllLimit(os.Stdin, 4<<20)
	if err != nil {
		worker.fail("script okunamadı: " + err.Error())
		return 1
	}

	cmd := exec.Command("bash")
	cmd.Dir = absCwd
	cmd.Stdin = bytes.NewReader(scriptBytes)
	
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	err = cmd.Run()
	outputStr := out.String()
	
	if err != nil {
		worker.fail(fmt.Sprintf("Script hatası: %v\nÇıktı:\n%s", err, truncateOutput(out.Bytes())))
		return 1
	}
	
	worker.ok(map[string]any{"output": truncateOutput([]byte(outputStr))})
	return 0
}
