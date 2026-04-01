package dict

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
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
func (d *dataLog) append(keyType KeyType, encoded []byte) (int64, error) {
	if len(encoded) > 255 {
		return 0, fmt.Errorf("encoded key too long: %d bytes (max 255)", len(encoded))
	}
	offset := d.size
	header := [2]byte{byte(keyType), byte(len(encoded))}
	if _, err := d.f.WriteAt(header[:], offset); err != nil {
		return 0, err
	}
	if _, err := d.f.WriteAt(encoded, offset+2); err != nil {
		return 0, err
	}
	d.size = offset + 2 + int64(len(encoded))
	return offset, nil
}

// readEntry reads the entry at the given offset, returning type, encoded key bytes.
func (d *dataLog) readEntry(offset int64) (KeyType, []byte, error) {
	var header [2]byte
	if _, err := d.f.ReadAt(header[:], offset); err != nil {
		return 0, nil, err
	}
	keyType := KeyType(header[0])
	keyLen := int(header[1])
	buf := make([]byte, keyLen)
	if _, err := d.f.ReadAt(buf, offset+2); err != nil {
		return 0, nil, err
	}
	return keyType, buf, nil
}

// entrySize returns the total byte size of an entry at the given offset.
func (d *dataLog) entrySize(offset int64) (int64, error) {
	var header [2]byte
	if _, err := d.f.ReadAt(header[:], offset); err != nil {
		return 0, err
	}
	return 2 + int64(header[1]), nil
}

// iterate calls fn for every valid entry in the log. Used to rebuild the index.
func (d *dataLog) iterate(fn func(offset int64, keyType KeyType, encoded []byte) error) error {
	offset := int64(0)
	var header [2]byte
	for offset < d.size {
		if _, err := d.f.ReadAt(header[:], offset); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		keyType := KeyType(header[0])
		keyLen := int(header[1])
		if offset+2+int64(keyLen) > d.size {
			// truncated entry at end of file — discard
			d.size = offset
			if err := d.f.Truncate(offset); err != nil {
				return err
			}
			break
		}
		buf := make([]byte, keyLen)
		if _, err := d.f.ReadAt(buf, offset+2); err != nil {
			return err
		}
		if err := fn(offset, keyType, buf); err != nil {
			return err
		}
		offset += 2 + int64(keyLen)
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

// ctrlByte derives the 1-byte fingerprint stored in the ctrl array.
func ctrlByte(h uint32) byte {
	return byte(h>>25) | 0x80
}

// entryHeader helpers for binary encoding used by index and reverse files.
var bo = binary.LittleEndian
