package dict

import (
	"fmt"
	"path/filepath"
	"sync"

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

	// Multi-chain
	KeyBech32         = codec.KeyBech32
	KeySolanaAddress  = codec.KeySolanaAddress
	KeySolanaSig      = codec.KeySolanaSig
	KeySS58           = codec.KeySS58
	KeyBitcoinAddress = codec.KeyBitcoinAddress
	KeyNumericString  = codec.KeyNumericString
)

const defaultCacheSize = 200_000

type cacheKey struct {
	t KeyType
	s string
}

// Dict is a persistent dictionary mapping typed strings to sequential uint64 IDs.
// IDs are assigned per key type, starting at 0 for each type.
// All public methods are safe for concurrent use.
type Dict struct {
	mu       sync.RWMutex
	dat      *internal.DataLog
	idx      *internal.HashIndex
	revs     [codec.MaxKeyType]*internal.ReverseIndex
	basePath string
	cache    *lru.Cache[cacheKey, uint64]
}

// Open opens or creates a dictionary at the given base path.
// Files used: <path>.dat, <path>.idx, <path>.rev.<type>
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

	dat, err := internal.OpenDataLog(datPath)
	if err != nil {
		return nil, fmt.Errorf("dict: open data: %w", err)
	}

	idx, err := internal.OpenIndex(idxPath)
	if err != nil {
		dat.Close()
		return nil, fmt.Errorf("dict: open index: %w", err)
	}

	var cache *lru.Cache[cacheKey, uint64]
	if cacheSize > 0 {
		cache, err = lru.New[cacheKey, uint64](cacheSize)
		if err != nil {
			dat.Close()
			idx.Close()
			return nil, fmt.Errorf("dict: create cache: %w", err)
		}
	}

	d := &Dict{
		dat:      dat,
		idx:      idx,
		basePath: filepath.Join(dir, base),
		cache:    cache,
	}

	if idx.Header.LiveEntries == 0 && dat.Size > 0 {
		if err := d.rebuildIndex(); err != nil {
			d.Close()
			return nil, fmt.Errorf("dict: rebuild index: %w", err)
		}
	}

	// Open reverse index files for active types.
	for t := range d.idx.Header.NextIDs {
		if d.idx.Header.NextIDs[t] > 0 && d.revs[t] == nil {
			rev, err := internal.OpenReverse(d.revFilePath(KeyType(t)))
			if err != nil {
				d.Close()
				return nil, fmt.Errorf("dict: open reverse for type %02x: %w", t, err)
			}
			d.revs[t] = rev
		}
	}

	return d, nil
}

// Get returns the ID for the given key, inserting it if it doesn't exist.
// IDs are sequential per key type, starting at 0.
func (d *Dict) Get(s string, keyType KeyType) (uint64, error) {
	ck := cacheKey{keyType, s}
	if d.cache != nil {
		if id, ok := d.cache.Get(ck); ok {
			return id, nil
		}
	}

	d.mu.Lock()
	// Double-check: another goroutine may have inserted while we waited.
	if d.cache != nil {
		if id, ok := d.cache.Get(ck); ok {
			d.mu.Unlock()
			return id, nil
		}
	}
	id, err := d.getFromDisk(s, keyType)
	d.mu.Unlock()
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
func (d *Dict) BatchGet(entries []BatchEntry) ([]uint64, error) {
	ids := make([]uint64, len(entries))
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

	d.mu.RLock()
	defer d.mu.RUnlock()

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

// Reverse returns the original string for a given per-type ID.
func (d *Dict) Reverse(id uint64, keyType KeyType) (string, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if id >= d.idx.Header.NextIDs[keyType] {
		return "", fmt.Errorf("dict: id %d not found for type %02x (max %d)", id, byte(keyType), d.idx.Header.NextIDs[keyType]-1)
	}
	rev := d.revs[keyType]
	if rev == nil {
		return "", fmt.Errorf("dict: no reverse index for type %02x", byte(keyType))
	}
	datOffset, err := rev.Get(id)
	if err != nil {
		return "", err
	}
	kt, encoded, err := d.dat.ReadEntry(datOffset)
	if err != nil {
		return "", fmt.Errorf("dict: read entry: %w", err)
	}
	c := codec.Get(kt)
	if c == nil {
		return "", fmt.Errorf("dict: unknown key type %d in data", kt)
	}
	s, err := c.Decode(encoded)
	if err != nil {
		return "", fmt.Errorf("dict: decode: %w", err)
	}
	return s, nil
}

// Len returns the total number of entries across all key types.
func (d *Dict) Len() uint64 {
	d.mu.RLock()
	defer d.mu.RUnlock()
	var total uint64
	for _, n := range d.idx.Header.NextIDs {
		total += n
	}
	return total
}

// LenType returns the number of entries for a specific key type.
func (d *Dict) LenType(keyType KeyType) uint64 {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.idx.Header.NextIDs[keyType]
}

// Sync flushes all changes to disk.
func (d *Dict) Sync() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.syncLocked()
}

func (d *Dict) syncLocked() error {
	if err := d.dat.Sync(); err != nil {
		return err
	}
	if err := d.idx.Sync(); err != nil {
		return err
	}
	for _, rev := range d.revs {
		if rev != nil {
			if err := rev.Sync(); err != nil {
				return err
			}
		}
	}
	return nil
}

// Close syncs and closes all files.
func (d *Dict) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.syncLocked()
	d.dat.Close()
	d.idx.Close()
	for i, rev := range d.revs {
		if rev != nil {
			rev.Close()
			d.revs[i] = nil
		}
	}
	return nil
}

