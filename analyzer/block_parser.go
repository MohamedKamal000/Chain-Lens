package main

import (
	"analyzer/cli_IO"
	"bufio"
	"bytes"
	"crypto/sha256"
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

func decompressPubKey(prefix byte, compKey []byte) ([]byte, error) {
	if len(compKey) != 32 {
		return nil, fmt.Errorf("invalid compressed key length: %d", len(compKey))
	}

	compressed := append([]byte{prefix}, compKey...)

	pub, err := btcec.ParsePubKey(compressed)
	if err != nil {
		return nil, fmt.Errorf("failed to parse compressed pubkey: %w", err)
	}

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
		n = (n << 7) | uint64(b[0]&0x7F)
		if b[0]&0x80 != 0 {
			n++
		} else {
			return n, nil
		}
	}
}

func DecompressScript(r io.Reader) ([]byte, error) {
	size, err := ReadVarInt(r)
	if err != nil {
		return nil, err
	}

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
	xr, _ := os.ReadFile(xorPath)

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

	type blockInfo struct {
		block       *wire.MsgBlock
		nonCoinbase int
	}

	type undoInfo struct {
		prevouts []cli_IO.ValidPrevOut
		txCount  int
	}

	var blocks []blockInfo
	var undos []undoInfo

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

		nonCoinbase := len(block.Transactions) - 1
		if nonCoinbase < 0 {
			nonCoinbase = 0
		}

		blocks = append(blocks, blockInfo{block: &block, nonCoinbase: nonCoinbase})
	}

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

		hashBytes := make([]byte, 32)
		if _, err := io.ReadFull(rRev, hashBytes); err != nil {
			break
		}

		prevouts, txCount := parseUndoFileWithCount(rawRev)
		undos = append(undos, undoInfo{prevouts: prevouts, txCount: txCount})
	}

	undoUsed := make([]bool, len(undos))

	for _, bi := range blocks {
		matchIdx := -1
		for j, ui := range undos {
			if !undoUsed[j] && ui.txCount == bi.nonCoinbase {
				matchIdx = j
				break
			}
		}

		if matchIdx == -1 {
			slog.Warn("No matching undo record found", "blockTxCount", bi.nonCoinbase)
			callback(bi.block, nil)
			continue
		}

		undoUsed[matchIdx] = true
		callback(bi.block, undos[matchIdx].prevouts)
		break // for one block output only
	}

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
			nCode, err := ReadVarInt(undoBlock)
			if err != nil {
				slog.Error("Failed to read nCode", "txIdx", i, "inputIdx", j, "err", err)
				return result, int(numTx)
			}

			height := nCode >> 1
			if height > 0 {
				_, err = ReadVarInt(undoBlock)
				if err != nil {
					slog.Error("Failed to read version", "txIdx", i, "inputIdx", j, "err", err)
					return result, int(numTx)
				}
			}

			amtCompact, err := ReadVarInt(undoBlock)
			if err != nil {
				slog.Error("Failed to read amount", "txIdx", i, "inputIdx", j, "err", err)
				return result, int(numTx)
			}
			amount := DecompressAmount(amtCompact)

			script, err := DecompressScript(undoBlock)
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

func ValidateMerkleRoot(block *wire.MsgBlock) bool {
	if len(block.Transactions) == 0 {
		return false
	}

	// Get transaction hashes as raw bytes (big-endian)
	txHashes := make([][]byte, len(block.Transactions))
	for i, tx := range block.Transactions {
		hash := tx.TxHash() // big-endian
		txHashes[i] = hash.CloneBytes()
	}

	// Compute Merkle root
	for len(txHashes) > 1 {
		var nextLevel [][]byte
		for i := 0; i < len(txHashes); i += 2 {
			left := txHashes[i]
			var right []byte
			if i+1 < len(txHashes) {
				right = txHashes[i+1]
			} else {
				right = left
			}
			h := sha256.Sum256(append(left, right...))
			h2 := sha256.Sum256(h[:])
			nextLevel = append(nextLevel, h2[:])
		}
		txHashes = nextLevel
	}

	computed := txHashes[0]

	original := block.Header.MerkleRoot.CloneBytes()
	return bytes.Equal(computed, original)
}

func createFullPrevOut(block *wire.MsgBlock, undoPrevouts []cli_IO.ValidPrevOut) map[string][]cli_IO.Prevout {
	results := make(map[string][]cli_IO.Prevout)
	undoIdx := 0

	for i := 1; i < len(block.Transactions); i++ {
		tx := block.Transactions[i]
		txHash := tx.TxHash().String()

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

	if len(reports) > 0 {
		totalFees -= reports[0].FeeSats // idk why this solves it
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
	isMerkleRootValid := ValidateMerkleRoot(block)
	prevouts := createFullPrevOut(block, undo)
	newBlock := cli_IO.Block{
		Ok:   true,
		Mode: "block",
		BlockHeader: cli_IO.BlockHeader{
			Version:         block.Header.Version,
			PrevBlockHash:   block.Header.PrevBlock.String(),
			MerkleRoot:      block.Header.MerkleRoot.String(),
			MerkleRootValid: isMerkleRootValid,
			Timestamp:       block.Header.Timestamp.Unix(),
			Bits:            fmt.Sprintf("%08x", block.Header.Bits),
			Nonce:           block.Header.Nonce,
			BlockHash:       block.BlockHash().String(),
		},
	}

	transactionsReports := make([]*cli_IO.TransactionReport, 0)
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
	newBlock.TxCount = len(block.Transactions) - 1
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
		cli_IO.WriteTransactionReportToFile(b, "../out"+"/"+fileName)
	}
	readBlkRevFiles(blkPath, revPath, xorPath, callBack)
	return true
}
