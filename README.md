# dict

A persistent dictionary that maps typed strings to sequential `uint64` IDs. Designed for multi-chain blockchain indexers where you need to assign compact integer IDs to addresses, hashes, and other identifiers across many chains.

Keys are stored in a compact binary format based on their type — a Tezos address takes 21 bytes instead of 36, an Ethereum address takes 20 bytes instead of 42, etc.

```go
go get github.com/tulpenhaendler/dict
```

## Usage

```go
d, err := dict.Open("path/to/mydict")
defer d.Close()

// Upsert — returns existing ID or inserts and returns new one
id, err := d.Get("tz1cSj7fTex3JPd1p1LN1fwek6AV1kH93Wwc", dict.KeyTezosAddress)

// Check existence without inserting
ok, err := d.Exists("tz1cSj7fTex3JPd1p1LN1fwek6AV1kH93Wwc", dict.KeyTezosAddress)

// Reverse lookup — ID back to string
s, keyType, err := d.Reverse(id)

// Batch upsert
ids, err := d.BatchGet([]dict.BatchEntry{
    {Key: "0x742d35Cc6634C0532925a3b844Bc9e7595f2bD18", KeyType: dict.KeyEVMAddress},
    {Key: "bc1qw508d6qejxtdg4y5r3zarvary0c5xw7kv8f3t4", KeyType: dict.KeyBech32},
    {Key: "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA", KeyType: dict.KeySolanaAddress},
})

// Housekeeping
d.Len()   // number of entries
d.Sync()  // flush to disk
```

## API

```go
func Open(basePath string) (*Dict, error)
func OpenWithCacheSize(basePath string, cacheSize int) (*Dict, error)

func (d *Dict) Get(s string, keyType KeyType) (uint64, error)
func (d *Dict) Exists(s string, keyType KeyType) (bool, error)
func (d *Dict) Reverse(id uint64) (string, KeyType, error)
func (d *Dict) BatchGet(entries []BatchEntry) ([]uint64, error)
func (d *Dict) Len() uint64
func (d *Dict) Sync() error
func (d *Dict) Close() error
```

`Open` creates three files: `<path>.dat`, `<path>.idx`, `<path>.rev`. An LRU cache (200k entries by default) sits in front of disk lookups.

## Key Types

### Generic

| Type | Input | Binary | Savings |
|---|---|---|---|
| `KeyRaw` | any string | N bytes | — |
| `KeyHex` | `0x`-prefixed hex | N/2 bytes | 50%+ |
| `KeyBase64` | base64 string | ~75% of input | ~25% |
| `KeyBase58` | base58 string (no checksum) | ~73% of input | ~27% |
| `KeyNumericString` | decimal number string | varint (1-10 bytes) | 50-90% |

### Tezos

| Type | Prefix | Binary | Savings |
|---|---|---|---|
| `KeyTezosAddress` | tz1/tz2/tz3/tz4/KT1/sr1 | 21 bytes | 42% |
| `KeyTezosBlockHash` | B | 32 bytes | 37% |
| `KeyTezosOpHash` | o | 32 bytes | 37% |
| `KeyTezosSignature` | sig | 64 bytes | 33% |
| `KeyTezosProtocolHash` | P | 32 bytes | 37% |
| `KeyTezosChainID` | Net | 4 bytes | 73% |
| `KeyTezosExprHash` | expr | 32 bytes | 41% |
| `KeyTezosContextHash` | Co | 32 bytes | 38% |
| `KeyTezosPayloadHash` | vh | 32 bytes | 38% |
| `KeyTezosPubkey` | edpk/sppk/p2pk/BLpk | 33-49 bytes | 28-39% |

### EVM (Ethereum)

| Type | Input | Binary | Savings |
|---|---|---|---|
| `KeyEVMAddress` | `0x` + 40 hex (42 chars) | 20 bytes | 52% |
| `KeyEVMHash32` | `0x` + 64 hex (66 chars) | 32 bytes | 52% |
| `KeyEVMSelector` | `0x` + 8 hex (10 chars) | 4 bytes | 60% |

