# Chain Lens

**Chain Lens** is a Bitcoin transaction analyzer that combines a CLI tool with an intuitive web visualizer. It turns raw Bitcoin transactions and blocks into precise, human-readable reports with diagrams and plain-English explanations.

Perfect for developers, auditors, educators, and anyone who wants to understand Bitcoin transactions at a deep level without needing to run a full node or connect to external APIs.

## Features

**Dual Interface:**
- **CLI Tool**: Parse Bitcoin transactions and blocks, generate detailed JSON reports with fee analysis, script classification, and more
- **Web Visualizer**: Interactive single-page app that explains transactions visually with diagrams, value flow arrows, and tooltips for non-technical users

**Transaction Analysis:**
- Parses both legacy and SegWit transactions (P2PKH, P2SH, P2WPKH, P2WSH, P2TR, OP_RETURN)
- Calculates transaction fees, fee rates, and weight
- Detects SegWit discount savings
- Identifies RBF signaling and timelock constraints (absolute & relative)
- Extracts and decodes OP_RETURN data with protocol detection (Omni, OpenTimestamps)
- Generates warnings for dust outputs, high fees, and unknown scripts

**Block Mode:**
- Parse raw Bitcoin Core block files (`blk*.dat`) with undo data (`rev*.dat`)
- XOR decoding with customizable keys
- Merkle root validation
- Coinbase transaction analysis with BIP34 block height extraction
- Block-level statistics: tx count, total fees, fee rate distribution, script type summary


---

## Installation

### Prerequisites

- **Go 1.19+** (for CLI analyzer)
- **Node.js 18+** & **npm** (for web visualizer)
- **Bash** shell

### Quick Start

```bash
# Clone the repository
git clone https://github.com/yourusername/chain-lens.git
cd chain-lens

# Install dependencies
./setup.sh
```

The `setup.sh` script will:
1. Build the Go analyzer
2. Install React/Vite dependencies for the web app

---

## Usage

### CLI: Analyze a Single Transaction

```bash
./cli.sh fixtures/transactions/tx_legacy_p2pkh.json
```

**Output:**
- Prints JSON report to stdout
- Writes detailed report to `out/<txid>.json`

**Example output:**
```json
{
  "ok": true,
  "txid": "abc123...",
  "fee_sats": 3456,
  "fee_rate_sat_vb": 24.51,
  "size_bytes": 222,
  "weight": 561,
  "vbytes": 141,
  "total_input_sats": 123456,
  "total_output_sats": 120000,
  "segwit": true,
  "vin": [...],
  "vout": [...],
  "warnings": [...]
}
```

### CLI: Analyze a Block

```bash
./cli.sh --block fixtures/blocks/blk04330.dat fixtures/blocks/rev04330.dat fixtures/blocks/xor.dat
```

**Output:**
- Writes `out/<block_hash>.json` with full block analysis
- Includes transaction-by-transaction breakdown
- Block statistics (tx count, total fees, fee rate distribution, script types)

### Web Visualizer

Start the web app with:

```bash
./web.sh
```

The web visualizer will be available at: **http://127.0.0.1:3000**

**Features:**
- **Transaction Input**: Paste JSON fixture or raw transaction hex
- **Visual Diagram**: Input/output flow with value arrows and fee highlighting
- **Story View**: Explain transactions in plain English
- **Tooltips**: Definitions for technical terms (SegWit, OP_RETURN, etc.)
- **Technical Details**: Toggle to show raw hex and script fields
- **Block Mode**: Upload block files for full block analysis

#### API Endpoints

The backend exposes two REST endpoints:

**Health Check:**
```bash
GET /api/health
```
Returns: `200 OK` with `{ "ok": true }`

**Analyze Transaction/Block:**
```bash
POST /api/analyze
Content-Type: application/json

{
  "mode": "transaction",
  "network": "mainnet",
  "raw_tx": "0200000001...",
  "prevouts": [
    {
      "txid": "...",
      "vout": 0,
      "value_sats": 123456,
      "script_pubkey_hex": "0014..."
    }
  ]
}
```

