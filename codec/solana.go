package codec

import "fmt"

// Solana addresses (pubkeys) are base58-encoded 32-byte keys (no checksum).
// Solana signatures are base58-encoded 64-byte values (no checksum).

type solanaAddressCodec struct{}

func (solanaAddressCodec) Encode(s string) ([]byte, error) {
	raw, err := b58decode(s)
	if err != nil {
		return nil, fmt.Errorf("solana: %w", err)
	}
	if len(raw) != 32 {
		return nil, fmt.Errorf("solana: invalid address length %d (want 32)", len(raw))
	}
	return raw, nil
}

func (solanaAddressCodec) Decode(b []byte) (string, error) {
	if len(b) != 32 {
		return "", fmt.Errorf("solana: invalid address binary length %d (want 32)", len(b))
	}
	return b58encode(b), nil
}

type solanaSigCodec struct{}

func (solanaSigCodec) Encode(s string) ([]byte, error) {
	raw, err := b58decode(s)
	if err != nil {
		return nil, fmt.Errorf("solana: %w", err)
	}
	if len(raw) != 64 {
		return nil, fmt.Errorf("solana: invalid signature length %d (want 64)", len(raw))
	}
	return raw, nil
}

func (solanaSigCodec) Decode(b []byte) (string, error) {
	if len(b) != 64 {
		return "", fmt.Errorf("solana: invalid signature binary length %d (want 64)", len(b))
	}
	return b58encode(b), nil
}