`KeyEVMHash32` works for transaction hashes, block hashes, storage slots, event topics, and any other 32-byte hex value.

### IPFS

| Type | Input | Binary | Savings |
|---|---|---|---|
| `KeyIPFSCID` | `Qm...` (CIDv0) | 33 bytes | 28% |
| `KeyIPFSCID` | `bafy.../bafk.../bafyr...` (CIDv1) | 33 bytes | 44% |

Auto-detects CIDv0 vs CIDv1, and the multicodec (dag-pb, raw, dag-cbor).

### Multi-chain

| Type | Chains | Binary | Savings |
|---|---|---|---|
| `KeyBech32` | Bitcoin segwit, Cosmos, Cardano, Lightning | varies | 38-50% |
| `KeySolanaAddress` | Solana pubkeys/programs | 32 bytes | 27% |
| `KeySolanaSig` | Solana transaction signatures | 64 bytes | 27% |
| `KeySS58` | Polkadot, Kusama, Substrate chains | 35-36 bytes | 27% |
| `KeyBitcoinAddress` | Bitcoin legacy (1.../3...) | 21 bytes | 38% |
| `KeyNumericString` | Block numbers, nonces, amounts | 1-10 bytes | 50-90% |

`KeyBech32` is a single codec that handles any bech32/bech32m address: Bitcoin segwit (`bc1q...`, `bc1p...`), all Cosmos chains (`cosmos1...`, `osmo1...`, `juno1...`), Cardano (`addr1...`), and more.

## Storage Format

Three files per dictionary:

- **`.dat`** — append-only data log. Entries: `[type:1][len:1][encoded_key:N]`.
- **`.idx`** — mmap'd hash index with split metadata (Swiss table inspired). Ctrl bytes for fast probing, separate slot array with 16-byte slots (id:8 + offset:8).
- **`.rev`** — mmap'd fixed-width array for reverse lookups. `rev[id]` = offset into `.dat`.

The `.dat` file is the source of truth. The `.idx` and `.rev` are derived and rebuilt automatically if corrupted or missing.

## Performance

On an AMD Ryzen 9 9950X3D with 100k entries:

| Operation | Time | Allocations |
|---|---|---|
| Get (LRU hit) | ~87 ns | 0 |
| Get (disk, cache miss) | ~1 μs | 1 |
| Tezos Get (LRU hit) | ~32 ns | 0 |
| Batch Get (1000 keys) | ~37 ns/key | — |

## Project Structure

```
dict/
├── dict.go              # Public API
├── dict_test.go         # Integration tests + benchmarks
├── codec/
│   ├── codec.go         # KeyType, Codec interface, registry
│   ├── base58.go        # Base58/base58check encoding
│   ├── bech32_encoding.go # Bech32/bech32m encoding
│   ├── tezos.go         # Tezos codecs
│   ├── evm.go           # EVM codecs
│   ├── ipfs.go          # IPFS CID codec
│   ├── solana.go        # Solana codecs
│   ├── ss58.go          # Substrate/Polkadot codec
│   ├── bitcoin.go       # Bitcoin legacy codec
│   ├── numeric.go       # Numeric string codec
│   └── encoding.go      # Generic hex/base64/base58 codecs
└── internal/
    ├── data.go          # Append-only data log + hash functions
    ├── index.go         # Mmap'd split-metadata hash index
    └── reverse.go       # Mmap'd reverse lookup array
```

## Adding Custom Key Types

Implement the `codec.Codec` interface and register it:

```go
import "github.com/tulpenhaendler/dict/codec"

type myCodec struct{}

func (myCodec) Encode(s string) ([]byte, error) {
    // Convert string to compact binary (max 255 bytes)
    return []byte(s), nil
}

func (myCodec) Decode(b []byte) (string, error) {
    // Convert compact binary back to original string
    return string(b), nil
}

func init() {
    codec.Codecs[codec.MaxKeyType] = myCodec{} // use a free slot
}
```
