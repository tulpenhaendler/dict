package codec

import "fmt"

// bech32Codec encodes any bech32/bech32m string to compact binary.
//
// Compact format: [flags:1][hrp_len:1][hrp:N][5bit_count:1][packed_5bit:M]
//   flags bit 0: 0=bech32, 1=bech32m
//
// Covers: Bitcoin segwit (bc1q.../bc1p...), Cosmos (cosmos1.../osmo1...),
// Cardano (addr1...), and any other bech32-encoded address.
type bech32Codec struct{}

func (bech32Codec) Encode(s string) ([]byte, error) {
	hrp, data5, spec, err := bech32DecodeFull(s)
	if err != nil {
		return nil, fmt.Errorf("bech32: %w", err)
	}
	if len(hrp) > 255 || len(data5) > 255 {
		return nil, fmt.Errorf("bech32: HRP or data too long")
	}

	packed := pack5bit(data5)

	var flags byte
	if spec == bech32mVariant {
		flags = 0x01
	}

	// [flags][hrp_len][hrp...][5bit_count][packed...]
	out := make([]byte, 1+1+len(hrp)+1+len(packed))
	out[0] = flags
	out[1] = byte(len(hrp))
	copy(out[2:], hrp)
	out[2+len(hrp)] = byte(len(data5))
	copy(out[3+len(hrp):], packed)
	return out, nil
}

func (bech32Codec) Decode(b []byte) (string, error) {
	if len(b) < 4 {
		return "", fmt.Errorf("bech32: binary too short")
	}
	flags := b[0]
	hrpLen := int(b[1])
	if 2+hrpLen+1 > len(b) {
		return "", fmt.Errorf("bech32: truncated")
	}
	hrp := string(b[2 : 2+hrpLen])
	count := int(b[2+hrpLen])
	packed := b[3+hrpLen:]

	data5 := unpack5bit(packed, count)

	spec := bech32Variant
	if flags&0x01 != 0 {
		spec = bech32mVariant
	}

	return bech32EncodeFull(hrp, data5, spec), nil
}
