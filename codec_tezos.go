package dict

import "fmt"

// b58codec is a generic base58check codec for fixed-size payloads.
// Strips/prepends the version prefix and validates lengths.
type b58codec struct {
	name       string // for error messages
	prefix     string // expected base58 string prefix (e.g. "B", "sig")
	version    []byte // base58check version bytes
	payloadLen int    // decoded payload size in bytes
	strLen     int    // expected base58 string length
}

func (c b58codec) Encode(s string) ([]byte, error) {
	if len(s) != c.strLen {
		return nil, fmt.Errorf("tezos: invalid %s length %d (want %d)", c.name, len(s), c.strLen)
	}
	if len(c.prefix) > 0 && (len(s) < len(c.prefix) || s[:len(c.prefix)] != c.prefix) {
		return nil, fmt.Errorf("tezos: %s must start with %s", c.name, c.prefix)
	}
	raw, err := b58checkDecode(s)
	if err != nil {
		return nil, fmt.Errorf("tezos: %w", err)
	}
	vLen := len(c.version)
	if len(raw) != vLen+c.payloadLen {
		return nil, fmt.Errorf("tezos: unexpected %s decoded length %d (want %d)", c.name, len(raw), vLen+c.payloadLen)
	}
	for i := 0; i < vLen; i++ {
		if raw[i] != c.version[i] {
			return nil, fmt.Errorf("tezos: unexpected %s version %x", c.name, raw[:vLen])
		}
	}
	return raw[vLen:], nil
}

func (c b58codec) Decode(b []byte) (string, error) {
	if len(b) != c.payloadLen {
		return "", fmt.Errorf("tezos: invalid %s binary length %d (want %d)", c.name, len(b), c.payloadLen)
	}
	vLen := len(c.version)
	payload := make([]byte, vLen+c.payloadLen)
	copy(payload[:vLen], c.version)
	copy(payload[vLen:], b)
	return b58checkEncode(payload), nil
}

// --- Simple b58 codecs (single version prefix, fixed payload) ---

var tezosBlockHashCodec = b58codec{
	name: "block hash", prefix: "B",
	version: []byte{0x01, 0x34}, payloadLen: 32, strLen: 51,
}

var tezosSignatureCodec = b58codec{
	name: "signature", prefix: "sig",
	version: []byte{0x04, 0x82, 0x2B}, payloadLen: 64, strLen: 96,
}

var tezosOpHashCodec = b58codec{
	name: "operation hash", prefix: "o",
	version: []byte{0x05, 0x74}, payloadLen: 32, strLen: 51,
}

var tezosProtocolHashCodec = b58codec{
	name: "protocol hash", prefix: "P",
	version: []byte{0x02, 0xAA}, payloadLen: 32, strLen: 51,
}

var tezosChainIDCodec = b58codec{
	name: "chain id", prefix: "Net",
	version: []byte{0x57, 0x52, 0x00}, payloadLen: 4, strLen: 15,
}

var tezosExprHashCodec = b58codec{
	name: "expression hash", prefix: "expr",
	version: []byte{0x0D, 0x2C, 0x40, 0x1B}, payloadLen: 32, strLen: 54,
}

var tezosContextHashCodec = b58codec{
	name: "context hash", prefix: "Co",
	version: []byte{0x4F, 0xC7}, payloadLen: 32, strLen: 52,
}

var tezosPayloadHashCodec = b58codec{
	name: "payload hash", prefix: "vh",
	version: []byte{0x01, 0x6A, 0xF2}, payloadLen: 32, strLen: 52,
}

// --- Tezos addresses: 36 chars → 21 bytes [1 tag + 20 hash] ---
//
// Multiple base58 prefixes (tz1, tz2, tz3, tz4, KT1, sr1) map to a
// single KeyType with a discriminator tag byte.

type tezosAddrType struct {
	tag     byte
	version [3]byte
}

