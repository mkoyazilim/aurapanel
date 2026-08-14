package drift

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/mkoyazilim/aurapanel/internal/ols"
	"github.com/mkoyazilim/aurapanel/internal/site"
	"github.com/mkoyazilim/aurapanel/internal/store"
)

const (
	sevCritical = "critical"
	sevWarning  = "warning"
)

// desiredFor, site kaydından istek hâlini üretir. Docroot ve diğer
// türetilmiş değerler kullanıcıdan alınmaz — ols.RenderVhost üretir.
func desiredFor(st *store.Site, domains []store.Domain, phpVersion, sitesRoot, certsRoot string) (*desiredState, error) {
	aliases := []string{}
	for _, d := range domains {
		if d.Kind == "alias" {
			aliases = append(aliases, d.Domain)
		}
	}
	v := ols.Vhost{
		SiteID:     st.ID,
		Domain:     st.Name,
		Aliases:    aliases,
		PHPVersion: phpVersion,
		IndexFiles: []string{"index.php", "index.html"},
	}
	artifacts, err := ols.RenderVhost(sitesRoot, certsRoot, v)
	if err != nil {
		return nil, fmt.Errorf("desired vhost: %w", err)
	}
	if len(artifacts) != 2 {
		return nil, fmt.Errorf("desired vhost: beklenmeyen artifact sayısı %d", len(artifacts))
	}

	var limits site.Limits
	if err := json.Unmarshal([]byte(st.Limits), &limits); err != nil {
		return nil, fmt.Errorf("desired limits: %w", err)
	}

	// cgroup değerleri, priv cgroup.limits op'uyla AYNI biçimde üretilir
	// (karşılaştırma tutarlılığı).
	cpu := limits.CPUMax
	if cpu != "max" {
		cpu += " 100000"
	}
	cgroup := map[string]string{
		"cpu.max":     cpu,
		"memory.max":  strconv.FormatUint(limits.MemoryMax, 10),
		"memory.high": strconv.FormatUint(limits.MemoryHigh, 10),
		"pids.max":    strconv.FormatUint(limits.PIDsMax, 10),
	}

	return &desiredState{
		VhostContent: string(artifacts[1].Content),
		User:         st.LinuxUser,
		Cgroup:       cgroup,
		DiskBlocks:   limits.DiskMB * 1024, // setquota 1 KiB blok birimi
		Inodes:       limits.Inodes,
		Dirs: map[string]os.FileMode{
			"home": 0o750,
			"logs": 0o750,
			"tmp":  0o700,
		},
	}, nil
}

// diff, istek hâli ile gerçek durumu karşılaştırır ve sapmaları döndürür.
func diff(d *desiredState, act actuals) []finding {
	out := []finding{}

	// 1) OLS vhost: içerik sapması kritiktir (elle düzenleme, silinme...).
	if act.VhostContent != d.VhostContent {
		out = append(out, finding{
			Resource: "ols.vhost", Severity: sevCritical,
			Expected: fmt.Sprintf("%d bayt istek hâli", len(d.VhostContent)),
			Actual:   fmt.Sprintf("%d bayt gerçek içerik", len(act.VhostContent)),
		})
	}

	// 2) Linux kullanıcısı: yoksa kritik — izolasyon sınırı çökmüş demektir.
	if !act.UserExists {
		out = append(out, finding{
			Resource: "linux.user", Severity: sevCritical,
			Expected: d.User, Actual: "kullanıcı yok",
		})
	}

	// 3) cgroup limitleri.
	for _, k := range []string{"cpu.max", "memory.max", "memory.high", "pids.max"} {
		if act.Cgroup[k] != d.Cgroup[k] {
			out = append(out, finding{
				Resource: "cgroup." + k, Severity: sevWarning,
				Expected: d.Cgroup[k], Actual: act.Cgroup[k],
			})
		}
	}

	// 4) Quota: yalnızca quota ETKİNSE karşılaştırılır — etkin değilse bu
	//    bir kurulum sorunudur, drift değil (W13).
	if act.Quota.Available {
		if act.Quota.DiskBlocks != d.DiskBlocks {
			out = append(out, finding{
				Resource: "quota.disk", Severity: sevWarning,
				Expected: strconv.FormatUint(d.DiskBlocks, 10),
				Actual:   strconv.FormatUint(act.Quota.DiskBlocks, 10),
			})
		}
		if act.Quota.Inodes != d.Inodes {
			out = append(out, finding{
				Resource: "quota.inodes", Severity: sevWarning,
				Expected: strconv.FormatUint(d.Inodes, 10),
				Actual:   strconv.FormatUint(act.Quota.Inodes, 10),
			})
		}
	}

	// 5) Site dizinleri: varlık + mod.
	for _, name := range []string{"home", "logs", "tmp"} {
		wantMode := d.Dirs[name]
		got, ok := act.Dirs[name]
		switch {
		case !ok || !got.Exists:
			out = append(out, finding{
				Resource: "fs." + name, Severity: sevCritical,
				Expected: "dizin var", Actual: "dizin yok",
			})
		case got.Mode != wantMode:
			out = append(out, finding{
				Resource: "fs." + name, Severity: sevWarning,
				Expected: fmt.Sprintf("mod %o", wantMode), Actual: fmt.Sprintf("mod %o", got.Mode),
			})
		}
	}

	return out
}
