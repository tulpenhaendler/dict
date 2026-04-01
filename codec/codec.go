package codec

import "unsafe"

// KeyType identifies the encoding used for a key.
type KeyType byte

const (
	// Generic
	KeyRaw    KeyType = 0x00
	KeyHex    KeyType = 0x01 // 0x-prefixed hex → raw bytes
	KeyBase64 KeyType = 0x02 // standard base64 → raw bytes
	KeyBase58 KeyType = 0x03 // base58 (no checksum) → raw bytes

	// Tezos (0x10-0x1F)
	KeyTezosAddress      KeyType = 0x10
	KeyTezosBlockHash    KeyType = 0x11
	KeyTezosSignature    KeyType = 0x12
	KeyTezosOpHash       KeyType = 0x13
	KeyTezosProtocolHash KeyType = 0x14
	KeyTezosChainID      KeyType = 0x15
	KeyTezosExprHash     KeyType = 0x16
	KeyTezosContextHash  KeyType = 0x17
	KeyTezosPayloadHash  KeyType = 0x18
	KeyTezosPubkey       KeyType = 0x19

	// EVM (0x20-0x2F)
	KeyEVMAddress  KeyType = 0x20 // 0x + 40 hex → 20 bytes
	KeyEVMHash32   KeyType = 0x21 // 0x + 64 hex → 32 bytes (tx/block/topic/slot)
	KeyEVMSelector KeyType = 0x22 // 0x + 8 hex → 4 bytes

	// IPFS (0x30-0x3F)
	KeyIPFSCID KeyType = 0x30 // auto-detect Qm.../bafy.../bafk.../bafyr... → tag + digest

	// Multi-chain (0x40-0x4F)
	KeyBech32          KeyType = 0x40 // any bech32/bech32m (Bitcoin segwit, Cosmos, Cardano)
	KeySolanaAddress   KeyType = 0x41 // base58, 32 bytes
	KeySolanaSig       KeyType = 0x42 // base58, 64 bytes
	KeySS58            KeyType = 0x43 // Substrate/Polkadot base58
	KeyBitcoinAddress  KeyType = 0x44 // legacy base58check (1.../3...)
	KeyNumericString   KeyType = 0x45 // decimal string → varint

	MaxKeyType = 0x50 // bump when adding types
)

// Codec encodes and decodes keys for a given KeyType.
type Codec interface {
	Encode(s string) ([]byte, error)
	Decode(b []byte) (string, error)
}

// Codecs is the array-indexed codec table.
var Codecs [MaxKeyType]Codec

func init() {
	// Generic
	Codecs[KeyRaw] = rawCodec{}
	Codecs[KeyHex] = hexCodec{}
	Codecs[KeyBase64] = base64Codec{}
	Codecs[KeyBase58] = base58Codec{}

	// Tezos
	Codecs[KeyTezosAddress] = tezosAddressCodec{}
	Codecs[KeyTezosBlockHash] = TezosBlockHashCodec
	Codecs[KeyTezosSignature] = TezosSignatureCodec
	Codecs[KeyTezosOpHash] = TezosOpHashCodec
	Codecs[KeyTezosProtocolHash] = TezosProtocolHashCodec
	Codecs[KeyTezosChainID] = TezosChainIDCodec
	Codecs[KeyTezosExprHash] = TezosExprHashCodec
	Codecs[KeyTezosContextHash] = TezosContextHashCodec
	Codecs[KeyTezosPayloadHash] = TezosPayloadHashCodec
	Codecs[KeyTezosPubkey] = tezosPubkeyCodec{}

	// EVM
	Codecs[KeyEVMAddress] = evmAddressCodec{}
	Codecs[KeyEVMHash32] = evmHash32Codec{}
	Codecs[KeyEVMSelector] = evmSelectorCodec{}

	// IPFS
	Codecs[KeyIPFSCID] = ipfsCIDCodec{}

	// Multi-chain
	Codecs[KeyBech32] = bech32Codec{}
	Codecs[KeySolanaAddress] = solanaAddressCodec{}
	Codecs[KeySolanaSig] = solanaSigCodec{}
	Codecs[KeySS58] = ss58Codec{}
	Codecs[KeyBitcoinAddress] = bitcoinAddressCodec{}
	Codecs[KeyNumericString] = numericStringCodec{}
}

// Get returns the codec for the given key type, or nil.
func Get(t KeyType) Codec {
	if int(t) < len(Codecs) {
		return Codecs[t]
	}
	return nil
}

// rawCodec stores strings as plain UTF-8 bytes.
type rawCodec struct{}

func (rawCodec) Encode(s string) ([]byte, error) { return GetRawStringBytes(s), nil }
func (rawCodec) Decode(b []byte) (string, error)  { return string(b), nil }

// GetRawStringBytes returns the string's underlying bytes without copying.
func GetRawStringBytes(s string) []byte {
	if len(s) == 0 {
		return nil
	}
	return unsafe.Slice(unsafe.StringData(s), len(s))
}
