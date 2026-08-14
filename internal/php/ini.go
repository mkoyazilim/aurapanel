package php

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// php.ini yönerge allowlist'i — değerler türüne göre doğrulanır.
// open_basedir, disable_functions, error_log vb. BİLİNÇLİ OLARAK burada
// YOKTUR: bunlar güvenlik profilleri ve panel tarafından yönetilir
// (ARCHITECTURE §9-10; kullanıcı izolasyonu kendi kendine gevşetemez).
var (
	reSize   = regexp.MustCompile(`^([0-9]+)([KMG])$`)
	sizeKeys = map[string]int{ // anahtar → max birim (ör. 1024 = 1024M)
		"upload_max_filesize": 1024,
		"post_max_size":       1024,
		"memory_limit":        1024,
	}
	intKeys = map[string]int{ // anahtar → max değer
		"max_execution_time":    86400,
		"max_input_vars":        100000,
		"session.gc_maxlifetime": 86400,
	}
	boolKeys = map[string]bool{
		"display_errors": true, "log_errors": true, "expose_php": true, "allow_url_fopen": true,
	}
)

// ValidateSettings, yönergeleri doğrular ve normalize eder.
// Dönüş yalnızca GEÇERLİ ve normalize edilmiş anahtarları içerir.
func ValidateSettings(settings map[string]string) (map[string]string, error) {
	out := map[string]string{}
	for k, v := range settings {
		switch {
		case sizeKeys[k] > 0:
			m := reSize.FindStringSubmatch(v)
			if m == nil {
				return nil, fmt.Errorf("%s: geçersiz boyut %q (ör. 64M, 2G)", k, v)
			}
			n, _ := strconv.Atoi(m[1])
			if n < 1 || n > sizeKeys[k] {
				return nil, fmt.Errorf("%s: aralık dışı (1..%d%s)", k, sizeKeys[k], m[2])
			}
			out[k] = v
		case intKeys[k] > 0:
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 || n > intKeys[k] {
				return nil, fmt.Errorf("%s: 1..%d aralığında tam sayı olmalı", k, intKeys[k])
			}
			out[k] = strconv.Itoa(n)
		case k == "date.timezone":
			if _, err := time.LoadLocation(v); err != nil {
				return nil, fmt.Errorf("date.timezone: geçersiz bölge %q", v)
			}
			out[k] = v
		case boolKeys[k]:
			switch strings.ToLower(v) {
			case "1", "on", "true":
				out[k] = "On"
			case "0", "off", "false":
				out[k] = "Off"
			default:
				return nil, fmt.Errorf("%s: On/Off olmalı", k)
			}
		default:
			return nil, fmt.Errorf("izin verilmeyen php.ini yönergesi: %s", k)
		}
	}
	if len(out) == 0 {
		return nil, errors.New("en az bir yönerge gerekli")
	}
	return out, nil
}

// RenderIni, normalize ayarları deterministik php.ini metnine dönüştürür
// (anahtarlar sıralı — drift karşılaştırması kararlı).
func RenderIni(settings map[string]string) string {
	keys := make([]string, 0, len(settings))
	for k := range settings {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	b.WriteString("# AuraPanel yönetimli php.ini — elle düzenlemeyin.\n")
	for _, k := range keys {
		fmt.Fprintf(&b, "%s = %s\n", k, settings[k])
	}
	return b.String()
}
