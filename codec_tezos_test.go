package dict

import (
	"testing"
)

var tezosTestAddresses = []struct {
	addr string
	tag  byte
}{
	{"tz1cSj7fTex3JPd1p1LN1fwek6AV1kH93Wwc", 0x00},  // tz1
	{"tz2TkyKVYyKSrhYtrvX3rSNep6bCuhuK5hSJ", 0x01},  // tz2
	{"tz3cqThj23Feu55KDynm7Vg81mCMpWDgzQZq", 0x02},  // tz3
	{"tz4XezFZgm3vaPavdryBv9KMFpYGdtAGWKMp", 0x03},  // tz4
	{"KT1Ug9wWbRuUs1XXRuK11o6syWdTFZQsmvw3", 0x04},  // KT1
	{"sr1GG29rd2XtsHkYvHEmJrWBzCAYBwkXXi6j", 0x05},  // sr1
}

func TestTezosAddressRoundtrip(t *testing.T) {
	codec := tezosAddressCodec{}
	for _, tc := range tezosTestAddresses {
		encoded, err := codec.Encode(tc.addr)
		if err != nil {
			t.Fatalf("Encode(%s): %v", tc.addr, err)
		}
		if len(encoded) != 21 {
			t.Fatalf("Encode(%s): got %d bytes, want 21", tc.addr, len(encoded))
		}
		if encoded[0] != tc.tag {
			t.Fatalf("Encode(%s): tag = %d, want %d", tc.addr, encoded[0], tc.tag)
		}
		decoded, err := codec.Decode(encoded)
		if err != nil {
			t.Fatalf("Decode: %v", err)
		}
		if decoded != tc.addr {
			t.Fatalf("roundtrip: %s -> %s", tc.addr, decoded)
		}
	}
}

func TestTezosAddressInDict(t *testing.T) {
	d, err := Open(tempPath(t))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	for i, tc := range tezosTestAddresses {
		id, err := d.Get(tc.addr, KeyTezosAddress)
		if err != nil {
			t.Fatalf("Get(%s): %v", tc.addr, err)
		}
		if id != uint32(i) {
			t.Fatalf("Get(%s) = %d, want %d", tc.addr, id, i)
		}
	}
	// upsert
	for i, tc := range tezosTestAddresses {
		id, _ := d.Get(tc.addr, KeyTezosAddress)
		if id != uint32(i) {
			t.Fatalf("upsert %s: got %d, want %d", tc.addr, id, i)
		}
	}
	// reverse
	for i, tc := range tezosTestAddresses {
		s, kt, err := d.Reverse(uint32(i))
		if err != nil {
			t.Fatal(err)
		}
		if kt != KeyTezosAddress || s != tc.addr {
			t.Fatalf("Reverse(%d) = %q, want %q", i, s, tc.addr)
		}
	}
}

// b58RoundtripTest tests a b58codec or any Codec with a known good value.
func b58RoundtripTest(t *testing.T, name string, codec Codec, keyType KeyType, value string, binaryLen int) {
	t.Helper()

	encoded, err := codec.Encode(value)
	if err != nil {
		t.Fatalf("%s Encode: %v", name, err)
	}
	if len(encoded) != binaryLen {
		t.Fatalf("%s: got %d bytes, want %d", name, len(encoded), binaryLen)
	}
	decoded, err := codec.Decode(encoded)
	if err != nil {
		t.Fatalf("%s Decode: %v", name, err)
	}
	if decoded != value {
		t.Fatalf("%s roundtrip:\n  got  %s\n  want %s", name, decoded, value)
	}

	// test through dict
	d, err := Open(tempPath(t))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	id, err := d.Get(value, keyType)
	if err != nil {
		t.Fatalf("%s dict Get: %v", name, err)
	}
	if id != 0 {
		t.Fatalf("%s id = %d, want 0", name, id)
	}
	id2, _ := d.Get(value, keyType)
	if id2 != 0 {
		t.Fatalf("%s upsert = %d, want 0", name, id2)
	}
	s, kt, _ := d.Reverse(0)
	if kt != keyType || s != value {
		t.Fatalf("%s reverse mismatch: got %q", name, s)
	}
}

func TestTezosBlockHash(t *testing.T) {
	b58RoundtripTest(t, "block hash",
		tezosBlockHashCodec, KeyTezosBlockHash,
		"BKwxR3jV27U44ssm3G7fJKXJgKn3FAeEHRyNoRK9w6DU45QDG9M", 32)
}

func TestTezosSignature(t *testing.T) {
	b58RoundtripTest(t, "signature",
		tezosSignatureCodec, KeyTezosSignature,
		"sigbmAunXeu7g8YYoBYKsBsKVvsKb7N8tSQH3z9gcutg5kYz78gGNzbeyUpHosv1jfAGTrbkf5uDHgyQg58L6HtGbn5uptSZ", 64)
}

func TestTezosOpHash(t *testing.T) {
	b58RoundtripTest(t, "operation hash",
		tezosOpHashCodec, KeyTezosOpHash,
		"onpHScbMhvwJd7WY2etxKJjmEyNgX7BBjmEYKuytwNLQZ3KbVTX", 32)
}

