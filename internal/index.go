package internal

import (
	"encoding/binary"
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"github.com/tulpenhaendler/dict/codec"
)

const (
	IdxMagic      = 0x58444944 // "DIDX"
	IdxVersion    = 3          // v3: per-type uint64 IDs
	idxMaxTypes   = 80         // must be >= codec.MaxKeyType
	IdxHeaderSize = 1024
	SlotSize      = 16 // id(8) + datOffset(8)
	InitialSlots  = 1 << 16
	MaxLoadNum    = 7
	MaxLoadDen    = 10
)

// Compile-time check that idxMaxTypes covers all codec key types.
var _ [idxMaxTypes - int(codec.MaxKeyType)]byte

type IdxHeader struct {
	Magic       uint32
	Version     uint32
	SlotCount   uint32
	LiveEntries uint32
	NextIDs     [idxMaxTypes]uint64
	_           [368]byte // pad to IdxHeaderSize (1024)
}

type HashIndex struct {
	f         *os.File
	data      []byte
	Header    *IdxHeader
	ctrl      []byte
	slotBase  uintptr
	SlotCount uint32
}

func CreateIndex(path string, slotCount uint32) (*HashIndex, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return nil, err
	}
	size := IdxFileSize(slotCount)
	if err := f.Truncate(int64(size)); err != nil {
		f.Close()
		return nil, err
	}
	return mmapIndex(f, slotCount)
}

func OpenIndex(path string) (*HashIndex, error) {
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
		return createFreshIndex(f, InitialSlots)
	}

	// Read magic and version to detect incompatible formats.
	var peek [8]byte
	if _, err := f.ReadAt(peek[:], 0); err != nil {
		f.Close()
		return nil, err
	}
	magic := BO.Uint32(peek[0:4])
	version := BO.Uint32(peek[4:8])
	if magic != IdxMagic || version != IdxVersion {
		// Incompatible format; recreate (data log will drive rebuild).
		f.Close()
		return CreateIndex(path, InitialSlots)
	}

	var hbuf [16]byte
	if _, err := f.ReadAt(hbuf[:], 0); err != nil {
		f.Close()
		return nil, err
	}
	slotCount := BO.Uint32(hbuf[8:12])
	return mmapIndex(f, slotCount)
}

func createFreshIndex(f *os.File, slotCount uint32) (*HashIndex, error) {
	size := IdxFileSize(slotCount)
	if err := f.Truncate(int64(size)); err != nil {
		f.Close()
		return nil, err
	}
	idx, err := mmapIndex(f, slotCount)
	if err != nil {
		return nil, err
	}
	idx.Header.Magic = IdxMagic
	idx.Header.Version = IdxVersion
	idx.Header.SlotCount = slotCount
	return idx, nil
}

func mmapIndex(f *os.File, slotCount uint32) (*HashIndex, error) {
	size := IdxFileSize(slotCount)
	data, err := syscall.Mmap(int(f.Fd()), 0, size, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("dict: mmap index: %w", err)
	}
	return &HashIndex{
		f:         f,
		data:      data,
		Header:    (*IdxHeader)(unsafe.Pointer(&data[0])),
		ctrl:      data[IdxHeaderSize : IdxHeaderSize+int(slotCount)],
		slotBase:  uintptr(unsafe.Pointer(&data[IdxHeaderSize+int(slotCount)])),
		SlotCount: slotCount,
	}, nil
}

func IdxFileSize(slotCount uint32) int {
	return IdxHeaderSize + int(slotCount) + int(slotCount)*SlotSize
}

func (ix *HashIndex) Lookup(h uint32, cb byte, match func(int64) bool) (uint64, bool) {
	mask := ix.SlotCount - 1
	pos := h & mask
	for {
		c := ix.ctrl[pos]
		if c == 0x00 {
			return 0, false
		}
		if c == cb {
			id, off := ix.ReadSlot(pos)
			if match(int64(off)) {
				return id, true
			}
		}
		pos = (pos + 1) & mask
	}
}

func (ix *HashIndex) Insert(h uint32, cb byte, id uint64, datOffset int64) {
	mask := ix.SlotCount - 1
	pos := h & mask
	for {
		if ix.ctrl[pos] == 0x00 {
			ix.ctrl[pos] = cb
			ix.WriteSlot(pos, id, uint64(datOffset))
			ix.Header.LiveEntries++
			return
		}
		pos = (pos + 1) & mask
	}
}

func (ix *HashIndex) NeedsGrow() bool {
	return uint64(ix.Header.LiveEntries)*MaxLoadDen >= uint64(ix.SlotCount)*MaxLoadNum
}

// Grow rebuilds the index with double the slot count. The caller provides
// a rebuild function that re-inserts all entries into the new index.
func (ix *HashIndex) Grow(rebuild func(*HashIndex) error) error {
	newCount := ix.SlotCount * 2
	path := ix.f.Name()
	tmpPath := path + ".tmp"

	newIdx, err := CreateIndex(tmpPath, newCount)
	if err != nil {
		return err
	}
	newIdx.Header.Magic = IdxMagic
	newIdx.Header.Version = IdxVersion
	newIdx.Header.SlotCount = newCount
	newIdx.Header.NextIDs = ix.Header.NextIDs
	newIdx.Header.LiveEntries = 0

	if err := rebuild(newIdx); err != nil {
		newIdx.Close()
		os.Remove(tmpPath)
		return err
	}

	if err := newIdx.Sync(); err != nil {
		newIdx.Close()
		os.Remove(tmpPath)
		return err
	}

	oldF := ix.f
	ix.munmap()
	oldF.Close()

	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}

	*ix = *newIdx
	return nil
}

func (ix *HashIndex) ReadSlot(pos uint32) (id uint64, datOffset uint64) {
	base := ix.slotBase + uintptr(pos)*SlotSize
	slot := unsafe.Slice((*byte)(unsafe.Pointer(base)), SlotSize)
	id = binary.LittleEndian.Uint64(slot[0:8])
	datOffset = binary.LittleEndian.Uint64(slot[8:16])
	return
}

func (ix *HashIndex) WriteSlot(pos uint32, id uint64, datOffset uint64) {
	base := ix.slotBase + uintptr(pos)*SlotSize
	slot := unsafe.Slice((*byte)(unsafe.Pointer(base)), SlotSize)
	binary.LittleEndian.PutUint64(slot[0:8], id)
	binary.LittleEndian.PutUint64(slot[8:16], datOffset)
}

func Msync(b []byte) error {
	_, _, errno := syscall.Syscall(syscall.SYS_MSYNC, uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)), syscall.MS_SYNC)
	if errno != 0 {
		return errno
	}
	return nil
}

func (ix *HashIndex) Sync() error {
	return Msync(ix.data)
}

func (ix *HashIndex) munmap() {
	if ix.data != nil {
		syscall.Munmap(ix.data)
		ix.data = nil
	}
}

func (ix *HashIndex) Close() error {
	ix.munmap()
	return ix.f.Close()
}
