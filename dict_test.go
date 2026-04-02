package dict

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func tempPath(t testing.TB) string {
	t.Helper()
	dir := t.TempDir()
	return filepath.Join(dir, "test")
}

func TestGetAndExists(t *testing.T) {
	d, err := Open(tempPath(t))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	id0, err := d.Get("hello", KeyRaw)
	if err != nil {
		t.Fatal(err)
	}
	if id0 != 0 {
		t.Fatalf("first id = %d, want 0", id0)
	}

	id1, err := d.Get("world", KeyRaw)
	if err != nil {
		t.Fatal(err)
	}
	if id1 != 1 {
		t.Fatalf("second id = %d, want 1", id1)
	}

	id0b, _ := d.Get("hello", KeyRaw)
	if id0b != 0 {
		t.Fatalf("repeat id = %d, want 0", id0b)
	}

	ok, _ := d.Exists("hello", KeyRaw)
	if !ok {
		t.Fatal("Exists(hello) = false")
	}

	ok, _ = d.Exists("missing", KeyRaw)
	if ok {
		t.Fatal("Exists(missing) = true")
	}
}

func TestReverse(t *testing.T) {
	d, err := Open(tempPath(t))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	d.Get("alpha", KeyRaw)
	d.Get("beta", KeyRaw)
	d.Get("gamma", KeyRaw)

	for id, want := range []string{"alpha", "beta", "gamma"} {
		got, err := d.Reverse(uint64(id), KeyRaw)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("Reverse(%d) = %q, want %q", id, got, want)
		}
	}
}

func TestPersistence(t *testing.T) {
	p := tempPath(t)

	d, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	d.Get("foo", KeyRaw)
	d.Get("bar", KeyRaw)
	d.Close()

	d, err = Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	if d.Len() != 2 {
		t.Fatalf("Len = %d, want 2", d.Len())
	}

	ok, _ := d.Exists("foo", KeyRaw)
	if !ok {
		t.Fatal("foo not found after reopen")
	}

	id, _ := d.Get("foo", KeyRaw)
	if id != 0 {
		t.Fatalf("foo id = %d, want 0", id)
	}

	s, _ := d.Reverse(1, KeyRaw)
	if s != "bar" {
		t.Fatalf("Reverse(1) = %q, want bar", s)
	}
}

func TestGrow(t *testing.T) {
	d, err := Open(tempPath(t))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	n := 50000
	for i := 0; i < n; i++ {
		key := fmt.Sprintf("key-%d", i)
		id, err := d.Get(key, KeyRaw)
		if err != nil {
			t.Fatalf("Get(%q) at i=%d: %v", key, i, err)
		}
		if id != uint64(i) {
			t.Fatalf("Get(%q) = %d, want %d", key, id, i)
		}
	}

	for i := 0; i < n; i++ {
		key := fmt.Sprintf("key-%d", i)
		ok, _ := d.Exists(key, KeyRaw)
		if !ok {
			t.Fatalf("key %q not found after grow", key)
		}
	}

	for i := 0; i < n; i++ {
		s, err := d.Reverse(uint64(i), KeyRaw)
		if err != nil {
			t.Fatal(err)
		}
		want := fmt.Sprintf("key-%d", i)
		if s != want {
			t.Fatalf("Reverse(%d) = %q, want %q", i, s, want)
		}
	}
}

func TestEmptyKey(t *testing.T) {
	d, err := Open(tempPath(t))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	id, _ := d.Get("", KeyRaw)
	if id != 0 {
		t.Fatalf("empty key id = %d, want 0", id)
	}

	s, _ := d.Reverse(0, KeyRaw)
	if s != "" {
		t.Fatalf("Reverse(0) = %q, want empty", s)
	}
}

func TestBatchGet(t *testing.T) {
	d, err := Open(tempPath(t))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	d.Get("existing", KeyRaw)

	entries := []BatchEntry{
		{"existing", KeyRaw},
		{"new1", KeyRaw},
		{"new2", KeyRaw},
		{"existing", KeyRaw},
		{"new1", KeyRaw},
	}

	ids, err := d.BatchGet(entries)
	if err != nil {
		t.Fatal(err)
	}

	if ids[0] != 0 {
		t.Fatalf("existing id = %d, want 0", ids[0])
	}
	if ids[3] != ids[0] {
		t.Fatalf("duplicate existing: %d != %d", ids[3], ids[0])
	}
	if ids[4] != ids[1] {
		t.Fatalf("duplicate new1: %d != %d", ids[4], ids[1])
	}
	if ids[1] == ids[2] {
		t.Fatal("new1 and new2 got same id")
	}
}

