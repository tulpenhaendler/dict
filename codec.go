package dict

// KeyType identifies the encoding used for a key.
type KeyType byte

const (
	KeyRaw KeyType = 0x00
)

// Codec encodes and decodes keys for a given KeyType.
type Codec interface {
	Encode(s string) ([]byte, error)
	Decode(b []byte) (string, error)
}

var codecs = map[KeyType]Codec{
	KeyRaw: rawCodec{},
}

func getCodec(t KeyType) Codec {
	return codecs[t]
}

// rawCodec stores strings as plain UTF-8 bytes.
type rawCodec struct{}

func (rawCodec) Encode(s string) ([]byte, error) { return GetRawStringBytes(s), nil }
func (rawCodec) Decode(b []byte) (string, error)  { return string(b), nil }

// GetRawStringBytes returns the plain byte representation of a string.
func GetRawStringBytes(s string) []byte {
	return []byte(s)
}
