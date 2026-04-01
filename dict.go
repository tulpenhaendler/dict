package dict

import (
	"fmt"
	"path/filepath"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/tulpenhaendler/dict/codec"
	"github.com/tulpenhaendler/dict/internal"
)

// Re-export codec types so callers can use dict.KeyRaw etc.
type KeyType = codec.KeyType

const (
	// Generic
	KeyRaw    = codec.KeyRaw
	KeyHex    = codec.KeyHex
	KeyBase64 = codec.KeyBase64
	KeyBase58 = codec.KeyBase58

	// Tezos
	KeyTezosAddress      = codec.KeyTezosAddress
	KeyTezosBlockHash    = codec.KeyTezosBlockHash
	KeyTezosSignature    = codec.KeyTezosSignature
	KeyTezosOpHash       = codec.KeyTezosOpHash
	KeyTezosProtocolHash = codec.KeyTezosProtocolHash
	KeyTezosChainID      = codec.KeyTezosChainID
	KeyTezosExprHash     = codec.KeyTezosExprHash
	KeyTezosContextHash  = codec.KeyTezosContextHash
	KeyTezosPayloadHash  = codec.KeyTezosPayloadHash
	KeyTezosPubkey       = codec.KeyTezosPubkey

	// EVM
	KeyEVMAddress  = codec.KeyEVMAddress
	KeyEVMHash32   = codec.KeyEVMHash32
	KeyEVMSelector = codec.KeyEVMSelector

	// IPFS
	KeyIPFSCID = codec.KeyIPFSCID
)

const defaultCacheSize = 200_000

type cacheKey struct {
	t KeyType
	s string
}

// Dict is a persistent dictionary mapping typed strings to sequential uint32 IDs.
type Dict struct {
	dat   *internal.DataLog
	idx   *internal.HashIndex
	rev   *internal.ReverseIndex
	cache *lru.Cache[cacheKey, uint32]
}

// Open opens or creates a dictionary at the given base path.
// Three files are used: <path>.dat, <path>.idx, <path>.rev
func Open(basePath string) (*Dict, error) {
	return OpenWithCacheSize(basePath, defaultCacheSize)
}

// OpenWithCacheSize opens a dictionary with a custom LRU cache size.
// Set cacheSize to 0 to disable the cache.
func OpenWithCacheSize(basePath string, cacheSize int) (*Dict, error) {
	dir := filepath.Dir(basePath)
	base := filepath.Base(basePath)
	datPath := filepath.Join(dir, base+".dat")
	idxPath := filepath.Join(dir, base+".idx")
	revPath := filepath.Join(dir, base+".rev")

	dat, err := internal.OpenDataLog(datPath)
	if err != nil {
		return nil, fmt.Errorf("dict: open data: %w", err)
	}

	idx, err := internal.OpenIndex(idxPath)
	if err != nil {
		dat.Close()
		return nil, fmt.Errorf("dict: open index: %w", err)
	}

	rev, err := internal.OpenReverse(revPath)
	if err != nil {
		dat.Close()
		idx.Close()
		return nil, fmt.Errorf("dict: open reverse: %w", err)
	}

	var cache *lru.Cache[cacheKey, uint32]
	if cacheSize > 0 {
		cache, err = lru.New[cacheKey, uint32](cacheSize)
		if err != nil {
			dat.Close()
			idx.Close()
			rev.Close()
			return nil, fmt.Errorf("dict: create cache: %w", err)
		}
	}

	d := &Dict{dat: dat, idx: idx, rev: rev, cache: cache}

	if idx.Header.LiveEntries == 0 && dat.Size > 0 {
		if err := d.rebuildIndex(); err != nil {
			d.Close()
			return nil, fmt.Errorf("dict: rebuild index: %w", err)
		}
	}

	return d, nil
}

// Get returns the ID for the given key, inserting it if it doesn't exist.
func (d *Dict) Get(s string, keyType KeyType) (uint32, error) {
	ck := cacheKey{keyType, s}
	if d.cache != nil {
		if id, ok := d.cache.Get(ck); ok {
			return id, nil
		}
	}

	id, err := d.getFromDisk(s, keyType)
	if err != nil {
		return 0, err
	}

	if d.cache != nil {
		d.cache.Add(ck, id)
	}
	return id, nil
}

// BatchEntry is an input for BatchGet.
type BatchEntry struct {
	Key     string
	KeyType KeyType
}

// BatchGet looks up or inserts multiple keys, returning their IDs in the same order.
func (d *Dict) BatchGet(entries []BatchEntry) ([]uint32, error) {
	ids := make([]uint32, len(entries))
	for i, e := range entries {
		id, err := d.Get(e.Key, e.KeyType)
		if err != nil {
			return nil, fmt.Errorf("dict: batch entry %d: %w", i, err)
		}
		ids[i] = id
	}
	return ids, nil
}

