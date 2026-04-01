package codec

import (
	"encoding/binary"
	"fmt"
	"strconv"
)

// numericStringCodec encodes decimal number strings to binary unsigned varints.
// "12345678" (8 bytes) → varint (4 bytes). Supports uint64 range.
type numericStringCodec struct{}

func (numericStringCodec) Encode(s string) ([]byte, error) {
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("numeric: %w", err)
	}
	var buf [binary.MaxVarintLen64]byte
	size := binary.PutUvarint(buf[:], n)
	return buf[:size], nil
}

func (numericStringCodec) Decode(b []byte) (string, error) {
	n, read := binary.Uvarint(b)
	if read <= 0 {
		return "", fmt.Errorf("numeric: invalid varint")
	}
	return strconv.FormatUint(n, 10), nil
}
