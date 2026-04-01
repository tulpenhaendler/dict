package codec

import "fmt"

// B58Codec is a generic base58check codec for fixed-size payloads.
// Strips/prepends the version prefix and validates lengths.
type B58Codec struct {
	Name       string // for error messages
	Prefix     string // expected base58 string prefix (e.g. "B", "sig")
	Version    []byte // base58check version bytes
	PayloadLen int    // decoded payload size in bytes
	StrLen     int    // expected base58 string length
}

func (c B58Codec) Encode(s string) ([]byte, error) {
	if len(s) != c.StrLen {
		return nil, fmt.Errorf("tezos: invalid %s length %d (want %d)", c.Name, len(s), c.StrLen)
	}
	if len(c.Prefix) > 0 && (len(s) < len(c.Prefix) || s[:len(c.Prefix)] != c.Prefix) {
		return nil, fmt.Errorf("tezos: %s must start with %s", c.Name, c.Prefix)
	}
	raw, err := b58checkDecode(s)
	if err != nil {
		return nil, fmt.Errorf("tezos: %w", err)
	}
	vLen := len(c.Version)
	if len(raw) != vLen+c.PayloadLen {
		return nil, fmt.Errorf("tezos: unexpected %s decoded length %d (want %d)", c.Name, len(raw), vLen+c.PayloadLen)
	}
	for i := 0; i < vLen; i++ {
		if raw[i] != c.Version[i] {
			return nil, fmt.Errorf("tezos: unexpected %s version %x", c.Name, raw[:vLen])
		}
	}
	return raw[vLen:], nil
}

func (c B58Codec) Decode(b []byte) (string, error) {
	if len(b) != c.PayloadLen {
		return "", fmt.Errorf("tezos: invalid %s binary length %d (want %d)", c.Name, len(b), c.PayloadLen)
	}
	vLen := len(c.Version)
	payload := make([]byte, vLen+c.PayloadLen)
	copy(payload[:vLen], c.Version)
	copy(payload[vLen:], b)
	return b58checkEncode(payload), nil
}

// --- Simple b58 codecs (single version prefix, fixed payload) ---

var TezosBlockHashCodec = B58Codec{
	Name: "block hash", Prefix: "B",
	Version: []byte{0x01, 0x34}, PayloadLen: 32, StrLen: 51,
}

var TezosSignatureCodec = B58Codec{
	Name: "signature", Prefix: "sig",
	Version: []byte{0x04, 0x82, 0x2B}, PayloadLen: 64, StrLen: 96,
}

var TezosOpHashCodec = B58Codec{
	Name: "operation hash", Prefix: "o",
	Version: []byte{0x05, 0x74}, PayloadLen: 32, StrLen: 51,
}

var TezosProtocolHashCodec = B58Codec{
	Name: "protocol hash", Prefix: "P",
	Version: []byte{0x02, 0xAA}, PayloadLen: 32, StrLen: 51,
}

var TezosChainIDCodec = B58Codec{
	Name: "chain id", Prefix: "Net",
	Version: []byte{0x57, 0x52, 0x00}, PayloadLen: 4, StrLen: 15,
}

var TezosExprHashCodec = B58Codec{
	Name: "expression hash", Prefix: "expr",
	Version: []byte{0x0D, 0x2C, 0x40, 0x1B}, PayloadLen: 32, StrLen: 54,
}

var TezosContextHashCodec = B58Codec{
	Name: "context hash", Prefix: "Co",
	Version: []byte{0x4F, 0xC7}, PayloadLen: 32, StrLen: 52,
}

var TezosPayloadHashCodec = B58Codec{
	Name: "payload hash", Prefix: "vh",
	Version: []byte{0x01, 0x6A, 0xF2}, PayloadLen: 32, StrLen: 52,
}

// --- Tezos addresses: 36 chars → 21 bytes [1 tag + 20 hash] ---

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