// Exists checks if a key is in the dictionary without inserting.
func (d *Dict) Exists(s string, keyType KeyType) (bool, error) {
	if d.cache != nil {
		if _, ok := d.cache.Get(cacheKey{keyType, s}); ok {
			return true, nil
		}
	}

	c := codec.Get(keyType)
	if c == nil {
		return false, fmt.Errorf("dict: unknown key type %d", keyType)
	}
	encoded, err := c.Encode(s)
	if err != nil {
		return false, err
	}
	var h uint32
	if keyType == KeyRaw {
		h = internal.HashKeyString(keyType, s)
	} else {
		h = internal.HashKey(keyType, encoded)
	}
	cb := internal.CtrlByte(h)
	_, found := d.idx.Lookup(h, cb, func(off int64) bool {
		return d.dat.MatchEntry(off, keyType, encoded)
	})
	return found, nil
}

// Reverse returns the string and key type for a given ID.
func (d *Dict) Reverse(id uint32) (string, KeyType, error) {
	if id >= d.idx.Header.NextID {
		return "", 0, fmt.Errorf("dict: id %d not found (max %d)", id, d.idx.Header.NextID-1)
	}
	datOffset, err := d.rev.Get(id)
	if err != nil {
		return "", 0, err
	}
	keyType, encoded, err := d.dat.ReadEntry(datOffset)
	if err != nil {
		return "", 0, fmt.Errorf("dict: read entry: %w", err)
	}
	c := codec.Get(keyType)
	if c == nil {
		return "", 0, fmt.Errorf("dict: unknown key type %d in data", keyType)
	}
	s, err := c.Decode(encoded)
	if err != nil {
		return "", 0, fmt.Errorf("dict: decode: %w", err)
	}
	return s, keyType, nil
}

// Len returns the number of entries in the dictionary.
func (d *Dict) Len() uint32 {
	return d.idx.Header.NextID
}

// Sync flushes all changes to disk.
func (d *Dict) Sync() error {
	if err := d.dat.Sync(); err != nil {
		return err
	}
	if err := d.idx.Sync(); err != nil {
		return err
	}
	return d.rev.Sync()
}

// Close syncs and closes all files.
func (d *Dict) Close() error {
	d.Sync()
	d.dat.Close()
	d.idx.Close()
	d.rev.Close()
	return nil
}

func (d *Dict) getFromDisk(s string, keyType KeyType) (uint32, error) {
	c := codec.Get(keyType)
	if c == nil {
		return 0, fmt.Errorf("dict: unknown key type %d", keyType)
	}

	encoded, err := c.Encode(s)
	if err != nil {
		return 0, fmt.Errorf("dict: encode: %w", err)
	}
	if len(encoded) > 255 {
		return 0, fmt.Errorf("dict: encoded key too long: %d bytes", len(encoded))
	}

	var h uint32
	if keyType == KeyRaw {
		h = internal.HashKeyString(keyType, s)
	} else {
		h = internal.HashKey(keyType, encoded)
	}
	cb := internal.CtrlByte(h)

	if id, found := d.idx.Lookup(h, cb, func(off int64) bool {
		return d.dat.MatchEntry(off, keyType, encoded)
	}); found {
		return id, nil
	}

	datOffset, err := d.dat.Append(keyType, encoded)
	if err != nil {
		return 0, fmt.Errorf("dict: append: %w", err)
	}

	id := d.idx.Header.NextID
	d.idx.Insert(h, cb, id, datOffset)
	d.idx.Header.NextID++

	if err := d.rev.Set(id, datOffset); err != nil {
		return 0, fmt.Errorf("dict: reverse set: %w", err)
	}

	if d.idx.NeedsGrow() {
		if err := d.idx.Grow(func(newIdx *internal.HashIndex) error {
			return d.dat.Iterate(func(offset int64, kt codec.KeyType, enc []byte) error {
				h := internal.HashKey(kt, enc)
				cb := internal.CtrlByte(h)
				newIdx.Insert(h, cb, newIdx.Header.LiveEntries, offset)
				return nil
			})
		}); err != nil {
			return 0, fmt.Errorf("dict: grow index: %w", err)
		}
	}

	return id, nil
}

func (d *Dict) rebuildIndex() error {
	nextID := uint32(0)
	return d.dat.Iterate(func(offset int64, keyType codec.KeyType, encoded []byte) error {
		h := internal.HashKey(keyType, encoded)
		cb := internal.CtrlByte(h)
		d.idx.Insert(h, cb, nextID, offset)
		if err := d.rev.Set(nextID, offset); err != nil {
			return err
		}
		nextID++
		d.idx.Header.NextID = nextID

		if d.idx.NeedsGrow() {
			return d.idx.Grow(func(newIdx *internal.HashIndex) error {
				return d.dat.Iterate(func(off int64, kt codec.KeyType, enc []byte) error {
					h := internal.HashKey(kt, enc)
					cb := internal.CtrlByte(h)
					newIdx.Insert(h, cb, newIdx.Header.LiveEntries, off)
					return nil
				})
			})
		}
		return nil
	})
}