func TestCacheHit(t *testing.T) {
	d, err := Open(tempPath(t))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	id1, _ := d.Get("cached", KeyRaw)
	id2, _ := d.Get("cached", KeyRaw)
	if id1 != id2 {
		t.Fatalf("cache returned different id: %d vs %d", id1, id2)
	}

	ok, _ := d.Exists("cached", KeyRaw)
	if !ok {
		t.Fatal("Exists missed cache")
	}
}

func TestNoCache(t *testing.T) {
	d, err := OpenWithCacheSize(tempPath(t), 0)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	id, _ := d.Get("hello", KeyRaw)
	if id != 0 {
		t.Fatalf("id = %d, want 0", id)
	}

	id2, _ := d.Get("hello", KeyRaw)
	if id2 != 0 {
		t.Fatalf("id = %d, want 0", id2)
	}
}

// --- Tezos integration tests (through Dict) ---

func TestTezosAddressInDict(t *testing.T) {
	d, err := Open(tempPath(t))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	addrs := []string{
		"tz1cSj7fTex3JPd1p1LN1fwek6AV1kH93Wwc",
		"tz2TkyKVYyKSrhYtrvX3rSNep6bCuhuK5hSJ",
		"tz3cqThj23Feu55KDynm7Vg81mCMpWDgzQZq",
		"tz4XezFZgm3vaPavdryBv9KMFpYGdtAGWKMp",
		"KT1Ug9wWbRuUs1XXRuK11o6syWdTFZQsmvw3",
		"sr1GG29rd2XtsHkYvHEmJrWBzCAYBwkXXi6j",
	}

	for i, addr := range addrs {
		id, err := d.Get(addr, KeyTezosAddress)
		if err != nil {
			t.Fatalf("Get(%s): %v", addr, err)
		}
		if id != uint64(i) {
			t.Fatalf("Get(%s) = %d, want %d", addr, id, i)
		}
	}
	for i, addr := range addrs {
		s, err := d.Reverse(uint64(i), KeyTezosAddress)
		if err != nil {
			t.Fatalf("Reverse(%d): %v", i, err)
		}
		if s != addr {
			t.Fatalf("Reverse(%d) = %q, want %q", i, s, addr)
		}
	}
}

func TestTezosOtherTypes(t *testing.T) {
	d, err := Open(tempPath(t))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	tests := []struct {
		value   string
		keyType KeyType
	}{
		{"BKwxR3jV27U44ssm3G7fJKXJgKn3FAeEHRyNoRK9w6DU45QDG9M", KeyTezosBlockHash},
		{"onpHScbMhvwJd7WY2etxKJjmEyNgX7BBjmEYKuytwNLQZ3KbVTX", KeyTezosOpHash},
		{"PtTALLiNtPec7mE7yY4m3k26J8Qukef3E3ehzhfXgFZKGtDdAXu", KeyTezosProtocolHash},
		{"NetXdQprcVkpaWU", KeyTezosChainID},
		{"sigbmAunXeu7g8YYoBYKsBsKVvsKb7N8tSQH3z9gcutg5kYz78gGNzbeyUpHosv1jfAGTrbkf5uDHgyQg58L6HtGbn5uptSZ", KeyTezosSignature},
		{"expruE5MGe6oKRLTiog6iBZzpztj5kCGzMEYBfWzsVebPnhn43ndYa", KeyTezosExprHash},
		{"CoWR81CLNoDEZBdx454giaxovUNFo67GMkY1a6hmUxF9L3GiE6cL", KeyTezosContextHash},
		{"vh2g1JN3Ck6b99QcZkMb5dDWxj8G3HmkUHVW6y9gLpRbDKSu5DsA", KeyTezosPayloadHash},
		{"edpkuxf7c72ZXnw4G3LpAhCRTjrXTB7fa7kC5jAoaZhuciXjur54dN", KeyTezosPubkey},
	}

	for _, tc := range tests {
		id, err := d.Get(tc.value, tc.keyType)
		if err != nil {
			t.Fatalf("Get(%s): %v", tc.value, err)
		}
		// Each is the first (and only) entry of its type → id 0.
		if id != 0 {
			t.Fatalf("Get(%s) = %d, want 0", tc.value, id)
		}

		// Upsert returns the same id.
		id2, _ := d.Get(tc.value, tc.keyType)
		if id2 != 0 {
			t.Fatalf("upsert %s: %d != 0", tc.value, id2)
		}

		// Reverse round-trip.
		s, err := d.Reverse(0, tc.keyType)
		if err != nil {
			t.Fatalf("Reverse(0, %02x): %v", byte(tc.keyType), err)
		}
		if s != tc.value {
			t.Fatalf("Reverse(0, %02x) = %q, want %q", byte(tc.keyType), s, tc.value)
		}
	}
}

