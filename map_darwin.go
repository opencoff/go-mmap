// map_darwin.go - flags we need

//go:build darwin

package mmap

import (
	"fmt"
	"golang.org/x/sys/unix"
	"math/bits"
	"os"
)

// Darwin doesn't have these; so we mark them zero
const (
	_MAP_HUGETLB  = 0
	_MAP_POPULATE = 0

	_DKIOCGETBLOCKSIZE  = 0x40046418
	_DKIOCGETBLOCKCOUNT = 0x40086419
)

func getBlockDevSize(fd *os.File) (int64, error) {
	d := int(fd.Fd())

	blksz, err := unix.IoctlGetInt(d, _DKIOCGETBLOCKSIZE)
	if err != nil {
		return 0, fmt.Errorf("block size: %s", err)
	}

	nblks, err := unix.IoctlGetInt(d, _DKIOCGETBLOCKCOUNT)
	if err != nil {
		return 0, fmt.Errorf("block count: %s", err)
	}

	hi, lo := bits.Mul64(uint64(blksz), uint64(nblks))
	if hi > 0 {
		return 0, fmt.Errorf("size overflow (nblks %d, blksz %d)", nblks, blksz)
	}

	return int64(lo), nil
}
