package dict

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"sync"
)

// dataLog is an append-only file of encoded key entries.
//
// Entry format:
//   [1] type_tag
//   [1] encoded_len (uint8)
//   [N] encoded_key_bytes
type dataLog struct {
	f    *os.File
	size int64
}

// pool for read buffers — avoids allocation on every lookup verification.
// Max encoded key is 255 bytes + 2 byte header = 257.
var bufPool = sync.Pool{
	New: func() any {
		b := make([]byte, 257)
		return &b
	},
}

func openDataLog(path string) (*dataLog, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	return &dataLog{f: f, size: info.Size()}, nil
}

// append writes an entry and returns the offset where it was written.
// Uses a single WriteAt call to reduce syscalls.
func (d *dataLog) append(keyType KeyType, encoded []byte) (int64, error) {
	n := len(encoded)
	if n > 255 {
		return 0, fmt.Errorf("encoded key too long: %d bytes (max 255)", n)
	}
	offset := d.size

	// write header + body in one syscall
	bp := bufPool.Get().(*[]byte)
	buf := (*bp)[:2+n]
	buf[0] = byte(keyType)
	buf[1] = byte(n)
	copy(buf[2:], encoded)
	_, err := d.f.WriteAt(buf, offset)
	bufPool.Put(bp)
	if err != nil {
		return 0, err
	}

	d.size = offset + int64(2+n)
	return offset, nil
}

// matchEntry reads the entry at offset and returns true if it matches keyType and encoded.
// Zero-allocation on the hot path via sync.Pool.
func (d *dataLog) matchEntry(offset int64, keyType KeyType, encoded []byte) bool {
	bp := bufPool.Get().(*[]byte)
	buf := (*bp)[:2]
	_, err := d.f.ReadAt(buf, offset)
	if err != nil || KeyType(buf[0]) != keyType {
		bufPool.Put(bp)
		return false
	}
	kLen := int(buf[1])
	if kLen != len(encoded) {
		bufPool.Put(bp)
		return false
	}
	if kLen == 0 {
		bufPool.Put(bp)
		return true
	}
	buf = (*bp)[:kLen]
	_, err = d.f.ReadAt(buf, offset+2)
	if err != nil {
		bufPool.Put(bp)
		return false
	}
	eq := bytes.Equal(buf, encoded)
	bufPool.Put(bp)
	return eq
}

// readEntry reads the entry at the given offset, returning type and encoded key bytes.
// Allocates — use matchEntry on the hot lookup path instead.
func (d *dataLog) readEntry(offset int64) (KeyType, []byte, error) {
	var header [2]byte
	if _, err := d.f.ReadAt(header[:], offset); err != nil {
		return 0, nil, err
	}
	keyType := KeyType(header[0])
	keyLen := int(header[1])
	buf := make([]byte, keyLen)
	if keyLen > 0 {
		if _, err := d.f.ReadAt(buf, offset+2); err != nil {
			return 0, nil, err
		}
	}
	return keyType, buf, nil
}

// iterate calls fn for every valid entry in the log. Used to rebuild the index.
func (d *dataLog) iterate(fn func(offset int64, keyType KeyType, encoded []byte) error) error {
	// read entire file into memory for fast sequential scan
	if d.size == 0 {
		return nil
	}
	all := make([]byte, d.size)
	if _, err := d.f.ReadAt(all, 0); err != nil && err != io.EOF {
		return err
	}

	offset := int64(0)
	for offset+2 <= d.size {
		keyType := KeyType(all[offset])
		keyLen := int(all[offset+1])
		end := offset + 2 + int64(keyLen)
		if end > d.size {
			// truncated entry — discard
			d.size = offset
			if err := d.f.Truncate(offset); err != nil {
				return err
			}
			break
		}
		if err := fn(offset, keyType, all[offset+2:end]); err != nil {
			return err
		}
		offset = end
	}
	return nil
}

func (d *dataLog) sync() error  { return d.f.Sync() }
func (d *dataLog) close() error { return d.f.Close() }

// hashKey computes a hash from type tag + encoded bytes using FNV-1a.
func hashKey(keyType KeyType, encoded []byte) uint32 {
	h := uint32(2166136261)
	h ^= uint32(keyType)
	h *= 16777619
	for _, b := range encoded {
		h ^= uint32(b)
		h *= 16777619
	}
	if h == 0 {
		h = 1 // 0 is reserved for empty ctrl slots
	}
	return h
}

// hashKeyString hashes directly from a string without converting to []byte.
// Used for raw keys to avoid allocation.
func hashKeyString(keyType KeyType, s string) uint32 {
	h := uint32(2166136261)
	h ^= uint32(keyType)
	h *= 16777619
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= 16777619
	}
	if h == 0 {
		h = 1
	}
	return h
}

// ctrlByte derives the 1-byte fingerprint stored in the ctrl array.
func ctrlByte(h uint32) byte {
	return byte(h>>25) | 0x80
}

var bo = binary.LittleEndian
