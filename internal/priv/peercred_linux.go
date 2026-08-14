//go:build linux

package priv

import (
	"errors"
	"net"

	"golang.org/x/sys/unix"
)

// peerCred, bağlanan sürecin UID/PID'ini SO_PEERCRED ile okur
// (ARCHITECTURE §3.1: sokete yalnızca panel kullanıcısı bağlanabilir).
func peerCred(conn net.Conn) (uid, pid uint32, err error) {
	uc, ok := conn.(*net.UnixConn)
	if !ok {
		return 0, 0, errors.New("unix bağlantısı değil")
	}
	raw, err := uc.SyscallConn()
	if err != nil {
		return 0, 0, err
	}
	var ucred *unix.Ucred
	var sockErr error
	if err := raw.Control(func(fd uintptr) {
		ucred, sockErr = unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
	}); err != nil {
		return 0, 0, err
	}
	if sockErr != nil {
		return 0, 0, sockErr
	}
	return ucred.Uid, uint32(ucred.Pid), nil
}
