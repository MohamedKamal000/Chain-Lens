package main

import (
	"analyzer/cli_IO"
	"bytes"
	"encoding/hex"
	"errors"
	"log/slog"
	"math"

	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

/*
Field requirements:
txid: hex string (64 chars), standard display convention.
wtxid: must be null for non-SegWit transactions.
fee_rate_sat_vb: JSON number; evaluator accepts small rounding differences (+/-0.01).
address: required for recognized types (on both inputs and outputs), else null.
vout[n].n: must equal the output index n (0-based).
witness: for legacy txs, return [] for each input. For SegWit, return the exact witness stack items in order (hex strings, including empty items as "").
warnings: order does not matter.
segwit_savings: must be null for non-SegWit transactions.
*/

/*
OP_RETURN payload decoding
For outputs classified as op_return, add three additional fields:

op_return_data_hex: concatenation of all data pushes after OP_RETURN, in order. If there are no data pushes (bare OP_RETURN), return "".
op_return_data_utf8: UTF-8 decode of the raw bytes. If the bytes are not valid UTF-8, return null.
op_return_protocol: detect known protocols by prefix:
"omni" — data starts with 6f6d6e69 (ASCII "omni")
"opentimestamps" — data starts with 0109f91102
"unknown" — anything else (including empty)
Parsing requirement: OP_RETURN payloads may use any valid push opcode (direct push 0x01-0x4b, OP_PUSHDATA1, OP_PUSHDATA2, OP_PUSHDATA4).
Your parser must handle all of these, not just assume a single direct push. Multiple push operations after OP_RETURN are concatenated.
*/

/*
on error (invalid fixture, malformed tx, inconsistent prevouts, etc.)
If a prevout is missing, duplicated, or does not correspond to an input outpoint, you must error.
On errors:

error.code and error.message must be present and non-empty strings.
Error output (on failures) must be:

{ "ok": false, "error": { "code": "INVALID_TX", "message": "..." } }
*/
func validateTransaction() bool {
	return false
}

func matchTxIdToPrevOuts(ins []*wire.TxIn, prevOuts []cli_IO.Prevout) (map[string]cli_IO.ValidPrevOut, cli_IO.CliError) {
	inputPrevOutFreq := make(map[string]int)
	for _, in := range ins {
		inputPrevOutFreq[MergeTxIdWithIndex(in.PreviousOutPoint.Hash.String(), in.PreviousOutPoint.Index)]++
	}

	givenPrevOuts := make(map[string]int)
	for _, prevOut := range prevOuts {
		givenPrevOuts[MergeTxIdWithIndex(prevOut.Txid, prevOut.Vout)]++
	}

	if len(inputPrevOutFreq) != len(givenPrevOuts) {
		return nil, cli_IO.NewErrorWithLog(errors.New("wrong Prevout exist"), false, cli_IO.INCONSISTENT_PREVOUTS, "INCONSISTENT_PREVOUTS")
	}

	for key, value := range inputPrevOutFreq {
		if givenPrevOuts[key] == 0 || value != givenPrevOuts[key] {
			return nil, cli_IO.NewErrorWithLog(errors.New("wrong Prevout exist"), false, cli_IO.INCONSISTENT_PREVOUTS, "INCONSISTENT_PREVOUTS")
		}
	}

	result := map[string]cli_IO.ValidPrevOut{}
	for _, prevOut := range prevOuts {
		result[MergeTxIdWithIndex(prevOut.Txid, prevOut.Vout)] = cli_IO.ValidPrevOut{
			ValueSats:       prevOut.ValueSats,
			ScriptPubkeyHex: prevOut.ScriptPubkeyHex,
		}
	}
	return result, cli_IO.CliError{Ok: true}
}

func applyWarnings(transactionReport *cli_IO.TransactionReport) {
	if transactionReport.FeeSats > FEE_SATS_THRESHOLD || transactionReport.FeeRateSatVb > FEE_RATE_VB_THRESHOLD {
		transactionReport.Warnings = append(transactionReport.Warnings, cli_IO.Warning{
			Code: HIGH_FEE,
		})
	}

	for _, v := range transactionReport.Vout {
		if v.ScriptType != "op_return" && v.ValueSats < OP_RETURN_SATS_THRESHOLD {
			transactionReport.Warnings = append(transactionReport.Warnings, cli_IO.Warning{
				Code: DUST_OUTPUT,
			})
			break
		}
	}

	for _, v := range transactionReport.Vout {
		if v.ScriptType == "unknown" {
			transactionReport.Warnings = append(transactionReport.Warnings, cli_IO.Warning{
				Code: UNKNOWN_OUTPUT_SCRIPT,
			})
			break
		}
	}

	for _, v := range transactionReport.Vin {
		if v.Sequence < RBF_SIGNALING_THRESHOLD {
			transactionReport.Warnings = append(transactionReport.Warnings, cli_IO.Warning{
				Code: RBF_SIGNALING,
			})
			transactionReport.RbfSignaling = true
			break
		}
	}
}

func decodeRawTransaction(rawTx string) (*wire.MsgTx, error) {
	res, err := hex.DecodeString(rawTx)
	var msgTx wire.MsgTx
	err = msgTx.Deserialize(bytes.NewReader(res))
	if err != nil {
		slog.Error("ErrorDetails happen", err)
	}
	return &msgTx, err
}

func constructVin(txIn *wire.TxIn, validPrevOut map[string]cli_IO.ValidPrevOut) cli_IO.Vin {
	txId := txIn.PreviousOutPoint.Hash.String()
	index := txIn.PreviousOutPoint.Index
	prevOut := validPrevOut[MergeTxIdWithIndex(txId, index)]
	result := cli_IO.Vin{
		Txid:         txId,
		Index:        index,
		Sequence:     txIn.Sequence,
		ScriptSigHex: hex.EncodeToString(txIn.SignatureScript),
		ScriptAsm: func() string {
			asm, err := txscript.DisasmString(txIn.SignatureScript)
			if err != nil {
				return ""
			}
			return asm
		}(),
		ScriptType:       MapVinScriptType(prevOut.ScriptPubkeyHex, txIn),
		Address:          ExtractAddressFromPkScript(prevOut.ScriptPubkeyHex),
		PrevOut:          prevOut,
		RelativeTimelock: ComputeRelativeTimelock(txIn.Sequence),
		Witness:          make([]string, 0),
	}
	if len(txIn.Witness) > 0 {
		for _, item := range txIn.Witness {
			result.Witness = append(result.Witness, hex.EncodeToString(item))
		}
	}
	return result
}

func constructVout(txOut *wire.TxOut, index int) cli_IO.Vout {
	class := txscript.GetScriptClass(txOut.PkScript)

	vout := cli_IO.Vout{
		N:               index,
		ValueSats:       txOut.Value,
		ScriptPubkeyHex: hex.EncodeToString(txOut.PkScript),
		ScriptAsm: func() string {
			asm, err := txscript.DisasmString(txOut.PkScript)
			if err != nil {
				return ""
			}
			return asm
		}(),
		ScriptType: MapVoutScriptType(class),
		Address:    ExtractAddressFromPkScript(hex.EncodeToString(txOut.PkScript)),
	}

	if class == txscript.NullDataTy {
		handleOpReturn(&vout, txOut.PkScript)
	}

	return vout
}

func calculateSats(report *cli_IO.TransactionReport) {
	totalInputSats := int64(0)
	totalOutputSats := int64(0)

	for _, v := range report.Vin {
		totalInputSats += v.PrevOut.ValueSats
	}

	for _, v := range report.Vout {
		totalOutputSats += v.ValueSats
	}

	report.TotalInputSats = totalInputSats
	report.TotalOutputSats = totalOutputSats
	report.FeeSats = totalInputSats - totalOutputSats
	report.FeeRateSatVb = float64(report.FeeSats) / float64(report.Vbytes)
}

func calculateSegwitSavings(msgTx *wire.MsgTx) (*cli_IO.SegwitSavings, error) {
	fullBuf := new(bytes.Buffer)
	if err := msgTx.Serialize(fullBuf); err != nil {
		return nil, err
	}
	totalBytes := fullBuf.Len()

	nonWitnessBuf := new(bytes.Buffer)
	if err := msgTx.SerializeNoWitness(nonWitnessBuf); err != nil {
		return nil, err
	}
	nonWitnessBytes := nonWitnessBuf.Len()

	witnessBytes := totalBytes - nonWitnessBytes

	weightActual := (nonWitnessBytes * 3) + totalBytes

	weightIfLegacy := totalBytes * 4

	var savingsPct float64
	if weightIfLegacy > 0 {
		savingsPct = (1 - float64(weightActual)/float64(weightIfLegacy)) * 100
		savingsPct = math.Round(savingsPct*100) / 100
	}

	// 7. If there is no witness data, return nil
	if witnessBytes == 0 {
		return nil, nil
	}

	return &cli_IO.SegwitSavings{
		WitnessBytes:    witnessBytes,
		NonWitnessBytes: nonWitnessBytes,
		TotalBytes:      totalBytes,
		WeightActual:    weightActual,
		WeightIfLegacy:  weightIfLegacy,
		SavingsPct:      savingsPct,
	}, nil
}

func AddSigWitIfExists(report *cli_IO.TransactionReport, msgTx *wire.MsgTx) {
	if !msgTx.HasWitness() {
		return
	}
	segSavings, err := calculateSegwitSavings(msgTx)
	if err != nil {
		// handle error or just skip
		return
	}

	if segSavings != nil {
		report.SegwitSavings = segSavings
		report.Segwit = true
		wTxid := msgTx.WitnessHash().String()
		report.Wtxid = &wTxid
	} else {
		report.SegwitSavings = nil
		report.Wtxid = nil
	}
}

func constructBaseTransaction(input cli_IO.TransactionInput) (*cli_IO.TransactionReport, cli_IO.CliError) {
	// validation to determine the OK
	msgTx, err := decodeRawTransaction(input.RawTx)
	if err != nil {
		slog.Error("ErrorDetails happen", err)
	}
	vPO, cliErr := matchTxIdToPrevOuts(msgTx.TxIn, input.Prevouts)
	if !cliErr.Ok {
		return nil, cliErr
	}

	transactionReport := cli_IO.TransactionReport{}
	transactionReport.Ok = true
	transactionReport.Network = input.Network
	transactionReport.Txid = msgTx.TxID()
	transactionReport.Version = msgTx.Version
	transactionReport.Locktime = msgTx.LockTime
	transactionReport.LocktimeType = GetLockTimeType(msgTx.LockTime)
	transactionReport.LocktimeValue = msgTx.LockTime
	transactionReport.Vin = make([]cli_IO.Vin, 0)
	transactionReport.Vout = make([]cli_IO.Vout, 0)
	transactionReport.Warnings = make([]cli_IO.Warning, 0)
	sizeBytes, weight, vbytes, err := calculateTxSizes(msgTx)
	transactionReport.SizeBytes = sizeBytes
	transactionReport.Weight = weight
	transactionReport.Vbytes = vbytes

	for _, v := range msgTx.TxIn {
		transactionReport.Vin = append(transactionReport.Vin, constructVin(v, vPO))
	}

	for i, v := range msgTx.TxOut {
		transactionReport.Vout = append(transactionReport.Vout, constructVout(v, i))
	}

	calculateSats(&transactionReport)
	applyWarnings(&transactionReport)
	AddSigWitIfExists(&transactionReport, msgTx)
	return &transactionReport, cli_IO.CliError{Ok: true}
}

func GenerateTransactionReport(transactionInput cli_IO.TransactionInput) (*cli_IO.TransactionReport, cli_IO.CliError) {
	tr, cliErr := constructBaseTransaction(transactionInput)
	return tr, cliErr
}
