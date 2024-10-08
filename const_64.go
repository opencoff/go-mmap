// const_64.go -- constants for 64-bit archs
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

//go:build amd64 || arm64 || mips64 || ppc64

package mmap

const _MaxMmapSize int64 = 1024 * 1024 * 1048576
