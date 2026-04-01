package codec

import (
	"encoding/hex"
	"fmt"
	"strings"
)

// evmAddressCodec: "0x" + 40 hex chars → 20 bytes
type evmAddressCodec struct{}

func (evmAddressCodec) Encode(s string) ([]byte, error) {
	s = strings.TrimPrefix(s, "0x")
	s = strings.TrimPrefix(s, "0X")
	if len(s) != 40 {
		return nil, fmt.Errorf("evm: invalid address length %d (want 40 hex chars)", len(s))
	}
	return hex.DecodeString(strings.ToLower(s))
}

func (evmAddressCodec) Decode(b []byte) (string, error) {
	if len(b) != 20 {
		return "", fmt.Errorf("evm: invalid address binary length %d (want 20)", len(b))
	}
	return "0x" + hex.EncodeToString(b), nil
}

// evmHash32Codec: "0x" + 64 hex chars → 32 bytes
// Used for tx hashes, block hashes, storage slots, event topics, etc.
type evmHash32Codec struct{}

func (evmHash32Codec) Encode(s string) ([]byte, error) {
	s = strings.TrimPrefix(s, "0x")
	s = strings.TrimPrefix(s, "0X")
	if len(s) != 64 {
		return nil, fmt.Errorf("evm: invalid hash length %d (want 64 hex chars)", len(s))
	}
	return hex.DecodeString(s)
}

func (evmHash32Codec) Decode(b []byte) (string, error) {
	if len(b) != 32 {
		return "", fmt.Errorf("evm: invalid hash binary length %d (want 32)", len(b))
	}
	return "0x" + hex.EncodeToString(b), nil
}

// evmSelectorCodec: "0x" + 8 hex chars → 4 bytes
type evmSelectorCodec struct{}

func (evmSelectorCodec) Encode(s string) ([]byte, error) {
	s = strings.TrimPrefix(s, "0x")
	s = strings.TrimPrefix(s, "0X")
	if len(s) != 8 {
		return nil, fmt.Errorf("evm: invalid selector length %d (want 8 hex chars)", len(s))
	}
	return hex.DecodeString(s)
}

func (evmSelectorCodec) Decode(b []byte) (string, error) {
	if len(b) != 4 {
		return "", fmt.Errorf("evm: invalid selector binary length %d (want 4)", len(b))
	}
	return "0x" + hex.EncodeToString(b), nil
}
