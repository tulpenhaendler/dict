package internal

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/tulpenhaendler/dict/codec"
)

// DataLog is an append-only file of encoded key entries.
//
// Entry format:
//
//	[1] type_tag
//	[1] encoded_len (uint8)
//	[N] encoded_key_bytes
type DataLog struct {
	f    *os.File
	Size int64
}

var bufPool = sync.Pool{
	New: func() any {
		b := make([]byte, 257)
		return &b
	},
}

func OpenDataLog(path string) (*DataLog, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	return &DataLog{f: f, Size: info.Size()}, nil
}

func (d *DataLog) Append(keyType codec.KeyType, encoded []byte) (int64, error) {
	n := len(encoded)
	if n > 255 {
		return 0, fmt.Errorf("encoded key too long: %d bytes (max 255)", n)
	}
	offset := d.Size

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

	d.Size = offset + int64(2+n)
	return offset, nil
}

func (d *DataLog) MatchEntry(offset int64, keyType codec.KeyType, encoded []byte) bool {
	bp := bufPool.Get().(*[]byte)
	buf := (*bp)[:2]
	_, err := d.f.ReadAt(buf, offset)
	if err != nil || codec.KeyType(buf[0]) != keyType {
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

func (d *DataLog) ReadEntry(offset int64) (codec.KeyType, []byte, error) {
	var header [2]byte
	if _, err := d.f.ReadAt(header[:], offset); err != nil {
		return 0, nil, err
	}
	keyType := codec.KeyType(header[0])
	keyLen := int(header[1])
	buf := make([]byte, keyLen)
	if keyLen > 0 {
		if _, err := d.f.ReadAt(buf, offset+2); err != nil {
			return 0, nil, err
		}
	}
	return keyType, buf, nil
}

func (d *DataLog) Iterate(fn func(offset int64, keyType codec.KeyType, encoded []byte) error) error {
	if d.Size == 0 {
		return nil
	}
	all := make([]byte, d.Size)
	if _, err := d.f.ReadAt(all, 0); err != nil && err != io.EOF {
		return err
	}

	offset := int64(0)
	for offset+2 <= d.Size {
		keyType := codec.KeyType(all[offset])
		keyLen := int(all[offset+1])
		end := offset + 2 + int64(keyLen)
		if end > d.Size {
			d.Size = offset
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

func (d *DataLog) Sync() error  { return d.f.Sync() }
func (d *DataLog) Close() error { return d.f.Close() }

// HashKey computes a hash from type tag + encoded bytes using FNV-1a.
func HashKey(keyType codec.KeyType, encoded []byte) uint32 {
	h := uint32(2166136261)
	h ^= uint32(keyType)
	h *= 16777619
	for _, b := range encoded {
		h ^= uint32(b)
		h *= 16777619
	}
	if h == 0 {
		h = 1
	}
	return h
}

// HashKeyString hashes directly from a string without converting to []byte.
func HashKeyString(keyType codec.KeyType, s string) uint32 {
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

// CtrlByte derives the 1-byte fingerprint stored in the ctrl array.
func CtrlByte(h uint32) byte {
	return byte(h>>25) | 0x80
}

var BO = binary.LittleEndian
