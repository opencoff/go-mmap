// mmap_windows.go -- mmap abstraction for windows
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

//go:build windows

package mmap

import (
	"fmt"
	"golang.org/x/sys/windows"
	"os"
	"reflect"
	"unsafe"
)

type Mapping struct {
	ptr     uintptr
	sz      uintptr
	mapping windows.Handle
	wr      bool
	m       *Mmap
}

func (m *Mmap) mmap(sz, off int64, prot Prot, flags Flag) (*Mapping, error) {
	mflag, macc := convert(prot, flags)

	fd := windows.Handle(m.fd.Fd())
	p, err := m.do_mmap(fd, sz, off, mflag, macc)
	if err == nil {
		p.wr = prot&PROT_WRITE != 0
	}
	return p, err
}

func (m *Mmap) map_anon(sz, off int64, prot Prot, flags Flag) (*Mapping, error) {
	mflag, macc := convert(prot, flags)

	// These two flags are ONLY available for anon mappings.
	mflag |= _SEC_RESERVE
	if flags&F_HUGETLB != 0 {
		mflag |= _SEC_LARGE_PAGES
	}

	fd := windows.Handle(^uintptr(0))
	p, err := m.do_mmap(fd, sz, off, mflag, macc)
	if err == nil {
		p.wr = prot&PROT_WRITE != 0
	}
	return p, err
}

func (m *Mmap) do_mmap(fd windows.Handle, sz, off int64, mflag, macc uint32) (*Mapping, error) {
	maxSz := uint64(sz) + uint64(off)
	maxH := uint32(maxSz >> 32)
	maxL := uint32(maxSz & 0xffffffff)

	h, err := windows.CreateFileMapping(fd, nil, mflag, maxH, maxL, nil)
	if h == 0 {
		return nil, fmt.Errorf("%s: mmap %d at %d: %w",
			m.fd.Name(), sz, off, os.NewSyscallError("CreateFileMapping", err))
	}

	// now map into memory
	offH := uint32(uint64(off) >> 32)
	offL := uint32(uint64(off) & 0xffffffff)
	addr, err := windows.MapViewOfFile(h, macc, offH, offL, uintptr(sz))
	if addr == 0 {
		return nil, fmt.Errorf("%s: mmap %d at %d: %w",
			m.fd.Name(), sz, off, os.NewSyscallError("MapViewOfFile", err))
	}

	p := &Mapping{
		ptr:     addr,
		sz:      uintptr(sz),
		mapping: h,
		m:       m,
	}
	return p, nil
}

func (p *Mapping) addr() uintptr {
	return p.ptr
}

func (p *Mapping) bytes() []byte {
	var b []byte
	sh := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	sh.Data = p.ptr
	sh.Len = int(p.sz)
	sh.Cap = int(p.sz)
	return b
}

func (p *Mapping) lock() error {
	err := windows.VirtualLock(p.ptr, uintptr(p.sz))
	if err != nil {
		return fmt.Errorf("VirtualLock %x: (%d bytes): %w",
			p.ptr, p.sz, os.NewSyscallError("VirtualLock", err))
	}
	return nil
}

func (p *Mapping) unlock() error {
	err := windows.VirtualUnlock(p.ptr, uintptr(p.sz))
	if err != nil {
		return fmt.Errorf("VirtualUnlock %x: (%d bytes): %w",
			p.ptr, p.sz, os.NewSyscallError("VirtualUnlock", err))
	}
	return nil
}

func (p *Mapping) flush() error {
	// This is a complex dance on Windows :(
	err := windows.FlushViewOfFile(p.ptr, uintptr(p.sz))
	if err != nil {
		return fmt.Errorf("VirtualUnlock %x: (%d bytes): %w",
			p.ptr, p.sz, os.NewSyscallError("VirtualUnlock", err))
	}

	h := windows.Handle(p.m.fd.Fd())
	if p.wr && h != windows.Handle(^uintptr(0)) {
		if err = windows.FlushFileBuffers(h); err != nil {
			return fmt.Errorf("flush %x: (%d bytes): %w",
				p.ptr, p.sz, os.NewSyscallError("VirtualUnlock", err))
		}
	}
	return nil
}

func (p *Mapping) unmap() error {
	err := p.flush()
	if err != nil {
		return err
	}

	err = windows.UnmapViewOfFile(p.ptr)
	if err != nil {
		return fmt.Errorf("unmap %x: (%d bytes): %w",
			p.ptr, p.sz, os.NewSyscallError("UnmapViewOfFile", err))
	}

	err = windows.CloseHandle(p.mapping)
	if err != nil {
		return fmt.Errorf("unmap %x: (%d bytes): %w",
			p.ptr, p.sz, os.NewSyscallError("CloseHandle", err))
	}
	return nil
}

// Missing constants in sys/windows
const (
	_SEC_LARGE_PAGES uint32 = 0x80000000
	_SEC_RESERVE     uint32 = 0x4000000
)

func convert(prot Prot, flags Flag) (mflag, macc uint32) {
	// Windows is weird:
	//   - mflag is used by CreateFileMapping() and is mostly not a bitfield
	//   - macc is used by MapViewOfFile() and _IS_ a bitfield
	//
	// Also we observe that windows has separate _mirroed_ flags for EXEC;
	// ie, READONLY, READWRITE, WRITECOPY are mirrored for the EXEC
	// variants as well - except shifted left by 4 bits.
	//
	// So, we will first figure out the non exec variants of mflag

	mflag = windows.PAGE_READONLY
	macc = windows.FILE_MAP_READ
	if prot&PROT_WRITE != 0 {
		if flags&F_COW != 0 {
			macc |= windows.FILE_MAP_COPY
			mflag = windows.PAGE_WRITECOPY
		} else {
			macc |= windows.FILE_MAP_WRITE
			mflag = windows.PAGE_READWRITE
		}
	} else {
		macc |= windows.FILE_MAP_READ
	}

	if prot&PROT_EXEC != 0 {
		macc |= windows.FILE_MAP_EXECUTE

		// the exec bits are left shifted-by 4 and _mirror_ the PAGE_READWRITE, _WRITECOPY
		// buts.
		mflag <<= 4
	}

	// NB: Windows doesn't support LARGE_PAGES for non-anon mappings!
	return
}
