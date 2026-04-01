package codec

import (
	"testing"
)

var tezosTestAddresses = []struct {
	addr string
	tag  byte
}{
	{"tz1cSj7fTex3JPd1p1LN1fwek6AV1kH93Wwc", 0x00},
	{"tz2TkyKVYyKSrhYtrvX3rSNep6bCuhuK5hSJ", 0x01},
	{"tz3cqThj23Feu55KDynm7Vg81mCMpWDgzQZq", 0x02},
	{"tz4XezFZgm3vaPavdryBv9KMFpYGdtAGWKMp", 0x03},
	{"KT1Ug9wWbRuUs1XXRuK11o6syWdTFZQsmvw3", 0x04},
	{"sr1GG29rd2XtsHkYvHEmJrWBzCAYBwkXXi6j", 0x05},
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

func b58RoundtripTest(t *testing.T, name string, c Codec, value string, binaryLen int) {
	t.Helper()
	encoded, err := c.Encode(value)
	if err != nil {
		t.Fatalf("%s Encode: %v", name, err)
	}
	if len(encoded) != binaryLen {
		t.Fatalf("%s: got %d bytes, want %d", name, len(encoded), binaryLen)
	}
	decoded, err := c.Decode(encoded)
	if err != nil {
		t.Fatalf("%s Decode: %v", name, err)
	}
	if decoded != value {
		t.Fatalf("%s roundtrip:\n  got  %s\n  want %s", name, decoded, value)
	}
}

func TestTezosBlockHash(t *testing.T) {
	b58RoundtripTest(t, "block hash", TezosBlockHashCodec,
		"BKwxR3jV27U44ssm3G7fJKXJgKn3FAeEHRyNoRK9w6DU45QDG9M", 32)
}

func TestTezosSignature(t *testing.T) {
	b58RoundtripTest(t, "signature", TezosSignatureCodec,
		"sigbmAunXeu7g8YYoBYKsBsKVvsKb7N8tSQH3z9gcutg5kYz78gGNzbeyUpHosv1jfAGTrbkf5uDHgyQg58L6HtGbn5uptSZ", 64)
}

func TestTezosOpHash(t *testing.T) {
	b58RoundtripTest(t, "operation hash", TezosOpHashCodec,
		"onpHScbMhvwJd7WY2etxKJjmEyNgX7BBjmEYKuytwNLQZ3KbVTX", 32)
}

func TestTezosProtocolHash(t *testing.T) {
	b58RoundtripTest(t, "protocol hash", TezosProtocolHashCodec,
		"PtTALLiNtPec7mE7yY4m3k26J8Qukef3E3ehzhfXgFZKGtDdAXu", 32)
}

func TestTezosChainID(t *testing.T) {
	b58RoundtripTest(t, "chain id", TezosChainIDCodec,
		"NetXdQprcVkpaWU", 4)
}

func TestTezosExprHash(t *testing.T) {
	b58RoundtripTest(t, "expression hash", TezosExprHashCodec,
		"expruE5MGe6oKRLTiog6iBZzpztj5kCGzMEYBfWzsVebPnhn43ndYa", 32)
}

func TestTezosContextHash(t *testing.T) {
	b58RoundtripTest(t, "context hash", TezosContextHashCodec,
		"CoWR81CLNoDEZBdx454giaxovUNFo67GMkY1a6hmUxF9L3GiE6cL", 32)
}

func TestTezosPayloadHash(t *testing.T) {
	b58RoundtripTest(t, "payload hash", TezosPayloadHashCodec,
		"vh2g1JN3Ck6b99QcZkMb5dDWxj8G3HmkUHVW6y9gLpRbDKSu5DsA", 32)
}

var tezosTestPubkeys = []struct {
	key       string
	tag       byte
	binaryLen int
}{
	{"edpkuxf7c72ZXnw4G3LpAhCRTjrXTB7fa7kC5jAoaZhuciXjur54dN", 0x00, 33},
	{"sppk7bn9MKAWDUFwqowcxA1zJgp12yn2kEnMQJP3WmqSZ4W8WQhLqJN", 0x01, 34},
	{"p2pk67wVncLFS1DQDm2gVR45sYCzQSXTtqn3bviNYXVCq6WRoqtxHXL", 0x02, 34},
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

func TestTezosInvalidAddress(t *testing.T) {
	codec := tezosAddressCodec{}
	if _, err := codec.Encode("tz1short"); err == nil {
		t.Fatal("expected error for short address")
	}
	if _, err := codec.Encode("tz1VSUr8wwNhLAzempoch5d6hLRiTh8Cjcjc"); err == nil {
		t.Fatal("expected error for bad checksum")
	}
}

func BenchmarkTezosEncode(b *testing.B) {
	codec := tezosAddressCodec{}
	addr := "tz1cSj7fTex3JPd1p1LN1fwek6AV1kH93Wwc"
	for i := 0; i < b.N; i++ {
		codec.Encode(addr)
	}
}
