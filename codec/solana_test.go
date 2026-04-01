package codec

import "testing"

func TestSolanaAddressRoundtrip(t *testing.T) {
	// Solana system program
	addr := "11111111111111111111111111111111"
	c := solanaAddressCodec{}

	enc, err := c.Encode(addr)
	if err != nil {
		t.Fatal(err)
	}
	if len(enc) != 32 {
		t.Fatalf("got %d bytes, want 32", len(enc))
	}
	dec, _ := c.Decode(enc)
	if dec != addr {
		t.Fatalf("roundtrip: %s -> %s", addr, dec)
	}
}

func TestSolanaAddressReal(t *testing.T) {
	// Token program
	addr := "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"
	c := solanaAddressCodec{}

	enc, err := c.Encode(addr)
	if err != nil {
		t.Fatal(err)
	}
	if len(enc) != 32 {
		t.Fatalf("got %d bytes, want 32", len(enc))
	}
	dec, _ := c.Decode(enc)
	if dec != addr {
		t.Fatalf("roundtrip: %s -> %s", addr, dec)
	}
}

func TestSolanaSigRoundtrip(t *testing.T) {
	// Example tx signature (88 chars base58 → 64 bytes)
	sig := "5VERv8NMvzbJMEkV8xnrLkEaWRtSz9CosKDYjCJjBRnbJLgp8uirBgmQpjKhoR4tjF3ZpRzrFmBV6UjKdiSZkQU"
	c := solanaSigCodec{}

	enc, err := c.Encode(sig)
	if err != nil {
		t.Fatal(err)
	}
	if len(enc) != 64 {
		t.Fatalf("got %d bytes, want 64", len(enc))
	}
	dec, _ := c.Decode(enc)
	if dec != sig {
		t.Fatalf("roundtrip failed")
	}
}
