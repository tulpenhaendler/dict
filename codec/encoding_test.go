package codec

import "testing"

func TestHexCodecRoundtrip(t *testing.T) {
	c := hexCodec{}
	tests := []string{"0xdeadbeef", "0x00", "0xff00ff"}
	for _, s := range tests {
		enc, err := c.Encode(s)
		if err != nil {
			t.Fatalf("Encode(%s): %v", s, err)
		}
		dec, _ := c.Decode(enc)
		if dec != s {
			t.Fatalf("roundtrip: %s -> %s", s, dec)
		}
	}
}

func TestBase64CodecRoundtrip(t *testing.T) {
	c := base64Codec{}
	s := "SGVsbG8gV29ybGQ=" // "Hello World"
	enc, err := c.Encode(s)
	if err != nil {
		t.Fatal(err)
	}
	if string(enc) != "Hello World" {
		t.Fatalf("decoded to %q", enc)
	}
	dec, _ := c.Decode(enc)
	if dec != s {
		t.Fatalf("roundtrip: %s", dec)
	}
}

func TestBase58CodecRoundtrip(t *testing.T) {
	c := base58Codec{}
	// "Hello World" in base58 (no checksum)
	s := "JxF12TrwUP45BMd"
	enc, err := c.Encode(s)
	if err != nil {
		t.Fatal(err)
	}
	if string(enc) != "Hello World" {
		t.Fatalf("decoded to %q", enc)
	}
	dec, _ := c.Decode(enc)
	if dec != s {
		t.Fatalf("roundtrip: %s -> %s", s, dec)
	}
}
