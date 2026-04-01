package codec

import "testing"

func TestSS58Roundtrip(t *testing.T) {
	tests := []string{
		// Polkadot (network 0)
		"15oF4uVJwmo4TdGW7VfQxNLavjCXviqWrztPu5DBHC1VJduW",
		// Kusama (network 2)
		"HNZata7iMYWmk5RvZRTiAsSDhV8366zq2YGb3tLH5Upf74F",
	}

	c := ss58Codec{}
	for _, addr := range tests {
		enc, err := c.Encode(addr)
		if err != nil {
			t.Fatalf("Encode(%s): %v", addr, err)
		}
		if len(enc) != 35 && len(enc) != 36 {
			t.Fatalf("Encode(%s): got %d bytes", addr, len(enc))
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
