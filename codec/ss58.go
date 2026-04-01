package codec

import "fmt"

// SS58 codec for Substrate/Polkadot addresses.
//
// Decoded form: [network_prefix:1-2][account_key:32][checksum:2]
// We store the full decoded bytes including checksum (35-36 bytes)
// to avoid needing blake2b for checksum recomputation on decode.
type ss58Codec struct{}

func (ss58Codec) Encode(s string) ([]byte, error) {
	raw, err := b58decode(s)
	if err != nil {
		return nil, fmt.Errorf("ss58: %w", err)
	}
	// 1-byte prefix: total 35 (1+32+2)
	// 2-byte prefix: total 36 (2+32+2)
	if len(raw) != 35 && len(raw) != 36 {
		return nil, fmt.Errorf("ss58: invalid decoded length %d (want 35 or 36)", len(raw))
	}
	return raw, nil
}

func (ss58Codec) Decode(b []byte) (string, error) {
	if len(b) != 35 && len(b) != 36 {
		return "", fmt.Errorf("ss58: invalid binary length %d (want 35 or 36)", len(b))
	}
	return b58encode(b), nil
}
