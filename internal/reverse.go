package internal

import (
	"encoding/binary"
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

const (
	RevMagic      = 0x56455244 // "DREV"
	RevHeaderSize = 16
	RevEntrySize  = 8
	RevGrowChunk  = 1 << 20
)

type revHeader struct {
	Magic    uint32
	_        uint32 // reserved
	Capacity uint64
}

type ReverseIndex struct {
	f        *os.File
	data     []byte
	header   *revHeader
	Capacity uint64
}

func OpenReverse(path string) (*ReverseIndex, error) {
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
		cap := uint64(RevGrowChunk)
		size := revFileSize(cap)
		if err := f.Truncate(int64(size)); err != nil {
			f.Close()
			return nil, err
		}
		rev, err := mmapReverse(f, cap)
		if err != nil {
			return nil, err
		}
		rev.header.Magic = RevMagic
		rev.header.Capacity = cap
		return rev, nil
	}
	var buf [RevHeaderSize]byte
	if _, err := f.ReadAt(buf[:], 0); err != nil {
		f.Close()
		return nil, err
	}
	magic := BO.Uint32(buf[0:4])
	if magic != RevMagic {
		f.Close()
		return nil, fmt.Errorf("dict: bad reverse magic %x", magic)
	}
	cap := BO.Uint64(buf[8:16])
	return mmapReverse(f, cap)
}

func mmapReverse(f *os.File, cap uint64) (*ReverseIndex, error) {
	size := revFileSize(cap)
	data, err := syscall.Mmap(int(f.Fd()), 0, int(size), syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("dict: mmap reverse: %w", err)
	}
	return &ReverseIndex{
		f:        f,
		data:     data,
		header:   (*revHeader)(unsafe.Pointer(&data[0])),
		Capacity: cap,
	}, nil
}

func revFileSize(cap uint64) int64 {
	return int64(RevHeaderSize) + int64(cap)*RevEntrySize
}

func (r *ReverseIndex) Set(id uint64, datOffset int64) error {
	if id >= r.Capacity {
		if err := r.grow(id); err != nil {
			return err
		}
	}
	off := RevHeaderSize + int(id)*RevEntrySize
	binary.LittleEndian.PutUint64(r.data[off:off+8], uint64(datOffset))
	return nil
}

func (r *ReverseIndex) Get(id uint64) (int64, error) {
	if id >= r.Capacity {
		return 0, fmt.Errorf("dict: reverse id %d out of range (cap %d)", id, r.Capacity)
	}
	off := RevHeaderSize + int(id)*RevEntrySize
	return int64(binary.LittleEndian.Uint64(r.data[off : off+8])), nil
}

func (r *ReverseIndex) grow(needID uint64) error {
	newCap := r.Capacity
	for needID >= newCap {
		newCap += RevGrowChunk
	}
	syscall.Munmap(r.data)
	r.data = nil

	size := revFileSize(newCap)
	if err := r.f.Truncate(int64(size)); err != nil {
		return err
	}
	data, err := syscall.Mmap(int(r.f.Fd()), 0, int(size), syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		return fmt.Errorf("dict: mmap reverse after grow: %w", err)
	}
	r.data = data
	r.header = (*revHeader)(unsafe.Pointer(&data[0]))
	r.header.Capacity = newCap
	r.Capacity = newCap
	return nil
}

func (r *ReverseIndex) Sync() error  { return Msync(r.data) }
func (r *ReverseIndex) Close() error { syscall.Munmap(r.data); return r.f.Close() }
