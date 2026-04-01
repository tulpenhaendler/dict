package dict

import (
	"encoding/binary"
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

// reverseIndex is an mmap'd fixed-width array mapping id → dat_offset.
//
// File layout:
//   Header (16 bytes)
//   entries[capacity] × 8 bytes (uint64 dat offsets)
const (
	revMagic      = 0x56455244 // "DREV" little-endian
	revHeaderSize = 16
	revEntrySize  = 8
	revGrowChunk  = 1 << 20 // grow by 1M entries at a time (8MB)
)

type revHeader struct {
	Magic    uint32
	Capacity uint32
	_        [8]byte
}

type reverseIndex struct {
	f        *os.File
	data     []byte
	header   *revHeader
	capacity uint32
}

func openReverse(path string) (*reverseIndex, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	if info.Size() == 0 {
		cap := uint32(revGrowChunk)
		size := revFileSize(cap)
		if err := f.Truncate(int64(size)); err != nil {
			f.Close()
			return nil, err
		}
		rev, err := mmapReverse(f, cap)
		if err != nil {
			return nil, err
		}
		rev.header.Magic = revMagic
		rev.header.Capacity = cap
		return rev, nil
	}
	var buf [revHeaderSize]byte
	if _, err := f.ReadAt(buf[:], 0); err != nil {
		f.Close()
		return nil, err
	}
	magic := bo.Uint32(buf[0:4])
	if magic != revMagic {
		f.Close()
		return nil, fmt.Errorf("dict: bad reverse magic %x", magic)
	}
	cap := bo.Uint32(buf[4:8])
	return mmapReverse(f, cap)
}

func mmapReverse(f *os.File, cap uint32) (*reverseIndex, error) {
	size := revFileSize(cap)
	data, err := syscall.Mmap(int(f.Fd()), 0, size, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("dict: mmap reverse: %w", err)
	}
	return &reverseIndex{
		f:        f,
		data:     data,
		header:   (*revHeader)(unsafe.Pointer(&data[0])),
		capacity: cap,
	}, nil
}

func revFileSize(cap uint32) int {
	return revHeaderSize + int(cap)*revEntrySize
}

func (r *reverseIndex) set(id uint32, datOffset int64) error {
	if id >= r.capacity {
		if err := r.grow(id); err != nil {
			return err
		}
	}
	off := revHeaderSize + int(id)*revEntrySize
	binary.LittleEndian.PutUint64(r.data[off:off+8], uint64(datOffset))
	return nil
}

func (r *reverseIndex) get(id uint32) (int64, error) {
	if id >= r.capacity {
		return 0, fmt.Errorf("dict: reverse id %d out of range (cap %d)", id, r.capacity)
	}
	off := revHeaderSize + int(id)*revEntrySize
	return int64(binary.LittleEndian.Uint64(r.data[off : off+8])), nil
}

func (r *reverseIndex) grow(needID uint32) error {
	newCap := r.capacity
	for needID >= newCap {
		newCap += revGrowChunk
	}
	// unmap, resize, remap
	syscall.Munmap(r.data)
	r.data = nil

	size := revFileSize(newCap)
	if err := r.f.Truncate(int64(size)); err != nil {
		return err
	}
	data, err := syscall.Mmap(int(r.f.Fd()), 0, size, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		return fmt.Errorf("dict: mmap reverse after grow: %w", err)
	}
	r.data = data
	r.header = (*revHeader)(unsafe.Pointer(&data[0]))
	r.header.Capacity = newCap
	r.capacity = newCap
	return nil
}

func (r *reverseIndex) sync() error  { return msync(r.data) }
func (r *reverseIndex) close() error { syscall.Munmap(r.data); return r.f.Close() }
