package dict

import (
	"fmt"
	"path/filepath"

	lru "github.com/hashicorp/golang-lru/v2"
)

const defaultCacheSize = 200_000

// cacheKey is the LRU key: type tag + raw string to avoid encoding on cache hit.
type cacheKey struct {
	t KeyType
	s string
}

// Dict is a persistent dictionary mapping typed strings to sequential uint32 IDs.
type Dict struct {
	dat   *dataLog
	idx   *hashIndex
	rev   *reverseIndex
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

	dat, err := openDataLog(datPath)
	if err != nil {
		return nil, fmt.Errorf("dict: open data: %w", err)
	}

	idx, err := openIndex(idxPath)
	if err != nil {
		dat.close()
		return nil, fmt.Errorf("dict: open index: %w", err)
	}

	rev, err := openReverse(revPath)
	if err != nil {
		dat.close()
		idx.close()
		return nil, fmt.Errorf("dict: open reverse: %w", err)
	}

	var cache *lru.Cache[cacheKey, uint32]
	if cacheSize > 0 {
		cache, err = lru.New[cacheKey, uint32](cacheSize)
		if err != nil {
			dat.close()
			idx.close()
			rev.close()
			return nil, fmt.Errorf("dict: create cache: %w", err)
		}
	}

	d := &Dict{dat: dat, idx: idx, rev: rev, cache: cache}

	// If the index is empty but dat has data, rebuild from dat.
	if idx.header.LiveEntries == 0 && dat.size > 0 {
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

	codec := getCodec(keyType)
	if codec == nil {
		return false, fmt.Errorf("dict: unknown key type %d", keyType)
	}
	encoded, err := codec.Encode(s)
	if err != nil {
		return false, err
	}
	var h uint32
	if keyType == KeyRaw {
		h = hashKeyString(keyType, s)
	} else {
		h = hashKey(keyType, encoded)
	}
	cb := ctrlByte(h)
	_, found := d.idx.lookup(h, cb, keyType, encoded, d.dat)
	return found, nil
}

// Reverse returns the string and key type for a given ID.
func (d *Dict) Reverse(id uint32) (string, KeyType, error) {
	if id >= d.idx.header.NextID {
		return "", 0, fmt.Errorf("dict: id %d not found (max %d)", id, d.idx.header.NextID-1)
	}
	datOffset, err := d.rev.get(id)
	if err != nil {
		return "", 0, err
	}
	keyType, encoded, err := d.dat.readEntry(datOffset)
	if err != nil {
		return "", 0, fmt.Errorf("dict: read entry: %w", err)
	}
	codec := getCodec(keyType)
	if codec == nil {
		return "", 0, fmt.Errorf("dict: unknown key type %d in data", keyType)
	}
	s, err := codec.Decode(encoded)
	if err != nil {
		return "", 0, fmt.Errorf("dict: decode: %w", err)
	}
	return s, keyType, nil
}

// Len returns the number of entries in the dictionary.
func (d *Dict) Len() uint32 {
	return d.idx.header.NextID
}

// Sync flushes all changes to disk.
func (d *Dict) Sync() error {
	if err := d.dat.sync(); err != nil {
		return err
	}
	if err := d.idx.sync(); err != nil {
		return err
	}
	return d.rev.sync()
}

// Close syncs and closes all files.
func (d *Dict) Close() error {
	d.Sync()
	d.dat.close()
	d.idx.close()
	d.rev.close()
	return nil
}

func (d *Dict) getFromDisk(s string, keyType KeyType) (uint32, error) {
	codec := getCodec(keyType)
	if codec == nil {
		return 0, fmt.Errorf("dict: unknown key type %d", keyType)
	}

	// For raw keys, hash directly from string (zero-alloc) and use
	// unsafe string-to-bytes for the encoded form (also zero-alloc).
	encoded, err := codec.Encode(s)
	if err != nil {
		return 0, fmt.Errorf("dict: encode: %w", err)
	}
	if len(encoded) > 255 {
		return 0, fmt.Errorf("dict: encoded key too long: %d bytes", len(encoded))
	}

	var h uint32
	if keyType == KeyRaw {
		h = hashKeyString(keyType, s)
	} else {
		h = hashKey(keyType, encoded)
	}
	cb := ctrlByte(h)

	if id, found := d.idx.lookup(h, cb, keyType, encoded, d.dat); found {
		return id, nil
	}

	// Insert — for raw codec, encoded is a view into the string via unsafe,
	// but append copies it to disk so that's fine.
	datOffset, err := d.dat.append(keyType, encoded)
	if err != nil {
		return 0, fmt.Errorf("dict: append: %w", err)
	}

	id := d.idx.header.NextID
	d.idx.insert(h, cb, id, datOffset)
	d.idx.header.NextID++

	if err := d.rev.set(id, datOffset); err != nil {
		return 0, fmt.Errorf("dict: reverse set: %w", err)
	}

	if d.idx.needsGrow() {
		if err := d.idx.grow(d.dat); err != nil {
			return 0, fmt.Errorf("dict: grow index: %w", err)
		}
	}

	return id, nil
}

func (d *Dict) rebuildIndex() error {
	nextID := uint32(0)
	return d.dat.iterate(func(offset int64, keyType KeyType, encoded []byte) error {
		h := hashKey(keyType, encoded)
		cb := ctrlByte(h)
		d.idx.insert(h, cb, nextID, offset)
		if err := d.rev.set(nextID, offset); err != nil {
			return err
		}
		nextID++
		d.idx.header.NextID = nextID

		if d.idx.needsGrow() {
			return d.idx.grow(d.dat)
		}
		return nil
	})
}
