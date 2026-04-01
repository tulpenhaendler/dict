package codec

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
)

// hexCodec encodes 0x-prefixed hex strings to raw bytes.
// "0xDEAD" → [0xDE, 0xAD] (2 bytes)
type hexCodec struct{}

func (hexCodec) Encode(s string) ([]byte, error) {
	s = strings.TrimPrefix(s, "0x")
	if len(s)%2 != 0 {
		return nil, fmt.Errorf("hex: odd length %d", len(s))
	}
	return hex.DecodeString(s)
}

func (hexCodec) Decode(b []byte) (string, error) {
	return "0x" + hex.EncodeToString(b), nil
}

// base64Codec encodes standard base64 strings to raw bytes.
type base64Codec struct{}

func (base64Codec) Encode(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}

func (base64Codec) Decode(b []byte) (string, error) {
	return base64.StdEncoding.EncodeToString(b), nil
}

// base58Codec encodes base58 strings (no checksum) to raw bytes.
type base58Codec struct{}

func (base58Codec) Encode(s string) ([]byte, error) {
	return b58decode(s)
}

func (base58Codec) Decode(b []byte) (string, error) {
	return b58encode(b), nil
}
