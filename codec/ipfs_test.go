package codec

import "testing"

func TestIPFSCIDv0Roundtrip(t *testing.T) {
	// Real CIDv0 — sha2-256 multihash of "hello world\n"
	cid := "QmT78zSuBmuS4z925WZfrqQ1qHaJ56DQaTfyMUF7F8ff5o"
	c := ipfsCIDCodec{}

	enc, err := c.Encode(cid)
	if err != nil {
		t.Fatal(err)
	}
	if len(enc) != 33 {
		t.Fatalf("got %d bytes, want 33", len(enc))
	}
	if enc[0] != cidTagV0 {
		t.Fatalf("tag = %d, want %d", enc[0], cidTagV0)
	}

	dec, err := c.Decode(enc)
	if err != nil {
		t.Fatal(err)
	}
	if dec != cid {
		t.Fatalf("roundtrip:\n  got  %s\n  want %s", dec, cid)
	}
}

func TestIPFSCIDv1DagPBRoundtrip(t *testing.T) {
	// CIDv1 dag-pb, sha2-256 — base32lower
	cid := "bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi"
	c := ipfsCIDCodec{}

	enc, err := c.Encode(cid)
	if err != nil {
		t.Fatal(err)
	}
	if len(enc) != 33 {
		t.Fatalf("got %d bytes, want 33", len(enc))
	}
	if enc[0] != cidTagDagPB {
		t.Fatalf("tag = %d, want %d (dag-pb)", enc[0], cidTagDagPB)
	}

	dec, err := c.Decode(enc)
	if err != nil {
		t.Fatal(err)
	}
	if dec != cid {
		t.Fatalf("roundtrip:\n  got  %s\n  want %s", dec, cid)
	}
}

func TestIPFSCIDv1RawRoundtrip(t *testing.T) {
	// CIDv1 raw, sha2-256
	cid := "bafkreihdwdcefgh4dqkjv67uzcmw7ojee6xedzdetojuzjevtenosa7tiy"
	c := ipfsCIDCodec{}

	enc, err := c.Encode(cid)
	if err != nil {
		t.Fatal(err)
	}
	if len(enc) != 33 {
		t.Fatalf("got %d bytes, want 33", len(enc))
	}
	if enc[0] != cidTagRaw {
		t.Fatalf("tag = %d, want %d (raw)", enc[0], cidTagRaw)
	}

	dec, err := c.Decode(enc)
	if err != nil {
		t.Fatal(err)
	}
	if dec != cid {
		t.Fatalf("roundtrip:\n  got  %s\n  want %s", dec, cid)
	}
}

func TestIPFSInvalid(t *testing.T) {
	c := ipfsCIDCodec{}

	if _, err := c.Encode("not-a-cid"); err == nil {
		t.Fatal("expected error")
	}
	if _, err := c.Encode("Qmshort"); err == nil {
		t.Fatal("expected error for short CIDv0")
	}
}
