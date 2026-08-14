package ols

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"path"
	"strings"
	"text/template"
)

// Artifact, uygulanacak tek bir config dosyası.
type Artifact struct {
	RelPath string
	Content []byte
	Mode    os.FileMode
}

//go:embed templates/vhconf.conf.tmpl
var vhconfTmpl string

type vhostTemplateData struct {
	Docroot       string
	LogDir        string
	Domain        string
	AliasesLine   string
	IndexFiles    string
	LSAPIName     string
	LSAPISock     string
	ExtUser       string
	EnableGzip    int
	TLSRedirect   bool
	WAF           bool
	SSL           *SSLConfig
}

// RenderVhost, desired state'ten OLS native vhost (vhconf.conf) üretir.
// Çıktı deterministiktir (zaman damgası yok) — drift karşılaştırmasını
// ve testleri kararlı kılar.
//
// NOT: Şablon içeriği OLS 1.7+ native vhost formatındadır; sürüme özgü
// direktifler ilk entegrasyon testinde gerçek OLS'ye karşı doğrulanacak
// (ARCHITECTURE §5.2 uyumluluk matrisi).
func RenderVhost(sitesRoot, certsRoot string, v Vhost) ([]Artifact, error) {
	if err := v.Validate(sitesRoot, certsRoot); err != nil {
		return nil, err
	}

	phpDigits := strings.ReplaceAll(v.PHPVersion, ".", "")
	data := vhostTemplateData{
		Docroot:     path.Join(sitesRoot, v.SiteID, "home"),
		LogDir:      path.Join(sitesRoot, v.SiteID, "logs"),
		Domain:      v.Domain,
		AliasesLine: strings.Join(v.Aliases, ", "),
		IndexFiles:  strings.Join(v.IndexFiles, ", "),
		LSAPIName:   "lsphp" + phpDigits,
		LSAPISock:   "uds://tmp/lshttpd/lsphp" + phpDigits + ".sock",
		ExtUser:     "www-" + v.SiteID,
		EnableGzip:  boolToInt(v.EnableGzip),
		TLSRedirect: v.TLSRedirect,
		WAF:         v.WAF,
		SSL:         v.SSL,
	}

	tmpl, err := template.New("vhconf").Parse(vhconfTmpl)
	if err != nil {
		return nil, fmt.Errorf("şablon hatası: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("render hatası: %w", err)
	}

	return []Artifact{
		{RelPath: "vhconf.conf", Content: buf.Bytes(), Mode: 0o644},
	}, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