func TestTezosProtocolHash(t *testing.T) {
	b58RoundtripTest(t, "protocol hash",
		tezosProtocolHashCodec, KeyTezosProtocolHash,
		"PtTALLiNtPec7mE7yY4m3k26J8Qukef3E3ehzhfXgFZKGtDdAXu", 32)
}

func TestTezosChainID(t *testing.T) {
	b58RoundtripTest(t, "chain id",
		tezosChainIDCodec, KeyTezosChainID,
		"NetXdQprcVkpaWU", 4)
}

func TestTezosExprHash(t *testing.T) {
	b58RoundtripTest(t, "expression hash",
		tezosExprHashCodec, KeyTezosExprHash,
		"expruE5MGe6oKRLTiog6iBZzpztj5kCGzMEYBfWzsVebPnhn43ndYa", 32)
}

func TestTezosContextHash(t *testing.T) {
	b58RoundtripTest(t, "context hash",
		tezosContextHashCodec, KeyTezosContextHash,
		"CoWR81CLNoDEZBdx454giaxovUNFo67GMkY1a6hmUxF9L3GiE6cL", 32)
}

func TestTezosPayloadHash(t *testing.T) {
	b58RoundtripTest(t, "payload hash",
		tezosPayloadHashCodec, KeyTezosPayloadHash,
		"vh2g1JN3Ck6b99QcZkMb5dDWxj8G3HmkUHVW6y9gLpRbDKSu5DsA", 32)
}

var tezosTestPubkeys = []struct {
	key       string
	tag       byte
	binaryLen int
}{
	{"edpkuxf7c72ZXnw4G3LpAhCRTjrXTB7fa7kC5jAoaZhuciXjur54dN", 0x00, 33}, // edpk: 1 tag + 32
	{"sppk7bn9MKAWDUFwqowcxA1zJgp12yn2kEnMQJP3WmqSZ4W8WQhLqJN", 0x01, 34}, // sppk: 1 tag + 33
	{"p2pk67wVncLFS1DQDm2gVR45sYCzQSXTtqn3bviNYXVCq6WRoqtxHXL", 0x02, 34}, // p2pk: 1 tag + 33
}

func TestTezosPubkeyRoundtrip(t *testing.T) {
	codec := tezosPubkeyCodec{}
	for _, tc := range tezosTestPubkeys {
		encoded, err := codec.Encode(tc.key)
		if err != nil {
			t.Fatalf("Encode(%s): %v", tc.key, err)
		}
		if len(encoded) != tc.binaryLen {
			t.Fatalf("Encode(%s): got %d bytes, want %d", tc.key, len(encoded), tc.binaryLen)
		}
		if encoded[0] != tc.tag {
			t.Fatalf("Encode(%s): tag = %d, want %d", tc.key, encoded[0], tc.tag)
		}
		decoded, err := codec.Decode(encoded)
		if err != nil {
			t.Fatalf("Decode: %v", err)
		}
		if decoded != tc.key {
			t.Fatalf("roundtrip: %s -> %s", tc.key, decoded)
		}
	}
}

func TestTezosPubkeyInDict(t *testing.T) {
	d, err := Open(tempPath(t))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	for i, tc := range tezosTestPubkeys {
		id, err := d.Get(tc.key, KeyTezosPubkey)
		if err != nil {
			t.Fatalf("Get(%s): %v", tc.key, err)
		}
		if id != uint32(i) {
			t.Fatalf("Get(%s) = %d, want %d", tc.key, id, i)
		}
	}
	for i, tc := range tezosTestPubkeys {
		s, kt, _ := d.Reverse(uint32(i))
		if kt != KeyTezosPubkey || s != tc.key {
			t.Fatalf("Reverse(%d) = %q, want %q", i, s, tc.key)
		}
	}
}

func TestTezosInvalidAddress(t *testing.T) {
	codec := tezosAddressCodec{}
	if _, err := codec.Encode("tz1short"); err == nil {
		t.Fatal("expected error for short address")
	}
	if _, err := codec.Encode("tz1VSUr8wwNhLAzempoch5d6hLRiTh8Cjcjc"); err == nil {
		t.Fatal("expected error for bad checksum")
	}
}

func TestTezosNoCollisionWithRaw(t *testing.T) {
	d, err := Open(tempPath(t))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	addr := tezosTestAddresses[0].addr
	idRaw, _ := d.Get(addr, KeyRaw)
	idTezos, _ := d.Get(addr, KeyTezosAddress)
	if idRaw == idTezos {
		t.Fatalf("raw and tezos got same ID %d", idRaw)
	}
}

func BenchmarkTezosEncode(b *testing.B) {
	codec := tezosAddressCodec{}
	addr := "tz1cSj7fTex3JPd1p1LN1fwek6AV1kH93Wwc"
	for i := 0; i < b.N; i++ {
		codec.Encode(addr)
	}
}

func BenchmarkTezosGet(b *testing.B) {
	dir := b.TempDir()
	d, err := Open(dir + "/bench")
	if err != nil {
		b.Fatal(err)
	}
	defer d.Close()

	for _, tc := range tezosTestAddresses {
		d.Get(tc.addr, KeyTezosAddress)
	}

	addr := tezosTestAddresses[0].addr
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.Get(addr, KeyTezosAddress)
	}
}
