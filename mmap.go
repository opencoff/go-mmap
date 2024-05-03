// mmap.go - portable RW/RO mmap abstractions
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

// Package mmap provides an OS independent interface for memory mapped files
package mmap

import (
	"fmt"
	"os"
)

// Prot describes the protections for a mapping
type Prot uint

const (
	PROT_READ Prot = 1 << iota
	PROT_WRITE
	PROT_EXEC
)

// Flag describes additional properties for a given mapping
type Flag uint

const (
	F_COW Flag = 1 << iota
	F_HUGETLB
	F_READAHEAD
)

// Mmap describes mappings for a file backed object
type Mmap struct {
	fd *os.File
}

// New creates a new memory map object for the given file. It is a
// runtime error for a file to be opened in RO mode while asking for
// a PROT_WRITE mapping.
func New(fd *os.File) *Mmap {
	m := &Mmap{
		fd: fd,
	}
	return m
}

// NewAnon creates a mmemory map object suitable for anon mappings.
func NewAnon() *Mmap {
	m := &Mmap{
		fd: nil,
	}
	return m
}

// Map creates a memory mapping at offset 'off' for 'sz' bytes.
func (m *Mmap) Map(sz, off int64, prot Prot, flags Flag) (*Mapping, error) {
	if m.fd == nil {
		p, err := m.map_anon(sz, off, prot, flags)
		return p, err
	}

	st, err := m.fd.Stat()
	if err != nil {
		return nil, fmt.Errorf("mmap %d at %d: %w", sz, off, err)
	}

	if !st.Mode().IsRegular() {
		return nil, fmt.Errorf("mmap %d at %d: not a regular file", sz, off)
	}

	fsz := st.Size()
	if fsz == 0 {
		return nil, fmt.Errorf("mmap %d at %d: empty file", sz, off)
	}

	if sz <= 0 {
		sz = fsz
	}

	if sz > fsz || (sz+off) > fsz {
		return nil, fmt.Errorf("mmap %d at %d: out of bounds", sz, off)
	}

	if sz > _MaxMmapSize {
		return nil, fmt.Errorf("mmap %d at %d: too large", sz, off)
	}

	p, err := m.mmap(sz, off, prot, flags)
	return p, err
}

// Unmap unmaps a given mapping
func (m *Mmap) Unmap(p *Mapping) error {
	return p.unmap()
}

// Bytes returns a byte slice corresponding to the mapping
func (p *Mapping) Bytes() []byte {
	return p.bytes()
}

// Flush flushes any changes to the backing disk (or swap for anon mappings)
func (p *Mapping) Flush() error {
	return p.flush()
}

// Lock locks the given mappings in memory (prevents page out)
func (p *Mapping) Lock() error {
	return p.lock()
}

// Unlock unlocks the given mappings (enable page out as needed)
func (p *Mapping) Unlock() error {
	return p.unlock()
}

// Unmap unmaps the given mapping
func (p *Mapping) Unmap() error {
	return p.unmap()
}

// Reader mmap's chunks of the file and calls the given closure
// with successive chunks of the file contents until EOF. If the
// closure returns non-nil error, it breaks the iteration and the
// error is propogated back to the caller.
// Reader returns the number of bytes of read.
func Reader(fd *os.File, fp func(buf []byte) error) (int64, error) {
	st, err := fd.Stat()
	if err != nil {
		return 0, fmt.Errorf("mmap: %w", err)
	}

	var off, z, fsz int64

	m := New(fd)
	fsz = st.Size()
	for fsz > 0 {
		sz := fsz
		if sz > _MaxMmapSize {
			sz = _MaxMmapSize
		}

		p, err := m.mmap(sz, off, PROT_READ, F_READAHEAD)
		if err != nil {
			return 0, err
		}

		err = fp(p.bytes())
		if err != nil {
			return z, err
		}

		p.unmap()

		off += sz
		z += sz
		fsz -= sz
	}
	return z, nil
}
