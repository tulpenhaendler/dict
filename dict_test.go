package dict

import (
	"fmt"
	"os"
	"path/filepath"
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
		got, kt, err := d.Reverse(uint32(id))
		if err != nil {
			t.Fatal(err)
		}
		if got != want || kt != KeyRaw {
			t.Fatalf("Reverse(%d) = %q %d, want %q %d", id, got, kt, want, KeyRaw)
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

	s, _, _ := d.Reverse(1)
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
		if id != uint32(i) {
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
		s, _, err := d.Reverse(uint32(i))
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

	s, _, _ := d.Reverse(0)
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
		if id != uint32(i) {
			t.Fatalf("Get(%s) = %d, want %d", addr, id, i)
		}
	}
	for i, addr := range addrs {
		s, kt, _ := d.Reverse(uint32(i))
		if kt != KeyTezosAddress || s != addr {
			t.Fatalf("Reverse(%d) = %q, want %q", i, s, addr)
		}
	}
}

func TestTezosTypesInDict(t *testing.T) {
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

	for i, tc := range tests {
		id, err := d.Get(tc.value, tc.keyType)
		if err != nil {
			t.Fatalf("Get(%s): %v", tc.value, err)
		}
		if id != uint32(i) {
			t.Fatalf("Get(%s) = %d, want %d", tc.value, id, i)
		}

		// upsert
		id2, _ := d.Get(tc.value, tc.keyType)
		if id2 != uint32(i) {
			t.Fatalf("upsert %s: %d != %d", tc.value, id2, i)
		}

		// reverse
		s, kt, _ := d.Reverse(uint32(i))
		if kt != tc.keyType || s != tc.value {
			t.Fatalf("Reverse(%d): got %q, want %q", i, s, tc.value)
		}
	}
}

func TestTezosNoCollisionWithRaw(t *testing.T) {
	d, err := Open(tempPath(t))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	addr := "tz1cSj7fTex3JPd1p1LN1fwek6AV1kH93Wwc"
	idRaw, _ := d.Get(addr, KeyRaw)
	idTezos, _ := d.Get(addr, KeyTezosAddress)
	if idRaw == idTezos {
		t.Fatalf("raw and tezos got same ID %d", idRaw)
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

	for i, tc := range tests {
		id, err := d.Get(tc.value, tc.keyType)
		if err != nil {
			t.Fatalf("Get(%s): %v", tc.value, err)
		}
		if id != uint32(i) {
			t.Fatalf("Get(%s) = %d, want %d", tc.value, id, i)
		}
		// upsert
		id2, _ := d.Get(tc.value, tc.keyType)
		if id2 != uint32(i) {
			t.Fatalf("upsert: %d != %d", id2, i)
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
		if id != uint32(i) {
			t.Fatalf("Get(%s) = %d, want %d", cid, id, i)
		}
		s, kt, _ := d.Reverse(uint32(i))
		if kt != KeyIPFSCID || s != cid {
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

	for i, tc := range tests {
		id, err := d.Get(tc.value, tc.keyType)
		if err != nil {
			t.Fatalf("Get(%s): %v", tc.value, err)
		}
		if id != uint32(i) {
			t.Fatalf("Get(%s) = %d, want %d", tc.value, id, i)
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
