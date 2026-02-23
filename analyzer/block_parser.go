package main

import (
	"analyzer/cli_IO"
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

// first transaction in a block is always coinbase
// but just in case you know :>, idk how your hidden block testcases look like
func isCoinbase(tx *wire.MsgTx) bool {
	return len(tx.TxIn) == 1 &&
		tx.TxIn[0].PreviousOutPoint.Hash.IsEqual(&chainhash.Hash{}) &&
		tx.TxIn[0].PreviousOutPoint.Index == 0xFFFFFFFF
}

func BlockHashReversedHex(block *cli_IO.Block) string {
	hashBytes, err := hex.DecodeString(block.BlockHeader.BlockHash)
	if err != nil {
		return "" // or handle error
	}
	for i, j := 0, len(hashBytes)-1; i < j; i, j = i+1, j-1 {
		hashBytes[i], hashBytes[j] = hashBytes[j], hashBytes[i]
	}
	return hex.EncodeToString(hashBytes)
}

func parseCoinbaseHeight(tx *wire.MsgTx) uint32 {
	if len(tx.TxIn) == 0 {
		return 0
	}

	script := tx.TxIn[0].SignatureScript
	if len(script) == 0 {
		return 0
	}

	r := bytes.NewReader(script)

	// Read the first byte as push opcode
	pushSizeByte, err := r.ReadByte()
	if err != nil {
		return 0
	}
	pushSize := int(pushSizeByte)
	if pushSize <= 0 || pushSize > r.Len() {
		return 0
	}

	// Read the height bytes
	heightBytes := make([]byte, pushSize)
	_, err = r.Read(heightBytes)
	if err != nil {
		return 0
	}

	// Convert little-endian bytes to integer
	height := uint32(0)
	for i := 0; i < len(heightBytes); i++ {
		height |= uint32(heightBytes[i]) << (8 * i)
	}

	return height
}

func TxToRawHex(tx *wire.MsgTx) (string, error) {
	buf := new(bytes.Buffer)
	if err := tx.Serialize(buf); err != nil {
		return "", err
	}

	rawBytes := buf.Bytes()
	return hex.EncodeToString(rawBytes), nil
}

func applyXor(data []byte, key []byte) []byte {
	if len(key) == 0 {
		return data
	}

	// Check if key is all zeros
	allZero := true
	for _, b := range key {
		if b != 0x00 {
			allZero = false
			break
		}
	}
	if allZero {
		return data
	}

	for i := range data {
		data[i] ^= key[i%len(key)]
	}

	return data
}

func readCompactSize(r io.Reader) (uint64, error) {
	var fb [1]byte
	_, err := r.Read(fb[:])
	if err != nil {
		return 0, err
	}

	switch fb[0] {
	case 0xfd:
		var val uint16
		if err := binary.Read(r, binary.LittleEndian, &val); err != nil {
			return 0, err
		}
		return uint64(val), nil
	case 0xfe:
		var val uint32
		if err := binary.Read(r, binary.LittleEndian, &val); err != nil {
			return 0, err
		}
		return uint64(val), nil
	case 0xff:
		var val uint64
		if err := binary.Read(r, binary.LittleEndian, &val); err != nil {
			return 0, err
		}
		return val, nil
	default:
		return uint64(fb[0]), nil
	}
}

// peekTxCount reads the transaction count from the beginning of undo data without consuming it
func peekTxCount(data []byte) uint64 {
	if len(data) == 0 {
		return 0
	}
	r := bytes.NewReader(data)
	count, err := readCompactSize(r)
	if err != nil {
		return 0
	}
	return count
}

// DecompressAmount https://github.com/bitcoin/bitcoin/blob/master/src/compressor.cpp
func DecompressAmount(x uint64) int64 {
	if x == 0 {
		return 0
	}
	x--         // subtract 1 as per Bitcoin Core
	e := x % 10 // exponent
	x /= 10
	n := int64(0)

	if e < 9 {
		d := (x % 9) + 1
		x /= 9
		n = int64(x*10 + d)
	} else {
		n = int64(x) + 1
	}

	for i := uint64(0); i < e; i++ {
		n *= 10
	}
	return n
}

// decompressPubKey reconstructs a full 65-byte uncompressed pubkey from compressed format
func decompressPubKey(prefix byte, compKey []byte) ([]byte, error) {
	if len(compKey) != 32 {
		return nil, fmt.Errorf("invalid compressed key length: %d", len(compKey))
	}

	// Build the full compressed key bytes (33 bytes):
	// prefix (0x02 or 0x03) + 32‑byte X coordinate
	compressed := append([]byte{prefix}, compKey...)

	// Parse the compressed public key
	pub, err := btcec.ParsePubKey(compressed)
	if err != nil {
		return nil, fmt.Errorf("failed to parse compressed pubkey: %w", err)
	}

	// Serialize uncompressed (65 bytes): 0x04 || X(32) || Y(32)
	uncompressed := pub.SerializeUncompressed()
	return uncompressed, nil
}

func ReadVarInt(r io.Reader) (uint64, error) {
	var n uint64
	for {
		var b [1]byte
		if _, err := io.ReadFull(r, b[:]); err != nil {
			return 0, err
		}
		// Bitcoin VarInt: Each byte has a "more" bit in MSB.
		// If MSB is set, there's another byte coming.
		n = (n << 7) | uint64(b[0]&0x7F)
		if b[0]&0x80 != 0 {
			n++
		} else {
			return n, nil
		}
	}
}

func DecompressScript(r io.Reader) ([]byte, error) {
	t := make([]byte, 1)
	if _, err := r.Read(t); err != nil {
		return nil, err
	}

	switch t[0] {
	case 0x00: // P2PKH
		hash := make([]byte, 20)
		if _, err := io.ReadFull(r, hash); err != nil {
			return nil, err
		}
		return append([]byte{0x76, 0xa9, 0x14}, append(hash, 0x88, 0xac)...), nil

	case 0x01: // P2SH
		hash := make([]byte, 20)
		if _, err := io.ReadFull(r, hash); err != nil {
			return nil, err
		}
		return append([]byte{0xa9, 0x14}, append(hash, 0x87)...), nil

	case 0x02, 0x03: // P2PK compressed
		pubkey := make([]byte, 33)
		pubkey[0] = t[0]
		if _, err := io.ReadFull(r, pubkey[1:]); err != nil {
			return nil, err
		}
		return append([]byte{0x21}, append(pubkey, 0xac)...), nil

	case 0x04, 0x05: // P2PK compressed with full decompression
		compKey := make([]byte, 32)
		if _, err := io.ReadFull(r, compKey); err != nil {
			return nil, err
		}

		// decompress to 65-byte pubkey
		pubkey, err := decompressPubKey(t[0], compKey)
		if err != nil {
			return nil, err
		}

		script := make([]byte, 67)
		script[0] = 65
		copy(script[1:], pubkey)
		script[66] = 0xac // OP_CHECKSIG
		return script, nil

	default: // raw script
		fullReader := io.MultiReader(bytes.NewReader(t), r)
		size, err := ReadVarInt(fullReader)
		if err != nil {
			return nil, err
		}

		if size < 6 {
			return nil, fmt.Errorf("invalid script size in undo data")
		}

		realLength := size - 6
		script := make([]byte, realLength)
		if _, err := io.ReadFull(r, script); err != nil {
			return nil, err
		}
		return script, nil
	}
}

func DecompressScript_2(r io.Reader) ([]byte, error) {
	// 1. Read the VarInt prefix (this replaces the single byte read)
	size, err := ReadVarInt(r)
	if err != nil {
		return nil, err
	}

	// 2. Handle the compressed types (0-5)
	switch size {
	case 0x00: // P2PKH
		hash := make([]byte, 20)
		if _, err := io.ReadFull(r, hash); err != nil {
			return nil, err
		}
		return append([]byte{0x76, 0xa9, 0x14}, append(hash, 0x88, 0xac)...), nil

	case 0x01: // P2SH
		hash := make([]byte, 20)
		if _, err := io.ReadFull(r, hash); err != nil {
			return nil, err
		}
		return append([]byte{0xa9, 0x14}, append(hash, 0x87)...), nil

	case 0x02, 0x03: // P2PK compressed
		pubkey := make([]byte, 33)
		pubkey[0] = byte(size) // The size IS the prefix here
		if _, err := io.ReadFull(r, pubkey[1:]); err != nil {
			return nil, err
		}
		return append([]byte{0x21}, append(pubkey, 0xac)...), nil

	case 0x04, 0x05: // P2PK uncompressed
		compKey := make([]byte, 32)
		if _, err := io.ReadFull(r, compKey); err != nil {
			return nil, err
		}
		pubkey, err := decompressPubKey(byte(size), compKey)
		if err != nil {
			return nil, err
		}
		script := make([]byte, 67)
		script[0] = 65
		copy(script[1:], pubkey)
		script[66] = 0xac
		return script, nil

	default:
		// 3. Handle raw scripts (size >= 6)
		if size < 6 {
			return nil, fmt.Errorf("invalid script size")
		}

		realLength := size - 6

		script := make([]byte, realLength)
		if _, err := io.ReadFull(r, script); err != nil {
			return nil, err
		}
		return script, nil
	}
}

func readBlkRevFiles(blkPath, revPath, xorPath string, callback func(block *wire.MsgBlock, undo []cli_IO.ValidPrevOut)) {
	// Load XOR key
	xr, _ := os.ReadFile(xorPath)

	// Open both files
	fBlk, err := os.Open(blkPath)
	if err != nil {
		slog.Error("Error opening blk file", "err", err)
		return
	}
	defer fBlk.Close()

	fRev, err := os.Open(revPath)
	if err != nil {
		slog.Error("Error opening rev file", "err", err)
		return
	}
	defer fRev.Close()

	rBlk := bufio.NewReader(fBlk)
	rRev := bufio.NewReader(fRev)

	expectedMagic := []byte{0xf9, 0xbe, 0xb4, 0xd9}

	// Store parsed blocks with their tx counts
	type blockInfo struct {
		block       *wire.MsgBlock
		nonCoinbase int // number of non-coinbase transactions
	}

	// Store parsed undo records with their tx counts
	type undoInfo struct {
		prevouts []cli_IO.ValidPrevOut
		txCount  int
	}

	var blocks []blockInfo
	var undos []undoInfo

	// Read and parse all blocks
	for {
		magicBlk := make([]byte, 4)
		if _, err := io.ReadFull(rBlk, magicBlk); err != nil {
			break
		}

		if !bytes.Equal(magicBlk, expectedMagic) {
			break
		}

		var sizeBlk uint32
		if err := binary.Read(rBlk, binary.LittleEndian, &sizeBlk); err != nil {
			slog.Error("Error reading block size", "err", err)
			break
		}

		rawBlk := make([]byte, sizeBlk)
		if _, err := io.ReadFull(rBlk, rawBlk); err != nil {
			slog.Error("Error reading block data", "err", err)
			break
		}
		rawBlk = applyXor(rawBlk, xr)

		var block wire.MsgBlock
		if err := block.Deserialize(bytes.NewReader(rawBlk)); err != nil {
			slog.Warn("Error deserializing block", "err", err)
			continue
		}

		// Count non-coinbase transactions
		nonCoinbase := len(block.Transactions) - 1
		if nonCoinbase < 0 {
			nonCoinbase = 0
		}

		blocks = append(blocks, blockInfo{block: &block, nonCoinbase: nonCoinbase})
	}

	// Read and parse all undo records
	for {
		magicRev := make([]byte, 4)
		if _, err := io.ReadFull(rRev, magicRev); err != nil {
			break
		}

		if !bytes.Equal(magicRev, expectedMagic) {
			break
		}

		var sizeRev uint32
		if err := binary.Read(rRev, binary.LittleEndian, &sizeRev); err != nil {
			break
		}

		rawRev := make([]byte, sizeRev)
		if _, err := io.ReadFull(rRev, rawRev); err != nil {
			break
		}
		rawRev = applyXor(rawRev, xr)

		// Skip the 32-byte hash after undo data
		hashBytes := make([]byte, 32)
		if _, err := io.ReadFull(rRev, hashBytes); err != nil {
			break
		}

		prevouts, txCount := parseUndoFileWithCount(rawRev)
		undos = append(undos, undoInfo{prevouts: prevouts, txCount: txCount})
	}

	slog.Info("Read blocks and undos", "numBlocks", len(blocks), "numUndos", len(undos))

	// Create a map of undo records by tx count for matching
	// Since multiple undos can have the same tx count, we need to track which ones are used
	undoUsed := make([]bool, len(undos))

	// Process each block and find matching undo record
	for _, bi := range blocks {
		// Find matching undo record by transaction count
		matchIdx := -1
		for j, ui := range undos {
			if !undoUsed[j] && ui.txCount == bi.nonCoinbase {
				matchIdx = j
				break
			}
		}

		if matchIdx == -1 {
			slog.Warn("No matching undo record found", "blockTxCount", bi.nonCoinbase)
			// Still call callback with empty prevouts
			callback(bi.block, nil)
			continue
		}

		undoUsed[matchIdx] = true
		callback(bi.block, undos[matchIdx].prevouts)
	}

	slog.Info("Completed reading blk and rev files")
}

type undoRecord struct {
	txCount    int
	inputCount int
	prevouts   []cli_IO.ValidPrevOut
}

func readAllUndoRecords(revPath string, xr []byte) []undoRecord {
	var records []undoRecord

	fRev, err := os.Open(revPath)
	if err != nil {
		slog.Error("Error opening rev file", "err", err)
		return records
	}
	defer fRev.Close()

	rRev := bufio.NewReader(fRev)
	expectedMagic := []byte{0xf9, 0xbe, 0xb4, 0xd9}

	for {
		// Read magic
		magicRev := make([]byte, 4)
		if _, err := io.ReadFull(rRev, magicRev); err != nil {
			break
		}

		if !bytes.Equal(magicRev, expectedMagic) {
			break
		}

		// Read size
		var sizeRev uint32
		if err := binary.Read(rRev, binary.LittleEndian, &sizeRev); err != nil {
			break
		}

		// Read undo data
		rawRev := make([]byte, sizeRev)
		if _, err := io.ReadFull(rRev, rawRev); err != nil {
			break
		}
		rawRev = applyXor(rawRev, xr)

		// Skip the 32-byte hash
		hashBytes := make([]byte, 32)
		if _, err := io.ReadFull(rRev, hashBytes); err != nil {
			break
		}

		// Parse undo data
		prevouts, txCount := parseUndoFileWithCount(rawRev)

		records = append(records, undoRecord{
			txCount:    txCount,
			inputCount: len(prevouts),
			prevouts:   prevouts,
		})
	}

	return records
}

func parseUndoFileWithCount(blockBytes []byte) ([]cli_IO.ValidPrevOut, int) {
	result := make([]cli_IO.ValidPrevOut, 0)
	undoBlock := bytes.NewReader(blockBytes)

	numTx, err := readCompactSize(undoBlock)
	if err != nil {
		slog.Error("Failed to read numTx", "err", err)
		return result, 0
	}

	for i := uint64(0); i < numTx; i++ {
		numInputs, err := readCompactSize(undoBlock)
		if err != nil {
			slog.Error("Failed to read numInputs", "txIdx", i, "err", err)
			break
		}

		for j := uint64(0); j < numInputs; j++ {
			// 1. Read nCode (height << 1 | coinbase flag)
			nCode, err := ReadVarInt(undoBlock)
			if err != nil {
				slog.Error("Failed to read nCode", "txIdx", i, "inputIdx", j, "err", err)
				return result, int(numTx)
			}

			// 2. Read Version (only when height > 0)
			height := nCode >> 1
			if height > 0 {
				_, err = ReadVarInt(undoBlock)
				if err != nil {
					slog.Error("Failed to read version", "txIdx", i, "inputIdx", j, "err", err)
					return result, int(numTx)
				}
			}

			// 3. Read Compressed Amount
			amtCompact, err := ReadVarInt(undoBlock)
			if err != nil {
				slog.Error("Failed to read amount", "txIdx", i, "inputIdx", j, "err", err)
				return result, int(numTx)
			}
			amount := DecompressAmount(amtCompact)

			// 4. Read Compressed Script
			script, err := DecompressScript_2(undoBlock)
			if err != nil {
				slog.Error("DecompressScript failed", "txIdx", i, "inputIdx", j, "err", err)
				return result, int(numTx)
			}

			prevout := cli_IO.ValidPrevOut{
				ValueSats:       amount,
				ScriptPubkeyHex: hex.EncodeToString(script),
			}
			result = append(result, prevout)
		}
	}

	return result, int(numTx)
}

func parseUndoFile(blockBytes []byte) []cli_IO.ValidPrevOut {
	prevouts, _ := parseUndoFileWithCount(blockBytes)
	return prevouts
}

func createFullPrevOut(block *wire.MsgBlock, undoPrevouts []cli_IO.ValidPrevOut) map[string][]cli_IO.Prevout {
	results := make(map[string][]cli_IO.Prevout)
	undoIdx := 0

	// Iterate transactions in forward order (same as block), skipping coinbase
	for i := 1; i < len(block.Transactions); i++ {
		tx := block.Transactions[i]
		txHash := tx.TxHash().String()

		// Loop through THIS transaction's inputs
		for _, txIn := range tx.TxIn {
			if undoIdx >= len(undoPrevouts) {
				return results
			}

			results[txHash] = append(results[txHash], cli_IO.Prevout{
				ValueSats:       undoPrevouts[undoIdx].ValueSats,
				ScriptPubkeyHex: undoPrevouts[undoIdx].ScriptPubkeyHex,
				Txid:            txIn.PreviousOutPoint.Hash.String(),
				Vout:            txIn.PreviousOutPoint.Index,
			})
			undoIdx++
		}
	}

	return results
}

func computeBlockStats(reports []*cli_IO.TransactionReport) cli_IO.BlockStats {
	totalFees := int64(0)
	totalWeight := int64(0)
	totalVbytes := int64(0)
	scriptSummary := make(map[string]int)

	for _, tr := range reports {
		if tr == nil {
			continue
		}
		totalFees += tr.FeeSats
		totalWeight += tr.Weight
		totalVbytes += tr.Vbytes

		for _, v := range tr.Vout {
			scriptSummary[v.ScriptType]++
		}
	}

	avgFeeRate := float64(0)
	if totalVbytes > 0 {
		avgFeeRate = float64(totalFees) / float64(totalVbytes)
	}

	return cli_IO.BlockStats{
		TotalFeesSats:   totalFees,
		TotalWeight:     totalWeight,
		AvgFeeRateSatVb: avgFeeRate,
		ScriptTypeSummary: cli_IO.ScriptTypeSummary{
			P2Wpkh:   scriptSummary["p2wpkh"],
			P2Tr:     scriptSummary["p2tr"],
			P2Sh:     scriptSummary["p2sh"],
			P2Pkh:    scriptSummary["p2pkh"],
			P2Wsh:    scriptSummary["p2wsh"],
			OpReturn: scriptSummary["op_return"],
			Unknown:  scriptSummary["unknown"],
		},
	}
}

func parseCoinbaseTx(tx *wire.MsgTx) cli_IO.Coinbase {
	height := parseCoinbaseHeight(tx)
	coinbaseScriptHex := hex.EncodeToString(tx.TxIn[0].SignatureScript)
	totalOutput := int64(0)
	for _, out := range tx.TxOut {
		totalOutput += out.Value
	}
	return cli_IO.Coinbase{
		Bip34Height:       height,
		CoinbaseScriptHex: coinbaseScriptHex,
		TotalOutputSats:   totalOutput,
	}
}

func processBlock(block *wire.MsgBlock, undo []cli_IO.ValidPrevOut) *cli_IO.Block {
	prevouts := createFullPrevOut(block, undo)
	newBlock := cli_IO.Block{
		Ok:   true,
		Mode: "block",
		BlockHeader: cli_IO.BlockHeader{
			Version:         block.Header.Version,
			PrevBlockHash:   block.Header.PrevBlock.String(),
			MerkleRoot:      block.Header.MerkleRoot.String(),
			MerkleRootValid: true, // we can compute this ourselves if needed
			Timestamp:       block.Header.Timestamp.Unix(),
			Bits:            fmt.Sprintf("%08x", block.Header.Bits),
			Nonce:           block.Header.Nonce,
			BlockHash:       block.BlockHash().String(),
		},
	}

	transactionsReports := make([]*cli_IO.TransactionReport, 0)
	// Only the first transaction should be coinbase
	for i, tx := range block.Transactions {
		if i == 0 && isCoinbase(tx) {
			newBlock.Coinbase = parseCoinbaseTx(tx)
			continue
		}
		rawTx, _ := TxToRawHex(tx)
		txHash := tx.TxHash().String()
		txPrevouts := prevouts[txHash]

		txInp := cli_IO.TransactionInput{
			Network:  "mainnet",
			RawTx:    rawTx,
			Prevouts: txPrevouts,
		}
		tr, err := GenerateTransactionReport(txInp)
		if !err.Ok {
			continue
		}
		if tr != nil {
			transactionsReports = append(transactionsReports, tr)
		}
	}

	newBlock.BlockStats = computeBlockStats(transactionsReports)
	// tx_count is total transactions including coinbase
	newBlock.TxCount = len(block.Transactions)
	newBlock.Transactions = transactionsReports

	return &newBlock
}

func ProcessBlocks(blkPath, revPath, xorPath string) bool {
	callBack := func(block *wire.MsgBlock, undo []cli_IO.ValidPrevOut) {
		processed := processBlock(block, undo)
		b, err := json.MarshalIndent(processed, "", "  ")
		if err != nil {
			slog.Error("ErrorDetails happen", err)
		}
		fileName := cli_IO.ToJsonFileName(BlockHashReversedHex(processed))
		cli_IO.WriteTransactionReportToFile(b, "/home/da3l/GolandProjects/2026-developer-challenge-1-chain-lens-MohamedKamal000/out/"+fileName)
	}
	readBlkRevFiles(blkPath, revPath, xorPath, callBack)
	return true
}
