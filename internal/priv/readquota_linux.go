//go:build linux

package priv

import (
	"syscall"
	"unsafe"
)

// qGetQuota: Linux <sys/quota.h> içindeki Q_GETQUOTA komut kodu.
const qGetQuota = 0x800007

// dqblk, modern (64-bit alanlı, kernel >= 2.4.22) disk quota bloğu —
// 8 × uint64, 64 bayt. x/sys/unix Linux için quota yapılarını
// sağlamadığından yerel tanımdır; düzen çekirdek ABI'siyle birebirdir.
type dqblk struct {
	bhardlimit uint64
	bsoftlimit uint64
	curspace   uint64
	ihardlimit uint64
	isoftlimit uint64
	curinodes  uint64
	btime      uint64
	itime      uint64
}

// readQuota, Q_GETQUOTA ile kullanıcının hard limitlerini (1 KiB blok ve
// inode) okur. Quota etkin değilse errno döner — çağıran op bunu
// "available:false" olarak raporlar (drift değil, kurulum sorunu).
func readQuota(fsPath string, uid uint32) (blocks, inodes uint64, err error) {
	p, err := syscall.BytePtrFromString(fsPath)
	if err != nil {
		return 0, 0, err
	}
	var dq dqblk
	_, _, errno := syscall.Syscall6(syscall.SYS_QUOTACTL,
		qGetQuota,
		uintptr(unsafe.Pointer(p)),
		uintptr(uid),
		uintptr(unsafe.Pointer(&dq)),
		0, 0)
	if errno != 0 {
		return 0, 0, errno
	}
	return dq.bhardlimit, dq.ihardlimit, nil
}
