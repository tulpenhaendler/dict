package dict

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func tempPath(t *testing.T) string {
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

	// same key returns same id
	id0b, err := d.Get("hello", KeyRaw)
	if err != nil {
		t.Fatal(err)
	}
	if id0b != 0 {
		t.Fatalf("repeat id = %d, want 0", id0b)
	}

	ok, err := d.Exists("hello", KeyRaw)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("Exists(hello) = false, want true")
	}

	ok, err = d.Exists("missing", KeyRaw)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("Exists(missing) = true, want false")
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

	// reopen
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
		t.Fatalf("foo id = %d, want 0 after reopen", id)
	}

	s, _, _ := d.Reverse(1)
	if s != "bar" {
		t.Fatalf("Reverse(1) = %q, want bar", s)
	}
}

func TestGrow(t *testing.T) {
	p := tempPath(t)
	d, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	// insert enough entries to trigger at least one grow (initial 65536 * 0.7 ≈ 45k)
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

	// verify all exist
	for i := 0; i < n; i++ {
		key := fmt.Sprintf("key-%d", i)
		ok, err := d.Exists(key, KeyRaw)
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Fatalf("key %q not found after grow", key)
		}
	}

	// verify reverse
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

	id, err := d.Get("", KeyRaw)
	if err != nil {
		t.Fatal(err)
	}
	if id != 0 {
		t.Fatalf("empty key id = %d, want 0", id)
	}

	s, _, err := d.Reverse(0)
	if err != nil {
		t.Fatal(err)
	}
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

	// pre-insert one
	d.Get("existing", KeyRaw)

	entries := []BatchEntry{
		{"existing", KeyRaw},
		{"new1", KeyRaw},
		{"new2", KeyRaw},
		{"existing", KeyRaw}, // duplicate in same batch
		{"new1", KeyRaw},     // duplicate in same batch
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

	// first call goes to disk and populates cache
	id1, _ := d.Get("cached", KeyRaw)
	// second call should hit cache
	id2, _ := d.Get("cached", KeyRaw)
	if id1 != id2 {
		t.Fatalf("cache returned different id: %d vs %d", id1, id2)
	}

	// Exists should also hit cache
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

	id, err := d.Get("hello", KeyRaw)
	if err != nil {
		t.Fatal(err)
	}
	if id != 0 {
		t.Fatalf("id = %d, want 0", id)
	}

	id2, _ := d.Get("hello", KeyRaw)
	if id2 != 0 {
		t.Fatalf("id = %d, want 0", id2)
	}
}

func BenchmarkGet(b *testing.B) {
	dir, _ := os.MkdirTemp("", "dict-bench-*")
	defer os.RemoveAll(dir)
	p := filepath.Join(dir, "bench")

	d, err := Open(p)
	if err != nil {
		b.Fatal(err)
	}
	defer d.Close()

	// pre-populate
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
	p := filepath.Join(dir, "bench")

	d, err := OpenWithCacheSize(p, 0)
	if err != nil {
		b.Fatal(err)
	}
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
	p := filepath.Join(dir, "bench")

	d, err := Open(p)
	if err != nil {
		b.Fatal(err)
	}
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
