package dict

import "unsafe"

// KeyType identifies the encoding used for a key.
type KeyType byte

const (
	KeyRaw               KeyType = 0x00
	KeyTezosAddress      KeyType = 0x01 // tz1/tz2/tz3/tz4/KT1/sr1 → 21 bytes
	KeyTezosBlockHash    KeyType = 0x02 // B → 32 bytes
	KeyTezosSignature    KeyType = 0x03 // sig → 64 bytes
	KeyTezosOpHash       KeyType = 0x04 // o → 32 bytes
	KeyTezosProtocolHash KeyType = 0x05 // P → 32 bytes
	KeyTezosChainID      KeyType = 0x06 // Net → 4 bytes
	KeyTezosExprHash     KeyType = 0x07 // expr → 32 bytes
	KeyTezosContextHash  KeyType = 0x08 // Co → 32 bytes
	KeyTezosPayloadHash  KeyType = 0x09 // vh → 32 bytes
	KeyTezosPubkey       KeyType = 0x0A // edpk/sppk/p2pk/BLpk → 1+33 bytes
	maxKeyType                   = 0x0B // bump when adding types
)

// Codec encodes and decodes keys for a given KeyType.
type Codec interface {
	Encode(s string) ([]byte, error)
	Decode(b []byte) (string, error)
}

// Array-indexed codec table — avoids map lookup on hot path.
var codecs [maxKeyType]Codec

func init() {
	codecs[KeyRaw] = rawCodec{}
	codecs[KeyTezosAddress] = tezosAddressCodec{}
	codecs[KeyTezosBlockHash] = tezosBlockHashCodec
	codecs[KeyTezosSignature] = tezosSignatureCodec
	codecs[KeyTezosOpHash] = tezosOpHashCodec
	codecs[KeyTezosProtocolHash] = tezosProtocolHashCodec
	codecs[KeyTezosChainID] = tezosChainIDCodec
	codecs[KeyTezosExprHash] = tezosExprHashCodec
	codecs[KeyTezosContextHash] = tezosContextHashCodec
	codecs[KeyTezosPayloadHash] = tezosPayloadHashCodec
	codecs[KeyTezosPubkey] = tezosPubkeyCodec{}
}

func getCodec(t KeyType) Codec {
	if int(t) < len(codecs) {
		return codecs[t]
	}
	return nil
}

// rawCodec stores strings as plain UTF-8 bytes.
type rawCodec struct{}

func (rawCodec) Encode(s string) ([]byte, error) { return GetRawStringBytes(s), nil }
func (rawCodec) Decode(b []byte) (string, error)  { return string(b), nil }

// GetRawStringBytes returns the string's underlying bytes without copying.
func GetRawStringBytes(s string) []byte {
	if len(s) == 0 {
		return nil
	}
	return unsafe.Slice(unsafe.StringData(s), len(s))
}
