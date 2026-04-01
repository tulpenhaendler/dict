package codec

import "testing"

var bech32Tests = []struct {
	name string
	addr string
}{
	// Bitcoin segwit v0 (bech32)
	{"btc-p2wpkh", "bc1qw508d6qejxtdg4y5r3zarvary0c5xw7kv8f3t4"},
	// Bitcoin taproot v1 (bech32m) — BIP350 test vector
	{"btc-taproot", "bc1pw508d6qejxtdg4y5r3zarvary0c5xw7kw508d6qejxtdg4y5r3zarvary0c5xw7kt5nd6y"},
	// BIP173 test vector (bech32)
	{"btc-testnet", "tb1qw508d6qejxtdg4y5r3zarvary0c5xw7kxpjzsx"},
	// BIP350 test vectors (bech32m)
	{"bech32m-empty", "a1lqfn3a"},
}

func TestBech32Roundtrip(t *testing.T) {
	c := bech32Codec{}
	for _, tc := range bech32Tests {
		enc, err := c.Encode(tc.addr)
		if err != nil {
			t.Fatalf("%s Encode(%s): %v", tc.name, tc.addr, err)
		}
		dec, err := c.Decode(enc)
		if err != nil {
			t.Fatalf("%s Decode: %v", tc.name, err)
		}
		if dec != tc.addr {
			t.Fatalf("%s roundtrip:\n  got  %s\n  want %s", tc.name, dec, tc.addr)
		}
	}
}

func TestBech32CompactSize(t *testing.T) {
	c := bech32Codec{}
	for _, tc := range bech32Tests {
		enc, _ := c.Encode(tc.addr)
		if len(enc) >= len(tc.addr) {
			t.Fatalf("%s: binary %d >= string %d", tc.name, len(enc), len(tc.addr))
		}
		t.Logf("%s: %d chars → %d bytes (%.0f%% savings)", tc.name, len(tc.addr), len(enc),
			100*(1-float64(len(enc))/float64(len(tc.addr))))
	}
}

func TestBech32Invalid(t *testing.T) {
	c := bech32Codec{}
	if _, err := c.Encode("notbech32"); err == nil {
		t.Fatal("expected error")
	}
	if _, err := c.Encode("bc1qw508d6qejxtdg4y5r3zarvary0c5xw7kv8f3t5"); err == nil {
		t.Fatal("expected error for bad checksum")
	}
}
