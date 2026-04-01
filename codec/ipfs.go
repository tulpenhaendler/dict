package codec

import (
	"fmt"
)

// IPFS CID codec — auto-detects CIDv0 (Qm...) and CIDv1 (bafy.../bafk.../bafyr...).
//
// Compact binary format:
//   CIDv0: [tag=0x00] [32 bytes sha256 digest] = 33 bytes
//   CIDv1: [tag]      [32 bytes sha256 digest] = 33 bytes
//
// CIDv1 tags encode the multicodec:
//   0x01 = dag-pb  (0x70)
//   0x02 = raw     (0x55)
//   0x03 = dag-cbor(0x71)

const (
	cidTagV0      = 0x00
	cidTagDagPB   = 0x01
	cidTagRaw     = 0x02
	cidTagDagCBOR = 0x03
)

type cidVariant struct {
	tag        byte
	multicodec uint64
}

var cidV1Variants = []cidVariant{
	{cidTagDagPB, 0x70},
	{cidTagRaw, 0x55},
	{cidTagDagCBOR, 0x71},
}

type ipfsCIDCodec struct{}

func (ipfsCIDCodec) Encode(s string) ([]byte, error) {
	if len(s) == 46 && s[0] == 'Q' && s[1] == 'm' {
		return encodeCIDv0(s)
	}
	if len(s) > 1 && s[0] == 'b' {
		return encodeCIDv1(s)
	}
	return nil, fmt.Errorf("ipfs: unrecognized CID format %q", truncate(s, 20))
}

func (ipfsCIDCodec) Decode(b []byte) (string, error) {
	if len(b) < 2 {
		return "", fmt.Errorf("ipfs: CID too short")
	}
	if b[0] == cidTagV0 {
		return decodeCIDv0(b)
	}
	return decodeCIDv1(b)
}

// --- CIDv0: Qm... (base58, no checksum) → 34 bytes [0x12, 0x20, 32-byte digest] ---

func encodeCIDv0(s string) ([]byte, error) {
	raw, err := b58decode(s)
	if err != nil {
		return nil, fmt.Errorf("ipfs: CIDv0 base58: %w", err)
	}
	if len(raw) != 34 || raw[0] != 0x12 || raw[1] != 0x20 {
		return nil, fmt.Errorf("ipfs: CIDv0 not sha2-256 multihash (len=%d)", len(raw))
	}
	out := make([]byte, 33)
	out[0] = cidTagV0
	copy(out[1:], raw[2:]) // 32-byte digest
	return out, nil
}

func decodeCIDv0(b []byte) (string, error) {
	if len(b) != 33 {
		return "", fmt.Errorf("ipfs: invalid CIDv0 binary length %d", len(b))
	}
	raw := make([]byte, 34)
	raw[0] = 0x12
	raw[1] = 0x20
	copy(raw[2:], b[1:])
	return b58encode(raw), nil
}

// --- CIDv1: b... (base32-lower, multibase prefix 'b') ---