var tezosAddrTypes = []tezosAddrType{
	{0x00, [3]byte{0x06, 0xA1, 0x9F}}, // tz1
	{0x01, [3]byte{0x06, 0xA1, 0xA1}}, // tz2
	{0x02, [3]byte{0x06, 0xA1, 0xA4}}, // tz3
	{0x03, [3]byte{0x06, 0xA1, 0xA6}}, // tz4
	{0x04, [3]byte{0x02, 0x5A, 0x79}}, // KT1
	{0x05, [3]byte{0x06, 0x7C, 0x75}}, // sr1
}

type tezosAddressCodec struct{}

func (tezosAddressCodec) Encode(s string) ([]byte, error) {
	if len(s) != 36 {
		return nil, fmt.Errorf("tezos: invalid address length %d (want 36)", len(s))
	}
	raw, err := b58checkDecode(s)
	if err != nil {
		return nil, fmt.Errorf("tezos: %w", err)
	}
	if len(raw) != 23 {
		return nil, fmt.Errorf("tezos: unexpected decoded length %d", len(raw))
	}
	var ver [3]byte
	copy(ver[:], raw[:3])
	for _, at := range tezosAddrTypes {
		if at.version == ver {
			out := make([]byte, 21)
			out[0] = at.tag
			copy(out[1:], raw[3:])
			return out, nil
		}
	}
	return nil, fmt.Errorf("tezos: unknown address version %x", ver)
}

func (tezosAddressCodec) Decode(b []byte) (string, error) {
	if len(b) != 21 {
		return "", fmt.Errorf("tezos: invalid address binary length %d (want 21)", len(b))
	}
	for _, at := range tezosAddrTypes {
		if at.tag == b[0] {
			payload := make([]byte, 23)
			copy(payload[:3], at.version[:])
			copy(payload[3:], b[1:])
			return b58checkEncode(payload), nil
		}
	}
	return "", fmt.Errorf("tezos: unknown address tag %d", b[0])
}

// --- Tezos public keys: variable-length → 1 tag + payload ---
//
// edpk (32 bytes), sppk (33), p2pk (33), BLpk (48) have different
// payload sizes, so we store [1 tag][payload] with max 49 bytes.

type tezosPKType struct {
	tag        byte
	version    []byte
	payloadLen int
	strLen     int
}

var tezosPKTypes = []tezosPKType{
	{0x00, []byte{0x0D, 0x0F, 0x25, 0xD9}, 32, 54}, // edpk
	{0x01, []byte{0x03, 0xFE, 0xE2, 0x56}, 33, 55}, // sppk
	{0x02, []byte{0x03, 0xB2, 0x8B, 0x7F}, 33, 55}, // p2pk
	{0x03, []byte{0x06, 0x95, 0x87, 0xCC}, 48, 76}, // BLpk
}

type tezosPubkeyCodec struct{}

func (tezosPubkeyCodec) Encode(s string) ([]byte, error) {
	for _, pk := range tezosPKTypes {
		if len(s) != pk.strLen {
			continue
		}
		raw, err := b58checkDecode(s)
		if err != nil {
			continue
		}
		vLen := len(pk.version)
		if len(raw) != vLen+pk.payloadLen {
			continue
		}
		match := true
		for i := 0; i < vLen; i++ {
			if raw[i] != pk.version[i] {
				match = false
				break
			}
		}
		if match {
			out := make([]byte, 1+pk.payloadLen)
			out[0] = pk.tag
			copy(out[1:], raw[vLen:])
			return out, nil
		}
	}
	return nil, fmt.Errorf("tezos: unrecognized public key %q", s)
}

func (tezosPubkeyCodec) Decode(b []byte) (string, error) {
	if len(b) < 2 {
		return "", fmt.Errorf("tezos: public key too short")
	}
	for _, pk := range tezosPKTypes {
		if pk.tag == b[0] && len(b) == 1+pk.payloadLen {
			vLen := len(pk.version)
			payload := make([]byte, vLen+pk.payloadLen)
			copy(payload[:vLen], pk.version)
			copy(payload[vLen:], b[1:])
			return b58checkEncode(payload), nil
		}
	}
	return "", fmt.Errorf("tezos: unknown public key tag %d", b[0])
}