Returns: `200 OK` with detailed transaction analysis JSON

---

## Examples

### Example 1: Analyze a Legacy Transaction

```bash
./cli.sh fixtures/transactions/tx_legacy_p2pkh.json
```

This analyzes a P2PKH transaction and outputs:
- Transaction ID (txid) and witness transaction ID (wtxid)
- Inputs/outputs with addresses
- Fee analysis and SegWit discount (if applicable)
- Script types and classifications
- Any warnings (RBF, dust, high fees, etc.)

### Example 2: Analyze a SegWit Block

```bash
./cli.sh --block fixtures/blocks/blk05051.dat fixtures/blocks/rev05051.dat fixtures/blocks/xor.dat
```

Generates a block report including:
- Block hash and merkle root validation
- List of all transactions in the block
- Coinbase details with BIP34 height
- Block statistics (fees, weight, script type distribution)

### Example 3: Web UI Walkthrough

1. Start the server: `./web.sh`
2. Open http://127.0.0.1:3000
3. Paste a transaction fixture JSON or raw hex
4. View the visual diagram, story explanation, and technical details
5. Toggle "Show Technical Details" to see raw hex and script opcodes

---

## Input Formats

### Transaction Fixture (JSON)

```json
{
  "network": "mainnet",
  "raw_tx": "0200000001...",
  "prevouts": [
    {
      "txid": "11...aa",
      "vout": 0,
      "value_sats": 123456,
      "script_pubkey_hex": "0014..."
    }
  ]
}
```

- `raw_tx`: hex-encoded transaction bytes
- `prevouts`: array of spent outputs (needed to calculate fees)
  - Must include all inputs
  - Order does not matter; matched by `(txid, vout)` pair

### Block Files

Required for block mode analysis:
- `blk*.dat`: Bitcoin Core block data file (may contain multiple blocks)
- `rev*.dat`: Corresponding undo data file with prevout information
- `xor.dat`: XOR key for deobfuscating block and undo data

---

## Output Format

### Transaction Report

The CLI generates a comprehensive JSON report for each transaction with fields like:

```json
{
  "ok": true,
  "txid": "abc123...",
  "wtxid": "def456...",
  "version": 2,
  "locktime": 800000,
  "size_bytes": 222,
  "weight": 561,
  "vbytes": 141,
  "total_input_sats": 123456,
  "total_output_sats": 120000,
  "fee_sats": 3456,
  "fee_rate_sat_vb": 24.51,
  "rbf_signaling": true,
  "segwit": true,
  "vin": [
    {
      "txid": "...",
      "vout": 0,
      "sequence": 4294967295,
      "script_type": "p2wpkh",
      "address": "bc1...",
      "value_sats": 123456,
      "relative_timelock": { "enabled": false }
    }
  ],
  "vout": [
    {
      "n": 0,
      "value_sats": 120000,
      "script_type": "p2wpkh",
      "address": "bc1..."
    }
  ],
  "warnings": [
    { "code": "RBF_SIGNALING" }
  ],
  "segwit_savings": {
    "witness_bytes": 107,
    "non_witness_bytes": 115,
    "weight_actual": 561,
    "weight_if_legacy": 888,
    "savings_pct": 36.82
  }
}
```

### Block Report

For block mode, generates a JSON report with:

```json
{
  "ok": true,
  "mode": "block",
  "block_header": {
    "block_hash": "000000...",
    "merkle_root_valid": true,
    "timestamp": 1710000000,
    "tx_count": 150
  },
  "coinbase": {
    "bip34_height": 800000,
    "total_output_sats": 631250000
  },
  "transactions": [/* full tx analysis array */],
  "block_stats": {
    "total_fees_sats": 6250000,
    "avg_fee_rate_sat_vb": 25.1,
    "script_type_summary": {
      "p2wpkh": 420,
      "p2tr": 180,
      "p2sh": 55,
      /* ... */
    }
  }
}
```

