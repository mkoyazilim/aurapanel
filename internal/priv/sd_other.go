//go:build !linux

package priv

import "net"

// systemdListener: yalnızca Linux'ta anlamlı (systemd). Diğer platformlarda
// helper soketi her zaman kendisi oluşturur.
func systemdListener() (net.Listener, bool) {
	return nil, false
}
