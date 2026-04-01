package codec

import "testing"

func TestNumericStringRoundtrip(t *testing.T) {
	tests := []struct {
		s         string
		maxBytes  int
	}{
		{"0", 1},
		{"127", 1},
		{"128", 2},
		{"12345678", 4},
		{"18446744073709551615", 10}, // max uint64
	}

	c := numericStringCodec{}
	for _, tc := range tests {
		enc, err := c.Encode(tc.s)
		if err != nil {
			t.Fatalf("Encode(%s): %v", tc.s, err)
		}
		if len(enc) > tc.maxBytes {
			t.Fatalf("Encode(%s): %d bytes > max %d", tc.s, len(enc), tc.maxBytes)
		}
		dec, err := c.Decode(enc)
		if err != nil {
			t.Fatalf("Decode: %v", err)
		}
		if dec != tc.s {
			t.Fatalf("roundtrip: %s -> %s", tc.s, dec)
		}
		t.Logf("%s: %d chars → %d bytes", tc.s, len(tc.s), len(enc))
	}
}

func TestNumericStringInvalid(t *testing.T) {
	c := numericStringCodec{}
	if _, err := c.Encode("not-a-number"); err == nil {
		t.Fatal("expected error")
	}
	if _, err := c.Encode("-1"); err == nil {
		t.Fatal("expected error for negative")
	}
}
