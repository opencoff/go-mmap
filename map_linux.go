// map_linux.go - flags we need

//go:build linux

package mmap

import (
	"golang.org/x/sys/unix"
)

const (
	_MAP_HUGETLB  = unix.MAP_HUGETLB
	_MAP_POPULATE = unix.MAP_POPULATE
)
