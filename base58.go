package dict

import (
	"crypto/sha256"
	"errors"
	"fmt"
)

// Base58check encode/decode without math/big.
// Uses byte-level arithmetic on fixed-size buffers.

const b58alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

var b58table [256]int8

func init() {
	for i := range b58table {
		b58table[i] = -1
	}
	for i, c := range b58alphabet {
		b58table[c] = int8(i)
	}
}

func b58checkEncode(payload []byte) string {
	h := doubleSHA256(payload)
	// payload + 4 byte checksum on stack
	buf := make([]byte, len(payload)+4)
	copy(buf, payload)
	buf[len(payload)] = h[0]
	buf[len(payload)+1] = h[1]
	buf[len(payload)+2] = h[2]
	buf[len(payload)+3] = h[3]
	return b58encode(buf)
}

func b58checkDecode(s string) ([]byte, error) {
	data, err := b58decode(s)
	if err != nil {
		return nil, err
	}
	if len(data) < 5 {
		return nil, errors.New("base58check: too short")
	}
	split := len(data) - 4
	expected := doubleSHA256(data[:split])
	if data[split] != expected[0] || data[split+1] != expected[1] ||
		data[split+2] != expected[2] || data[split+3] != expected[3] {
		return nil, errors.New("base58check: invalid checksum")
	}
	return data[:split], nil
}

// b58encode encodes bytes to base58 without math/big.
// Works by repeatedly dividing the byte array by 58.
func b58encode(data []byte) string {
	// count leading zeros
	var nzeros int
	for _, b := range data {
		if b != 0 {
			break
		}
		nzeros++
	}

	// worst case: log(256)/log(58) ≈ 1.366, so ceil(len*138/100) + 1
	size := len(data)*138/100 + 1
	buf := make([]byte, size)
	var length int

	for _, b := range data {
		carry := int(b)
		for j := 0; j < length; j++ {
			carry += int(buf[j]) << 8
			buf[j] = byte(carry % 58)
			carry /= 58
		}
		for carry > 0 {
			buf[length] = byte(carry % 58)
			length++
			carry /= 58
		}
	}

	// result = leading '1's + reversed alphabet-mapped buf
	result := make([]byte, nzeros+length)
	for i := 0; i < nzeros; i++ {
		result[i] = '1'
	}
	for i := 0; i < length; i++ {
		result[nzeros+i] = b58alphabet[buf[length-1-i]]
	}
	return string(result)
}

// b58decode decodes a base58 string without math/big.
func b58decode(s string) ([]byte, error) {
	// count leading '1's (leading zeros in output)
	var nzeros int
	for _, c := range []byte(s) {
		if c != '1' {
			break
		}
		nzeros++
	}

	// worst case output size
	size := len(s)*733/1000 + 1
	buf := make([]byte, size)
	var length int

	for _, c := range []byte(s) {
		v := b58table[c]
		if v < 0 {
			return nil, fmt.Errorf("base58: invalid character %q", c)
		}
		carry := int(v)
		for j := 0; j < length; j++ {
			carry += int(buf[j]) * 58
			buf[j] = byte(carry & 0xff)
			carry >>= 8
		}
		for carry > 0 {
			buf[length] = byte(carry & 0xff)
			length++
			carry >>= 8
		}
	}

	// result = leading zeros + reversed buf
	result := make([]byte, nzeros+length)
	// leading zeros are already 0x00
	for i := 0; i < length; i++ {
		result[nzeros+i] = buf[length-1-i]
	}
	return result, nil
}

func doubleSHA256(data []byte) [32]byte {
	first := sha256.Sum256(data)
	return sha256.Sum256(first[:])
}
