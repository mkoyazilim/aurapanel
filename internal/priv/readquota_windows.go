//go:build windows

package priv

import "errors"

func readQuota(fsPath string, uid uint32) (blocks, inodes uint64, err error) {
	return 0, 0, errors.New("quota yalnızca Linux'ta okunabilir")
}
