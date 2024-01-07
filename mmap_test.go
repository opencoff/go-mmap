// mmap_test.go - tests for mmap-go
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

package mmap_test

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/opencoff/go-mmap"
)

var _PAGE int64 = int64(os.Getpagesize())

func xTestRead(t *testing.T) {
	assert := newAsserter(t)

	var sz int64 = 3*_PAGE + (_PAGE / 3)
	pages := randData(sz)
	fname := tmpName(t)
	err := createFile(fname, pages)
	assert(err == nil, "create %s: %s", fname, err)

	fd, err := os.Open(fname)
	assert(err == nil, "open %s: %s", fname, err)

	defer fd.Close()

	m := mmap.New(fd)
	assert(m != nil, "create-mmap %s: nil ptr", fname)

	// first map the whole file
	p, err := m.Map(sz, 0, mmap.PROT_READ, 0)
	assert(err == nil, "mmap: %s: %s", fname, err)

	mapped := p.Bytes()
	for i := range pages {
		pg := &pages[i]
		n := len(pg.buf)
		mm := mapped[pg.off:]

		assert(bytes.Equal(pg.buf, mm[:n]), "mmap at %d: content mismatch", pg.off)
	}

	p.Unmap()
}

func TestWrite(t *testing.T) {
	assert := newAsserter(t)

	fname := tmpName(t)

	var sz int64 = 2*_PAGE + (_PAGE / 3)

	orig := randData(sz)

	err := createFile(fname, orig)
	assert(err == nil, "create %s: %s", fname, err)

	pages := randData(sz)

	fd, err := os.OpenFile(fname, os.O_CREATE|os.O_RDWR, 0600)
	assert(err == nil, "creat %s: %s", fname, err)

	m := mmap.New(fd)

	p, err := m.Map(sz, 0, mmap.PROT_READ|mmap.PROT_WRITE, 0)
	assert(err == nil, "mmap: %s: %s", fname, err)
	assert(p != nil, "mmap: %s: nil ptr", fname)

	mapped := p.Bytes()
	assert(len(mapped) == int(sz), "mmap: len exp %d, saw %d", sz, len(mapped))

	// see if we can write to these pages
	for i := range pages {
		pg := &pages[i]
		n := len(pg.buf)
		fmt.Printf("Pg %2d: off %d, len %d\n", i, pg.off, len(pg.buf))
		mm := mapped[pg.off:]

		m := copy(mm, pg.buf)
		assert(m == n, "wr at %d: exp %d, saw %d", pg.off, n, m)
	}

	p.Flush()
	p.Unmap()
	fd.Close()

	fd, err = os.Open(fname)
	assert(err == nil, "open %s: %s", fname, err)

	pgbuf := make([]byte, _PAGE)
	for i := range pages {
		pg := &pages[i]
		n := len(pg.buf)
		m, err := fd.Read(pgbuf[:n])
		assert(err == nil, "read %s: %s", fname, err)
		assert(m == n, "read %s: exp %d, saw %d",
			fname, n, m)

		assert(bytes.Equal(pg.buf, pgbuf[:n]), "mmap at %d: content mismatch", pg.off)
	}
	fd.Close()
}

func TestReader(t *testing.T) {
	assert := newAsserter(t)

	fname := tmpName(t)

	var sz int64 = 3*_PAGE + (_PAGE / 3)

	orig := randData(sz)
	osum := cksum(orig)

	err := createFile(fname, orig)
	assert(err == nil, "create %s: %s", fname, err)

	fd, err := os.Open(fname)
	assert(err == nil, "open: %s: %s", fname, err)

	// now calculate same checksum via the reader interface
	h := sha256.New()
	n, err := mmap.Reader(fd, func(b []byte) error {
		h.Write(b)
		return nil
	})
	assert(err == nil, "reader: %s: %s", fname, err)
	assert(n == sz, "reader %s: size exp %d, saw %d", fname, sz, n)

	nsum := h.Sum(nil)[:]

	assert(bytes.Equal(osum, nsum), "mmap: %s: content mismatch", fname)
	fd.Close()
}

func TestCOW(t *testing.T) {
	assert := newAsserter(t)

	fname := tmpName(t)

	var sz int64 = 3*_PAGE + (_PAGE / 3)

	orig := randData(sz)

	err := createFile(fname, orig)
	assert(err == nil, "create %s: %s", fname, err)

	fd, err := os.OpenFile(fname, os.O_RDWR, 0600)
	assert(err == nil, "open %s: %s", fname, err)

	m := mmap.New(fd)

	p, err := m.Map(sz, 0, mmap.PROT_READ|mmap.PROT_WRITE, mmap.F_COW)
	assert(err == nil, "mmap: %s: %s", fname, err)
	assert(p != nil, "mmap: %s: nil ptr", fname)

	mapped := p.Bytes()
	assert(len(mapped) == int(sz), "mmap: len exp %d, saw %d", sz, len(mapped))

	// mutate the contents
	pages := randData(sz)
	out := mapped[:]
	for i := range pages {
		pg := &pages[i]
		n := copy(out, pg.buf)
		out = out[n:]
	}
	p.Unmap()
	fd.Close()

	// now read the file back and see if the contents are same as orig
	fd, err = os.Open(fname)
	m = mmap.New(fd)
	assert(err == nil, "mmap: open %s: %s", fname, err)
	for i := range orig {
		pg := &orig[i]
		p, err := m.Map(int64(len(pg.buf)), pg.off, mmap.PROT_READ, 0)
		assert(err == nil, "mmap %d at %d: %s", len(pg.buf), pg.off, err)
		assert(p != nil, "mmap: %s: nil ptr", fname)
		assert(bytes.Equal(p.Bytes(), pg.buf), "mmap: %d at %d content mismatch", len(pg.buf), pg.off)
		p.Unmap()
	}
	fd.Close()
}

// Create a file that is sz bytes big
func createFile(nm string, d []data) error {
	fd, err := os.OpenFile(nm, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}

	defer func() {
		fd.Sync()
		fd.Close()
	}()

	for i := range d {
		pg := &d[i]
		m, err := fd.Write(pg.buf)
		if err != nil {
			return err
		}
		if len(pg.buf) != m {
			return fmt.Errorf("%s: partial write; exp %d, saw %d",
				nm, len(pg.buf), m)
		}
	}

	return nil
}

type data struct {
	off int64
	buf []byte
}

func randData(sz int64) []data {
	var pages []data
	var off int64

	for sz > 0 {
		page := make([]byte, _PAGE)
		n := len(page)
		if int64(n) > sz {
			n = int(sz)
		}
		rand.Read(page[:n])
		d := data{
			off: off,
			buf: page[:n],
		}
		pages = append(pages, d)

		sz -= int64(n)
		off += int64(n)
	}
	return pages
}

// sha256 of the data pages
func cksum(d []data) []byte {
	h := sha256.New()
	for i := range d {
		p := &d[i]
		h.Write(p.buf)
	}
	return h.Sum(nil)[:]
}

func tmpName(t *testing.T) string {
	dn := t.TempDir()
	bn := fmt.Sprintf("tmp%d-%x", os.Getpid(), randU32())
	return filepath.Join(dn, bn)
}

func randU32() uint32 {
	var b [4]byte

	_, err := io.ReadFull(rand.Reader, b[:])
	if err != nil {
		panic(fmt.Sprintf("can't read 4 rand bytes: %s", err))
	}

	return binary.LittleEndian.Uint32(b[:])
}