### Error Response

On any error:

```json
{
  "ok": false,
  "error": {
    "code": "INVALID_TX",
    "message": "Detailed error description"
  }
}
```

---

## Supported Script Types

### Output Scripts (Receive)

- **P2PKH** (Pay to Public Key Hash) — Traditional addresses starting with `1`
- **P2SH** (Pay to Script Hash) — Addresses starting with `3`
- **P2WPKH** (Pay to Witness PubKey Hash) — Native SegWit addresses starting with `bc1q`
- **P2WSH** (Pay to Witness Script Hash) — SegWit multi-sig addresses
- **P2TR** (Pay to Taproot) — Modern Taproot addresses starting with `bc1p`
- **OP_RETURN** — Metadata/note outputs (non-spendable)

### Input Scripts (Spend)

- **P2PKH spend** — Traditional legacy signature
- **P2SH spend** — Nested script execution
- **P2SH-P2WPKH** — Nested SegWit (legacy wrapped)
- **P2SH-P2WSH** — Nested SegWit multi-sig
- **P2WPKH spend** — Native SegWit signature
- **P2WSH spend** — SegWit script witness
- **P2TR keypath** — Taproot key path spend
- **P2TR scriptpath** — Taproot script path spend

---

## Architecture

### Analyzer (Go)

```
analyzer/
├── main.go              # CLI entry point & server startup
├── server.go            # REST API endpoints (/api/health, /api/analyze)
├── block_parser.go      # Bitcoin block parsing logic
├── single_transaction_report_gen.go  # Transaction analysis
├── constants.go         # Bitcoin protocol constants
├── utils.go             # Helper functions
└── cli_IO/              # Input/output models and file handling
    ├── models.go
    ├── block_models.go
    ├── file_read_write.go
    └── TError.go
```

**Key responsibilities:**
- Parse raw transaction/block bytes
- Calculate fees, weight, vbytes
- Classify scripts and addresses
- Generate comprehensive JSON reports
- Serve REST API endpoints

### Web Visualizer (React + Vite)

```
analyzer/WebVisualizer/
├── src/
│   ├── App.jsx          # Main app container
│   ├── TransactionGraph.jsx       # Visual diagram component
│   ├── TransactionStory.jsx       # Plain-English explanation
│   ├── TransactionSummary.jsx     # Key metrics display
│   ├── TransactionReportTable.jsx # Detailed table view
│   └── App.css          # Styling
├── index.html           # Entry point
├── vite.config.js       # Vite configuration
└── package.json         # Dependencies
```

**Key responsibilities:**
- Accept transaction JSON or raw hex input
- Call `/api/analyze` endpoint
- Render visual diagrams with value flows
- Display tooltips for technical terms
- Toggle technical details view
- Block file upload handler

---

## Testing

The repository includes test fixtures in `fixtures/` covering various transaction and block types:

**Transaction fixtures:**
- `tx_legacy_p2pkh.json` — Legacy P2PKH transaction
- `tx_legacy_p2sh_p2wsh.json` — P2SH-wrapped P2WSH
- `tx_segwit_p2wpkh_p2tr.json` — SegWit P2WPKH and Taproot outputs
- `multi_input_segwit.json` — Multi-input SegWit transaction
- `prevouts_unordered.json` — Test unordered prevout matching
- `segwit_nested_scriptsig_empty_witness_item.json` — Edge case: nested SegWit with empty witness items

**Block fixtures:**
- `blk04330.dat` / `rev04330.dat` — Real mainnet block data
- `blk05051.dat` / `rev05051.dat` — Additional block data
- `xor.dat` — XOR obfuscation key

Run tests:

```bash
./cli.sh fixtures/transactions/tx_legacy_p2pkh.json
./cli.sh --block fixtures/blocks/blk04330.dat fixtures/blocks/rev04330.dat fixtures/blocks/xor.dat
```


