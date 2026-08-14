package priv

import (
	"bytes"
	"testing"
)

// DoD (ROADMAP W2): "Rastgele JSON hiçbir beklenmeyen komut çalıştıramaz."

// FuzzDecodeRequest: rastgele girdilerde decode asla panic atmaz;
// başarılı decode her zaman boş olmayan op içerir.
func FuzzDecodeRequest(f *testing.F) {
	f.Add([]byte(`{"op":"priv.ping","args":{}}`))
	f.Add([]byte(`{"op":"user.create","args":{"name":"www-site001","home":"/srv/aurapanel/sites/site001/home"}}`))
	f.Add([]byte(`{"op":"cgroup.limits","args":{"site":"site001","cpu_max":"max"}}`))
	f.Add([]byte(`{`))
	f.Add([]byte(``))
	f.Add([]byte(`{"op":"x","args":{},}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		req, err := decodeRequest(bytes.NewReader(data))
		if err != nil {
			return
		}
		if req.Op == "" {
			t.Fatal("decode başarılı ama op boş")
		}
	})
}

// FuzzOpDispatch: bilinen op'lar rastgele argümanlarla plan üretirken
// asla panic atmaz; üretilen her plan bin allowlist'ine uygun kalır.
func FuzzOpDispatch(f *testing.F) {
	f.Add([]byte(`{"op":"user.create","args":{"name":"www-site001","home":"/srv/aurapanel/sites/site001/home"}}`))
	f.Add([]byte(`{"op":"quota.set","args":{"user":"www-site001","disk_mb":5120}}`))
	f.Add([]byte(`{"op":"firewall.apply","args":{"ruleset":"/etc/aurapanel/nftables/rules.nft"}}`))
	f.Add([]byte(`{"op":"user.create","args":{"name":"BAD","home":"/etc/passwd","shell":"/bin/bash"}}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		req, err := decodeRequest(bytes.NewReader(data))
		if err != nil {
			return
		}
		cfg := testCfg()
		fn, ok := newRegistry(cfg)[req.Op]
		if !ok {
			return // bilinmeyen op: handleConn reddeder
		}
		p, _, err := fn(cfg, req.Args)
		if err != nil {
			return
		}
		assertPlanBins(t, p)
	})
}
