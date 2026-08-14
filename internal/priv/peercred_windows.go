//go:build windows

package priv

import (
	"errors"
	"net"
)

func peerCred(conn net.Conn) (uid, pid uint32, err error) {
	return 0, 0, errors.New("SO_PEERCRED yalnızca Linux'ta desteklenir")
}