func TestPerTypeIDs(t *testing.T) {
	d, err := Open(tempPath(t))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	// Same string stored under different types gets per-type id 0.
	addr := "tz1cSj7fTex3JPd1p1LN1fwek6AV1kH93Wwc"
	idRaw, _ := d.Get(addr, KeyRaw)
	idTezos, _ := d.Get(addr, KeyTezosAddress)
	if idRaw != 0 || idTezos != 0 {
		t.Fatalf("expected both ids = 0, got raw=%d tezos=%d", idRaw, idTezos)
	}

	// They reverse independently.
	sRaw, _ := d.Reverse(0, KeyRaw)
	sTezos, _ := d.Reverse(0, KeyTezosAddress)
	if sRaw != addr || sTezos != addr {
		t.Fatalf("reverse mismatch: raw=%q tezos=%q", sRaw, sTezos)
	}

	// Total count is 2 (one per type).
	if d.Len() != 2 {
		t.Fatalf("Len = %d, want 2", d.Len())
	}
	if d.LenType(KeyRaw) != 1 {
		t.Fatalf("LenType(Raw) = %d, want 1", d.LenType(KeyRaw))
	}
	if d.LenType(KeyTezosAddress) != 1 {
		t.Fatalf("LenType(TezosAddress) = %d, want 1", d.LenType(KeyTezosAddress))
	}
}

func TestEVMTypesInDict(t *testing.T) {
	d, err := Open(tempPath(t))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	tests := []struct {
		value   string
		keyType KeyType
	}{
		{"0x742d35Cc6634C0532925a3b844Bc9e7595f2bD18", KeyEVMAddress},
		{"0xd4e56740f876aef8c010b86a40d5f56745a118d0906a34e69aec8c0db1cb8fa3", KeyEVMHash32},
		{"0xa9059cbb", KeyEVMSelector},
	}

	for _, tc := range tests {
		id, err := d.Get(tc.value, tc.keyType)
		if err != nil {
			t.Fatalf("Get(%s): %v", tc.value, err)
		}
		// Each is the first entry of its type.
		if id != 0 {
			t.Fatalf("Get(%s) = %d, want 0", tc.value, id)
		}
		id2, _ := d.Get(tc.value, tc.keyType)
		if id2 != 0 {
			t.Fatalf("upsert: %d != 0", id2)
		}
	}
}

func TestIPFSCIDInDict(t *testing.T) {
	d, err := Open(tempPath(t))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	cids := []string{
		"QmT78zSuBmuS4z925WZfrqQ1qHaJ56DQaTfyMUF7F8ff5o",
		"bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi",
		"bafkreihdwdcefgh4dqkjv67uzcmw7ojee6xedzdetojuzjevtenosa7tiy",
	}

	for i, cid := range cids {
		id, err := d.Get(cid, KeyIPFSCID)
		if err != nil {
			t.Fatalf("Get(%s): %v", cid, err)
		}
		if id != uint64(i) {
			t.Fatalf("Get(%s) = %d, want %d", cid, id, i)
		}
		s, err := d.Reverse(uint64(i), KeyIPFSCID)
		if err != nil {
			t.Fatalf("Reverse(%d): %v", i, err)
		}
		if s != cid {
			t.Fatalf("Reverse(%d) = %q, want %q", i, s, cid)
		}
	}
}

func TestGenericEncodingsInDict(t *testing.T) {
	d, err := Open(tempPath(t))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	tests := []struct {
		value   string
		keyType KeyType
	}{
		{"0xdeadbeef", KeyHex},
		{"SGVsbG8gV29ybGQ=", KeyBase64},
	}

	for _, tc := range tests {
		id, err := d.Get(tc.value, tc.keyType)
		if err != nil {
			t.Fatalf("Get(%s): %v", tc.value, err)
		}
		// Each is the first entry of its type.
		if id != 0 {
			t.Fatalf("Get(%s) = %d, want 0", tc.value, id)
		}
	}
}

// --- Concurrency tests ---

func TestConcurrentGet(t *testing.T) {
	d, err := Open(tempPath(t))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	const goroutines = 16
	const keysPerGoroutine = 1000
	var wg sync.WaitGroup

	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < keysPerGoroutine; i++ {
				key := fmt.Sprintf("key-%d", i)
				if _, err := d.Get(key, KeyRaw); err != nil {
					t.Errorf("Get(%q): %v", key, err)
					return
				}
			}
		}()
	}
	wg.Wait()

	if d.Len() != keysPerGoroutine {
		t.Fatalf("Len = %d, want %d", d.Len(), keysPerGoroutine)
	}

	// Verify each key has a stable, unique ID.
	seen := make(map[uint64]string)
	for i := 0; i < keysPerGoroutine; i++ {
		key := fmt.Sprintf("key-%d", i)
		id, _ := d.Get(key, KeyRaw)
		if prev, ok := seen[id]; ok && prev != key {
			t.Fatalf("ID %d maps to both %q and %q", id, prev, key)
		}
		seen[id] = key
	}
}

