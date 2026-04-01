package dict

import "unsafe"

// KeyType identifies the encoding used for a key.
type KeyType byte

const (
	KeyRaw    KeyType = 0x00
	maxKeyType        = 1 // bump when adding types
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
