// map_linux.go - flags we need

//go:build linux

package mmap

import (
	"fmt"
	"golang.org/x/sys/unix"
	"os"
	"unsafe"
)

const (
	_MAP_HUGETLB  = unix.MAP_HUGETLB
	_MAP_POPULATE = unix.MAP_POPULATE
)

func getBlockDevSize(fd *os.File) (int64, error) {
	var sz int64
	psz := uintptr(unsafe.Pointer(&sz))

	if _, _, err := unix.Syscall(unix.SYS_IOCTL, fd.Fd(), unix.BLKGETSIZE64, psz); err != 0 {
		return 0, fmt.Errorf("block device size: %s", err)
	}
	return sz, nil
}
