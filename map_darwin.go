// map_darwin.go - flags we need

//go:build darwin

package mmap

// Darwin doesn't have these; so we mark them zero
const (
	_MAP_HUGETLB  = 0
	_MAP_POPULATE = 0
)
