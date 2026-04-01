package codec

import (
	"errors"
	"fmt"
	"strings"
)

// Bech32/Bech32m encoding primitives.

const bech32Charset = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"

var bech32CharTable [256]int8

func init() {
	for i := range bech32CharTable {
		bech32CharTable[i] = -1
	}
	for i, c := range bech32Charset {
		bech32CharTable[c] = int8(i)
	}
}

const (
	bech32Variant  = 1
	bech32mVariant = 0x2bc830a3
)

func bech32Polymod(values []int) int {
	gen := [5]int{0x3b6a57b2, 0x26508e6d, 0x1ea119fa, 0x3d4233dd, 0x2a1462b3}
	chk := 1
	for _, v := range values {
		b := chk >> 25
		chk = ((chk & 0x1ffffff) << 5) ^ v
		for i := 0; i < 5; i++ {
			if (b>>uint(i))&1 != 0 {
				chk ^= gen[i]
			}
		}
	}
	return chk
}

func bech32HRPExpand(hrp string) []int {
	ret := make([]int, 0, len(hrp)*2+1)
	for _, c := range []byte(hrp) {
		ret = append(ret, int(c>>5))
	}
	ret = append(ret, 0)
	for _, c := range []byte(hrp) {
		ret = append(ret, int(c&31))
	}
	return ret
}

func bech32CreateChecksum(hrp string, data []int, spec int) []int {
	values := append(bech32HRPExpand(hrp), data...)
	values = append(values, 0, 0, 0, 0, 0, 0)
	polymod := bech32Polymod(values) ^ spec
	ret := make([]int, 6)
	for i := 0; i < 6; i++ {
		ret[i] = (polymod >> uint(5*(5-i))) & 31
	}
	return ret
}

// bech32DecodeFull decodes a bech32/bech32m string, returning the HRP,
// 5-bit data values, and the variant (bech32Variant or bech32mVariant).
func bech32DecodeFull(s string) (string, []int, int, error) {
	lower := strings.ToLower(s)
	pos := strings.LastIndex(lower, "1")
	if pos < 1 || pos+7 > len(lower) || len(lower) > 90 {
		return "", nil, 0, errors.New("bech32: invalid format")
	}
	hrp := lower[:pos]
	dataStr := lower[pos+1:]

	var dp []int
	for _, c := range []byte(dataStr) {
		v := bech32CharTable[c]
		if v < 0 {
			return "", nil, 0, fmt.Errorf("bech32: invalid character %q", c)
		}
		dp = append(dp, int(v))
	}

	values := append(bech32HRPExpand(hrp), dp...)
	chk := bech32Polymod(values)
	var spec int
	switch chk {
	case bech32Variant:
		spec = bech32Variant
	case bech32mVariant:
		spec = bech32mVariant
	default:
		return "", nil, 0, errors.New("bech32: invalid checksum")
	}

	return hrp, dp[:len(dp)-6], spec, nil
}

// bech32EncodeFull encodes an HRP + 5-bit data values into a bech32/bech32m string.
func bech32EncodeFull(hrp string, data []int, spec int) string {
	checksum := bech32CreateChecksum(hrp, data, spec)
	dp := append(data, checksum...)

	var sb strings.Builder
	sb.Grow(len(hrp) + 1 + len(dp))
	sb.WriteString(hrp)
	sb.WriteByte('1')
	for _, d := range dp {
		sb.WriteByte(bech32Charset[d])
	}
	return sb.String()
}

// pack5bit packs 5-bit values into bytes (big-endian bit packing).
func pack5bit(vals []int) []byte {
	n := (len(vals)*5 + 7) / 8
	out := make([]byte, n)
	bits := 0
	buf := 0
	pos := 0
	for _, v := range vals {
		buf = (buf << 5) | (v & 0x1F)
		bits += 5
		for bits >= 8 {
			bits -= 8
			out[pos] = byte(buf >> bits)
			pos++
		}
	}
	if bits > 0 {
		out[pos] = byte(buf << (8 - bits))
		pos++
	}
	return out[:pos]
}

// unpack5bit unpacks bytes into count 5-bit values.
func unpack5bit(data []byte, count int) []int {
	vals := make([]int, 0, count)
	bits := 0
	buf := 0
	for _, b := range data {
		buf = (buf << 8) | int(b)
		bits += 8
		for bits >= 5 && len(vals) < count {
			bits -= 5
			vals = append(vals, (buf>>bits)&0x1F)
		}
	}
	return vals
}