func TestConcurrentReadWrite(t *testing.T) {
	d, err := Open(tempPath(t))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	// Pre-populate
	for i := 0; i < 100; i++ {
		d.Get(fmt.Sprintf("pre-%d", i), KeyRaw)
	}

	done := make(chan struct{})
	var readers sync.WaitGroup
	var writer sync.WaitGroup

	// Writer: insert new keys until readers are done.
	writer.Add(1)
	go func() {
		defer writer.Done()
		for i := 0; ; i++ {
			select {
			case <-done:
				return
			default:
			}
			d.Get(fmt.Sprintf("new-%d", i), KeyRaw)
		}
	}()

	// Readers: each does finite work concurrently with the writer.
	for r := 0; r < 8; r++ {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for i := 0; i < 5000; i++ {
				d.Exists("pre-50", KeyRaw)
				d.Reverse(0, KeyRaw)
				d.Len()
			}
		}()
	}

	readers.Wait()
	close(done)
	writer.Wait()
}

func TestConcurrentGrow(t *testing.T) {
	d, err := Open(tempPath(t))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	const goroutines = 8
	const keysPerGoroutine = 5000
	var wg sync.WaitGroup

	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		g := g
		go func() {
			defer wg.Done()
			for i := 0; i < keysPerGoroutine; i++ {
				key := fmt.Sprintf("g%d-k%d", g, i)
				if _, err := d.Get(key, KeyRaw); err != nil {
					t.Errorf("Get(%q): %v", key, err)
					return
				}
			}
		}()
	}
	wg.Wait()

	expected := uint64(goroutines * keysPerGoroutine)
	if d.Len() != expected {
		t.Fatalf("Len = %d, want %d", d.Len(), expected)
	}

	// Verify all keys round-trip.
	for g := 0; g < goroutines; g++ {
		for i := 0; i < keysPerGoroutine; i++ {
			key := fmt.Sprintf("g%d-k%d", g, i)
			id, err := d.Get(key, KeyRaw)
			if err != nil {
				t.Fatalf("verify Get(%q): %v", key, err)
			}
			s, err := d.Reverse(id, KeyRaw)
			if err != nil {
				t.Fatalf("Reverse(%d): %v", id, err)
			}
			if s != key {
				t.Fatalf("Reverse(%d) = %q, want %q", id, s, key)
			}
		}
	}
}

// --- Benchmarks ---

func BenchmarkGet(b *testing.B) {
	dir, _ := os.MkdirTemp("", "dict-bench-*")
	defer os.RemoveAll(dir)
	d, _ := Open(filepath.Join(dir, "bench"))
	defer d.Close()

	for i := 0; i < 100000; i++ {
		d.Get(fmt.Sprintf("key-%d", i), KeyRaw)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.Get(fmt.Sprintf("key-%d", i%100000), KeyRaw)
	}
}

func BenchmarkGetNoCache(b *testing.B) {
	dir, _ := os.MkdirTemp("", "dict-bench-*")
	defer os.RemoveAll(dir)
	d, _ := OpenWithCacheSize(filepath.Join(dir, "bench"), 0)
	defer d.Close()

	for i := 0; i < 100000; i++ {
		d.Get(fmt.Sprintf("key-%d", i), KeyRaw)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.Get(fmt.Sprintf("key-%d", i%100000), KeyRaw)
	}
}

func BenchmarkBatchGet(b *testing.B) {
	dir, _ := os.MkdirTemp("", "dict-bench-*")
	defer os.RemoveAll(dir)
	d, _ := Open(filepath.Join(dir, "bench"))
	defer d.Close()

	for i := 0; i < 100000; i++ {
		d.Get(fmt.Sprintf("key-%d", i), KeyRaw)
	}

	batch := make([]BatchEntry, 1000)
	for i := range batch {
		batch[i] = BatchEntry{Key: fmt.Sprintf("key-%d", i), KeyType: KeyRaw}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.BatchGet(batch)
	}
}

func BenchmarkTezosGet(b *testing.B) {
	dir := b.TempDir()
	d, _ := Open(filepath.Join(dir, "bench"))
	defer d.Close()

	addr := "tz1cSj7fTex3JPd1p1LN1fwek6AV1kH93Wwc"
	d.Get(addr, KeyTezosAddress)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.Get(addr, KeyTezosAddress)
	}
}
