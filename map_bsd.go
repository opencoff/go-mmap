// map_bsd.go - flags we need

//go:build freebsd || openbsd || netbsd || dragonflybsd

package mmap

// Darwin doesn't have these; so we mark them zero
const (
	_MAP_HUGETLB  = 0
	_MAP_POPULATE = 0
)