func (d *Dict) revFilePath(keyType KeyType) string {
	return fmt.Sprintf("%s.rev.%02x", d.basePath, byte(keyType))
}

// ensureRev lazily opens the reverse index file for a key type.
// Must be called under the write lock.
func (d *Dict) ensureRev(keyType KeyType) (*internal.ReverseIndex, error) {
	if d.revs[keyType] != nil {
		return d.revs[keyType], nil
	}
	rev, err := internal.OpenReverse(d.revFilePath(keyType))
	if err != nil {
		return nil, err
	}
	d.revs[keyType] = rev
	return rev, nil
}

func (d *Dict) getFromDisk(s string, keyType KeyType) (uint64, error) {
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

	id := d.idx.Header.NextIDs[keyType]
	d.idx.Insert(h, cb, id, datOffset)
	d.idx.Header.NextIDs[keyType]++

	rev, err := d.ensureRev(keyType)
	if err != nil {
		return 0, fmt.Errorf("dict: open reverse: %w", err)
	}
	if err := rev.Set(id, datOffset); err != nil {
		return 0, fmt.Errorf("dict: reverse set: %w", err)
	}

	if d.idx.NeedsGrow() {
		if err := d.idx.Grow(func(newIdx *internal.HashIndex) error {
			var perType [codec.MaxKeyType]uint64
			return d.dat.Iterate(func(offset int64, kt codec.KeyType, enc []byte) error {
				h := internal.HashKey(kt, enc)
				cb := internal.CtrlByte(h)
				newIdx.Insert(h, cb, perType[kt], offset)
				perType[kt]++
				return nil
			})
		}); err != nil {
			return 0, fmt.Errorf("dict: grow index: %w", err)
		}
	}

	return id, nil
}

func (d *Dict) rebuildIndex() error {
	return d.dat.Iterate(func(offset int64, keyType codec.KeyType, encoded []byte) error {
		h := internal.HashKey(keyType, encoded)
		cb := internal.CtrlByte(h)
		id := d.idx.Header.NextIDs[keyType]
		d.idx.Insert(h, cb, id, offset)

		rev, err := d.ensureRev(keyType)
		if err != nil {
			return err
		}
		if err := rev.Set(id, offset); err != nil {
			return err
		}

		d.idx.Header.NextIDs[keyType]++

		if d.idx.NeedsGrow() {
			return d.idx.Grow(func(newIdx *internal.HashIndex) error {
				var perType [codec.MaxKeyType]uint64
				return d.dat.Iterate(func(off int64, kt codec.KeyType, enc []byte) error {
					h := internal.HashKey(kt, enc)
					cb := internal.CtrlByte(h)
					newIdx.Insert(h, cb, perType[kt], off)
					perType[kt]++
					return nil
				})
			})
		}
		return nil
	})
}
