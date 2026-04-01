package dict

import (
	"encoding/binary"
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

// hashIndex is a split-metadata hash table backed by an mmap'd file.
//
// File layout:
//   Header     (64 bytes)
//   ctrl[]     (slotCount × 1 byte)  — fingerprint per slot
//   slots[]    (slotCount × 12 bytes) — id(4) + datOffset(8)
//
// ctrl byte: 0x00 = empty, otherwise (hash>>25)|0x80.
const (
	idxMagic      = 0x58444944 // "DIDX" little-endian
	idxVersion    = 1
	idxHeaderSize = 64
	slotSize      = 12
	initialSlots  = 1 << 16 // 65536
	maxLoadNum    = 7
	maxLoadDen    = 10
)

type idxHeader struct {
	Magic       uint32
	Version     uint32
	SlotCount   uint32
	LiveEntries uint32
	NextID      uint32
	_           [44]byte
}

type hashIndex struct {
	f         *os.File
	data      []byte // full mmap
	header    *idxHeader
	ctrl      []byte   // points into data
	slotBase  uintptr  // start of slot array in data
	slotCount uint32
}

func createIndex(path string, slotCount uint32) (*hashIndex, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return nil, err
	}
	size := idxFileSize(slotCount)
	if err := f.Truncate(int64(size)); err != nil {
		f.Close()
		return nil, err
	}
	return mmapIndex(f, slotCount)
}

func openIndex(path string) (*hashIndex, error) {
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
		// new file — initialise
		size := idxFileSize(initialSlots)
		if err := f.Truncate(int64(size)); err != nil {
			f.Close()
			return nil, err
		}
		idx, err := mmapIndex(f, initialSlots)
		if err != nil {
			return nil, err
		}
		idx.header.Magic = idxMagic
		idx.header.Version = idxVersion
		idx.header.SlotCount = initialSlots
		return idx, nil
	}
	// read slot count from header
	var hdr idxHeader
	var buf [idxHeaderSize]byte
	if _, err := f.ReadAt(buf[:], 0); err != nil {
		f.Close()
		return nil, err
	}
	hdr.Magic = bo.Uint32(buf[0:4])
	hdr.Version = bo.Uint32(buf[4:8])
	hdr.SlotCount = bo.Uint32(buf[8:12])
	if hdr.Magic != idxMagic {
		f.Close()
		return nil, fmt.Errorf("dict: bad index magic %x", hdr.Magic)
	}
	return mmapIndex(f, hdr.SlotCount)
}

func mmapIndex(f *os.File, slotCount uint32) (*hashIndex, error) {
	size := idxFileSize(slotCount)
	data, err := syscall.Mmap(int(f.Fd()), 0, size, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("dict: mmap index: %w", err)
	}
	idx := &hashIndex{
		f:         f,
		data:      data,
		header:    (*idxHeader)(unsafe.Pointer(&data[0])),
		ctrl:      data[idxHeaderSize : idxHeaderSize+int(slotCount)],
		slotBase:  uintptr(unsafe.Pointer(&data[idxHeaderSize+int(slotCount)])),
		slotCount: slotCount,
	}
	return idx, nil
}

func idxFileSize(slotCount uint32) int {
	return idxHeaderSize + int(slotCount) + int(slotCount)*slotSize
}

// lookup probes the hash table for a key. Returns (id, true) if found.
// The caller must verify the actual key bytes via the data log.
// found is set only when the encoded key matches.
func (ix *hashIndex) lookup(h uint32, cb byte, keyType KeyType, encoded []byte, dat *dataLog) (uint32, bool) {
	mask := ix.slotCount - 1
	pos := h & mask
	for {
		c := ix.ctrl[pos]
		if c == 0x00 {
			return 0, false
		}
		if c == cb {
			id, off := ix.readSlot(pos)
			kt, enc, err := dat.readEntry(int64(off))
			if err == nil && kt == keyType && bytesEqual(enc, encoded) {
				return id, true
			}
		}
		pos = (pos + 1) & mask
	}
}

// insert writes a new entry into the first empty slot in the probe chain.
func (ix *hashIndex) insert(h uint32, cb byte, id uint32, datOffset int64) {
	mask := ix.slotCount - 1
	pos := h & mask
	for {
		if ix.ctrl[pos] == 0x00 {
			ix.ctrl[pos] = cb
			ix.writeSlot(pos, id, uint64(datOffset))
			ix.header.LiveEntries++
			return
		}
		pos = (pos + 1) & mask
	}
}

func (ix *hashIndex) needsGrow() bool {
	return uint64(ix.header.LiveEntries)*maxLoadDen >= uint64(ix.slotCount)*maxLoadNum
}

// grow rebuilds the index with double the slot count. Reads all entries from dat.
func (ix *hashIndex) grow(dat *dataLog) error {
	newCount := ix.slotCount * 2
	path := ix.f.Name()
	tmpPath := path + ".tmp"

	newIdx, err := createIndex(tmpPath, newCount)
	if err != nil {
		return err
	}
	newIdx.header.Magic = idxMagic
	newIdx.header.Version = idxVersion
	newIdx.header.SlotCount = newCount
	newIdx.header.NextID = ix.header.NextID
	newIdx.header.LiveEntries = 0

	// rehash all live slots — we already have hash info, but need to recompute
	// from the ctrl byte and position. Simpler: scan dat and reinsert.
	nextID := uint32(0)
	err = dat.iterate(func(offset int64, keyType KeyType, encoded []byte) error {
		h := hashKey(keyType, encoded)
		cb := ctrlByte(h)
		newIdx.insert(h, cb, nextID, offset)
		nextID++
		return nil
	})
	if err != nil {
		newIdx.close()
		os.Remove(tmpPath)
		return err
	}

	if err := newIdx.sync(); err != nil {
		newIdx.close()
		os.Remove(tmpPath)
		return err
	}

	// swap
	oldF := ix.f
	ix.munmap()
	oldF.Close()

	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}

	*ix = *newIdx
	return nil
}

func (ix *hashIndex) readSlot(pos uint32) (id uint32, datOffset uint64) {
	base := ix.slotBase + uintptr(pos)*slotSize
	slot := unsafe.Slice((*byte)(unsafe.Pointer(base)), slotSize)
	id = binary.LittleEndian.Uint32(slot[0:4])
	datOffset = binary.LittleEndian.Uint64(slot[4:12])
	return
}

func (ix *hashIndex) writeSlot(pos uint32, id uint32, datOffset uint64) {
	base := ix.slotBase + uintptr(pos)*slotSize
	slot := unsafe.Slice((*byte)(unsafe.Pointer(base)), slotSize)
	binary.LittleEndian.PutUint32(slot[0:4], id)
	binary.LittleEndian.PutUint64(slot[4:12], datOffset)
}

func msync(b []byte) error {
	_, _, errno := syscall.Syscall(syscall.SYS_MSYNC, uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)), syscall.MS_SYNC)
	if errno != 0 {
		return errno
	}
	return nil
}

func (ix *hashIndex) sync() error {
	return msync(ix.data)
}

func (ix *hashIndex) munmap() {
	if ix.data != nil {
		syscall.Munmap(ix.data)
		ix.data = nil
	}
}

func (ix *hashIndex) close() error {
	ix.munmap()
	return ix.f.Close()
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
