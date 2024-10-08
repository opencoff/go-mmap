// mmap_unix.go -- mmap for unix like systems
//
// (c) 2024- Sudhi Herle <sudhi@herle.net>
//
// Licensing Terms: GPLv2
//
// If you need a commercial license for this work, please contact
// the author.
//
// This software does not come with any express or implied
// warranty; it is provided "as is". No claim  is made to its
// suitability for any purpose.

//go:build darwin || linux || freebsd || openbsd || solaris || netbsd || dragonfly

package mmap

import (
	"fmt"
	"golang.org/x/sys/unix"
	"reflect"
	"unsafe"
)

func (m *Mmap) mmap(sz, off int64, prot Prot, flags Flag) (*Mapping, error) {
	mprot, mflag := convert(prot, flags)

	fd := m.fd.Fd()
	b, err := unix.Mmap(int(fd), off, int(sz), mprot, mflag)
	if err != nil {
		return nil, fmt.Errorf("%s: mmap %d at %d: %w", m.fd.Name(), sz, off, err)
	}

	p := &Mapping{
		buf: b,
		m:   m,
	}
	return p, nil
}

func (m *Mmap) map_anon(sz, off int64, prot Prot, flags Flag) (*Mapping, error) {
	mprot, mflag := convert(prot, flags)
	mflag |= unix.MAP_ANON

	b, err := unix.Mmap(-1, off, int(sz), mprot, mflag)
	if err != nil {
		return nil, fmt.Errorf("<anon>: mmap %d at %d: %w", sz, off, err)
	}

	p := &Mapping{
		buf: b,
		m:   m,
	}
	return p, nil
}

// convert canonical prot/flags to Unix specific ones
func convert(prot Prot, flags Flag) (mprot, mflag int) {
	mprot = unix.PROT_READ
	mflag = unix.MAP_SHARED | unix.MAP_FILE

	if prot&PROT_WRITE != 0 {
		mprot |= unix.PROT_WRITE

		// COW only makes sense for Writable mappings
		if flags&F_COW != 0 {
			mflag = unix.MAP_PRIVATE
		}
	}
	if prot&PROT_EXEC != 0 {
		mprot |= unix.PROT_EXEC
	}

	if flags&F_HUGETLB != 0 {
		mflag |= _MAP_HUGETLB
	}
	if flags&F_READAHEAD != 0 {
		mflag |= _MAP_POPULATE
	}
	return
}

type Mapping struct {
	buf []byte
	m   *Mmap
}

func (p *Mapping) addr() uintptr {
	sh := (*reflect.SliceHeader)(unsafe.Pointer(&p.buf))
	return sh.Data
}

func (p *Mapping) bytes() []byte {
	return p.buf
}

func (p *Mapping) lock() error {
	return unix.Mlock(p.buf)
}

func (p *Mapping) unlock() error {
	return unix.Munlock(p.buf)
}

func (p *Mapping) flush() error {
	var fd int = -1
	if p.m.fd != nil {
		fd = int(p.m.fd.Fd())
	}
	return unix.Msync(p.buf, fd)
}

func (p *Mapping) unmap() error {
	return unix.Munmap(p.buf)
}