func encodeCIDv1(s string) ([]byte, error) {
	if s[0] != 'b' {
		return nil, fmt.Errorf("ipfs: unsupported CIDv1 multibase %q (only base32lower supported)", s[0:1])
	}
	raw, err := base32LowerDecode(s[1:])
	if err != nil {
		return nil, fmt.Errorf("ipfs: CIDv1 base32: %w", err)
	}
	// Parse: version (varint) + codec (varint) + multihash
	pos := 0
	version, n := uvarint(raw[pos:])
	if n <= 0 || version != 1 {
		return nil, fmt.Errorf("ipfs: expected CIDv1, got version %d", version)
	}
	pos += n

	mc, n := uvarint(raw[pos:])
	if n <= 0 {
		return nil, fmt.Errorf("ipfs: invalid multicodec varint")
	}
	pos += n

	// Parse multihash: fn (varint) + digestLen (varint) + digest
	hashFn, n := uvarint(raw[pos:])
	if n <= 0 {
		return nil, fmt.Errorf("ipfs: invalid multihash function varint")
	}
	pos += n

	digestLen, n := uvarint(raw[pos:])
	if n <= 0 {
		return nil, fmt.Errorf("ipfs: invalid multihash digest length varint")
	}
	pos += n

	if hashFn != 0x12 || digestLen != 32 {
		return nil, fmt.Errorf("ipfs: unsupported hash function 0x%x/%d (only sha2-256/32)", hashFn, digestLen)
	}
	if pos+32 > len(raw) {
		return nil, fmt.Errorf("ipfs: CIDv1 truncated")
	}

	var tag byte
	found := false
	for _, v := range cidV1Variants {
		if v.multicodec == mc {
			tag = v.tag
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("ipfs: unsupported multicodec 0x%x", mc)
	}

	out := make([]byte, 33)
	out[0] = tag
	copy(out[1:], raw[pos:pos+32])
	return out, nil
}

func decodeCIDv1(b []byte) (string, error) {
	if len(b) != 33 {
		return "", fmt.Errorf("ipfs: invalid CIDv1 binary length %d", len(b))
	}
	var mc uint64
	found := false
	for _, v := range cidV1Variants {
		if v.tag == b[0] {
			mc = v.multicodec
			found = true
			break
		}
	}
	if !found {
		return "", fmt.Errorf("ipfs: unknown CID tag %d", b[0])
	}

	// Rebuild: version(1) + codec(varint) + sha2-256 multihash header + digest
	var raw [64]byte
	pos := 0
	pos += putUvarint(raw[pos:], 1)  // CID version
	pos += putUvarint(raw[pos:], mc) // multicodec
	raw[pos] = 0x12                  // sha2-256
	pos++
	raw[pos] = 0x20 // digest length 32
	pos++
	copy(raw[pos:], b[1:]) // 32-byte digest
	pos += 32

	return "b" + base32LowerEncode(raw[:pos]), nil
}

// --- base32 lower (RFC 4648, no padding) ---

const b32alphabet = "abcdefghijklmnopqrstuvwxyz234567"

func base32LowerEncode(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	out := make([]byte, (len(data)*8+4)/5)
	bits := 0
	buf := 0
	pos := 0
	for _, b := range data {
		buf = (buf << 8) | int(b)
		bits += 8
		for bits >= 5 {
			bits -= 5
			out[pos] = b32alphabet[(buf>>bits)&0x1F]
			pos++
		}
	}
	if bits > 0 {
		out[pos] = b32alphabet[(buf<<(5-bits))&0x1F]
		pos++
	}
	return string(out[:pos])
}

var b32table [256]int8

func init() {
	for i := range b32table {
		b32table[i] = -1
	}
	for i, c := range b32alphabet {
		b32table[c] = int8(i)
	}
}

func base32LowerDecode(s string) ([]byte, error) {
	out := make([]byte, len(s)*5/8)
	bits := 0
	buf := 0
	pos := 0
	for _, c := range []byte(s) {
		v := b32table[c]
		if v < 0 {
			return nil, fmt.Errorf("base32: invalid character %q", c)
		}
		buf = (buf << 5) | int(v)
		bits += 5
		if bits >= 8 {
			bits -= 8
			out[pos] = byte(buf >> bits)
			pos++
		}
	}
	return out[:pos], nil
}

// --- unsigned varint (LEB128) ---

func uvarint(buf []byte) (uint64, int) {
	var x uint64
	var s uint
	for i, b := range buf {
		if i >= 10 {
			return 0, -1
		}
		if b < 0x80 {
			return x | uint64(b)<<s, i + 1
		}
		x |= uint64(b&0x7f) << s
		s += 7
	}
	return 0, 0
}

func putUvarint(buf []byte, x uint64) int {
	i := 0
	for x >= 0x80 {
		buf[i] = byte(x) | 0x80
		x >>= 7
		i++
	}
	buf[i] = byte(x)
	return i + 1
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

