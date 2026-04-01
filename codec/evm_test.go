package codec

import "testing"

func TestEVMAddressRoundtrip(t *testing.T) {
	addr := "0x742d35Cc6634C0532925a3b844Bc9e7595f2bD18"
	c := evmAddressCodec{}

	enc, err := c.Encode(addr)
	if err != nil {
		t.Fatal(err)
	}
	if len(enc) != 20 {
		t.Fatalf("got %d bytes, want 20", len(enc))
	}

	dec, err := c.Decode(enc)
	if err != nil {
		t.Fatal(err)
	}
	// decode returns lowercase
	if dec != "0x742d35cc6634c0532925a3b844bc9e7595f2bd18" {
		t.Fatalf("decode: %s", dec)
	}
}

func TestEVMHash32Roundtrip(t *testing.T) {
	txHash := "0xd4e56740f876aef8c010b86a40d5f56745a118d0906a34e69aec8c0db1cb8fa3"
	c := evmHash32Codec{}

	enc, err := c.Encode(txHash)
	if err != nil {
		t.Fatal(err)
	}
	if len(enc) != 32 {
		t.Fatalf("got %d bytes, want 32", len(enc))
	}

	dec, _ := c.Decode(enc)
	if dec != txHash {
		t.Fatalf("roundtrip: %s", dec)
	}
}

func TestEVMSelectorRoundtrip(t *testing.T) {
	sel := "0xa9059cbb" // transfer(address,uint256)
	c := evmSelectorCodec{}

	enc, err := c.Encode(sel)
	if err != nil {
		t.Fatal(err)
	}
	if len(enc) != 4 {
		t.Fatalf("got %d bytes, want 4", len(enc))
	}

	dec, _ := c.Decode(enc)
	if dec != sel {
		t.Fatalf("roundtrip: %s", dec)
	}
}

func TestEVMAddressCaseInsensitive(t *testing.T) {
	c := evmAddressCodec{}
	enc1, _ := c.Encode("0x742d35Cc6634C0532925a3b844Bc9e7595f2bD18")
	enc2, _ := c.Encode("0x742d35cc6634c0532925a3b844bc9e7595f2bd18")

	if len(enc1) != len(enc2) {
		t.Fatal("different lengths")
	}
	for i := range enc1 {
		if enc1[i] != enc2[i] {
			t.Fatalf("byte %d differs", i)
		}
	}
}

func TestEVMInvalid(t *testing.T) {
	c := evmAddressCodec{}
	if _, err := c.Encode("0xshort"); err == nil {
		t.Fatal("expected error")
	}
	if _, err := c.Encode("not-hex-at-all"); err == nil {
		t.Fatal("expected error")
	}
}
