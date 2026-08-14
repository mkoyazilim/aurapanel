//go:build linux

package priv

import (
	"net"
	"os"
	"syscall"
)

// systemdListener, systemd socket activation'dan dinleyici soketi devralır.
// İki kanal denenir:
//  1. StandardInput=socket + Accept=no: systemd dinleyen soketi stdin'e
//     (fd 0) bağlar; bu modda LISTEN_FDS kurulmaz.
//  2. sd_listen_fds protokolü: LISTEN_FDS=1 + LISTEN_PID=kendimiz → fd 3.
//
// Hiçbiri yoksa (nil, false) döner — çağıran soketi kendisi oluşturur.
func systemdListener() (net.Listener, bool) {
	// Kanal 1: stdin bir dinleyen soket mi? (fstat + SO_ACCEPTCONN)
	if fi, err := os.Stdin.Stat(); err == nil && fi.Mode()&os.ModeSocket != 0 {
		if v, err := syscall.GetsockoptInt(0, syscall.SOL_SOCKET, syscall.SO_ACCEPTCONN); err == nil && v == 1 {
			if ln, ok := fileListener(0, "systemd-stdin"); ok {
				return ln, true
			}
		}
	}
	// Kanal 2: LISTEN_FDS=1 → fd 3.
	if sdListenEnabled() {
		if ln, ok := fileListener(3, "systemd-fd3"); ok {
			return ln, true
		}
	}
	return nil, false
}

// fileListener, fd'yi net.Listener'a sarar. FileListener fd'yi kopyaladığı
// için orijinal dosya tanıtıcısı hemen kapatılır (sızıntı ve çocuk süreçlere
// kalıtım engellenir).
func fileListener(fd uintptr, name string) (net.Listener, bool) {
	f := os.NewFile(fd, name)
	ln, err := net.FileListener(f)
	if err != nil {
		f.Close()
		return nil, false
	}
	f.Close()
	return ln, true
}
