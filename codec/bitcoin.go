package codec

import "fmt"

// Bitcoin legacy address codec (base58check).
// 1... (P2PKH, version 0x00) and 3... (P2SH, version 0x05).
// Decoded: [version:1][hash:20] = 21 bytes.
type bitcoinAddressCodec struct{}

func (bitcoinAddressCodec) Encode(s string) ([]byte, error) {
	raw, err := b58checkDecode(s)
	if err != nil {
		return nil, fmt.Errorf("bitcoin: %w", err)
	}
	if len(raw) != 21 {
		return nil, fmt.Errorf("bitcoin: invalid decoded length %d (want 21)", len(raw))
	}
	return raw, nil
}

func (bitcoinAddressCodec) Decode(b []byte) (string, error) {
	if len(b) != 21 {
		return "", fmt.Errorf("bitcoin: invalid binary length %d (want 21)", len(b))
	}
	return b58checkEncode(b), nil
}
