package codec

import "testing"

func TestBitcoinAddressRoundtrip(t *testing.T) {
	tests := []string{
		"1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", // genesis coinbase P2PKH
		"3J98t1WpEZ73CNmQviecrnyiWrnqRhWNLy", // P2SH example
	}

	c := bitcoinAddressCodec{}
	for _, addr := range tests {
		enc, err := c.Encode(addr)
		if err != nil {
			t.Fatalf("Encode(%s): %v", addr, err)
		}
		if len(enc) != 21 {
			t.Fatalf("Encode(%s): got %d bytes, want 21", addr, len(enc))
		}
		dec, err := c.Decode(enc)
		if err != nil {
			t.Fatalf("Decode: %v", err)
		}
		if dec != addr {
			t.Fatalf("roundtrip: %s -> %s", addr, dec)
		}
	}
}

func TestBitcoinAddressInvalid(t *testing.T) {
	c := bitcoinAddressCodec{}
	if _, err := c.Encode("1A1zP1eP5QGefi2DMPTfTL5SLmv7Divfxx"); err == nil {
		t.Fatal("expected error for bad checksum")
	}
}
