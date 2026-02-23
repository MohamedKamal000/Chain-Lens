package main

import (
	"analyzer/cli_IO"
	"bytes"
	"encoding/hex"
	"fmt"
	"log/slog"
	"math"
	"unicode/utf8"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

func GetLockTimeType(lockTime uint32) string {

	var locktimeType string

	if lockTime == 0 {
		locktimeType = "none"
	} else if lockTime < LockTimeThreshold {
		locktimeType = "block_height"
	} else {
		locktimeType = "unix_timestamp"
	}

	return locktimeType
}

/*
p2pkh
p2sh-p2wpkh
p2sh-p2wsh
p2wpkh
p2wsh
p2tr_keypath
p2tr_scriptpath
unknown
*/

func isNullAddress(class txscript.ScriptClass) bool {
	switch class {
	case txscript.NonStandardTy,
		txscript.MultiSigTy,
		txscript.NullDataTy,
		txscript.WitnessUnknownTy:
		return true
	default:
		return false
	}
}

func MapVoutScriptType(class txscript.ScriptClass) string {
	switch class {
	case txscript.PubKeyHashTy:
		return "p2pkh"
	case txscript.ScriptHashTy:
		return "p2sh"
	case txscript.WitnessV0PubKeyHashTy:
		return "p2wpkh"
	case txscript.WitnessV0ScriptHashTy:
		return "p2wsh"
	case txscript.WitnessV1TaprootTy:
		return "p2tr"
	case txscript.NullDataTy:
		return "op_return"
	default:
		return "unknown"
	}
}

func MapVinScriptType(pkScriptString string, txIn *wire.TxIn) string {
	pkScript, err := hex.DecodeString(pkScriptString)

	if err != nil {
		slog.Error("Error decoding pkScript", "err", err)
		return ""
	}

	class := txscript.GetScriptClass(pkScript)

	switch class {

	case txscript.PubKeyHashTy:
		return "p2pkh"

	case txscript.WitnessV0PubKeyHashTy:
		return "p2wpkh"

	case txscript.WitnessV0ScriptHashTy:
		return "p2wsh"

	case txscript.ScriptHashTy:
		pushes, err := txscript.PushedData(txIn.SignatureScript)
		if err != nil || len(pushes) == 0 {
			return "unknown"
		}

		redeemScript := pushes[len(pushes)-1]
		redeemClass := txscript.GetScriptClass(redeemScript)

		switch redeemClass {
		case txscript.WitnessV0PubKeyHashTy:
			return "p2sh-p2wpkh"
		case txscript.WitnessV0ScriptHashTy:
			return "p2sh-p2wsh"
		default:
			return "unknown"
		}
	case txscript.WitnessV1TaprootTy:
		if len(txIn.Witness) == 1 {
			return "p2tr_keypath"
		}
		return "p2tr_scriptpath"

	default:
		return "unknown"
	}
}

func ExtractAddressFromPkScript(pkScriptString string) *string {
	pkScript, err := hex.DecodeString(pkScriptString)

	if err != nil {
		slog.Error("Error decoding pkScript", "err", err)
		return nil
	}

	scriptClass, addresses, _, err := txscript.ExtractPkScriptAddrs(pkScript, &chaincfg.MainNetParams)

	if err != nil || len(addresses) == 0 || isNullAddress(scriptClass) {
		return nil
	}

	addr := addresses[0].EncodeAddress()
	return &addr
}

func MergeTxIdWithIndex(txId string, voutIndex uint32) string {
	return fmt.Sprintf("%s:%d", txId, voutIndex)
}

func ComputeRelativeTimelock(seq uint32) cli_IO.RelativeTimelock {
	if seq&disableFlag != 0 {
		// Relative timelock disabled
		return cli_IO.RelativeTimelock{Enabled: false}
	}

	isTime := seq&typeFlag != 0
	value := seq & mask

	rt := cli_IO.RelativeTimelock{Enabled: true}

	if isTime {
		rt.Type = "time"
		rt.Value = value * timeUnitSec
	} else {
		rt.Type = "blocks"
		rt.Value = value
	}

	return rt
}

func handleOpReturn(vout *cli_IO.Vout, pkScript []byte) {
	tokenizer := txscript.MakeScriptTokenizer(0, pkScript)

	if !tokenizer.Next() || tokenizer.Opcode() != txscript.OP_RETURN {
		return
	}

	var payload []byte

	for tokenizer.Next() {
		data := tokenizer.Data()
		if data != nil {
			payload = append(payload, data...)
		}
	}

	if tokenizer.Err() != nil {
		return
	}

	if len(payload) == 0 {
		vout.OpReturnDataHex = ""
		vout.OpReturnProtocol = "unknown"
		return
	}

	vout.OpReturnDataHex = hex.EncodeToString(payload)

	if utf8.Valid(payload) {
		str := string(payload)
		vout.OpReturnDataUtf8 = &str
	} else {
		vout.OpReturnDataUtf8 = nil
	}

	// Protocol detection
	switch {
	case bytes.HasPrefix(payload, []byte("omni")):
		vout.OpReturnProtocol = "omni"

	case bytes.HasPrefix(payload, []byte{0x01, 0x09, 0xf9, 0x11, 0x02}):
		vout.OpReturnProtocol = "opentimestamps"

	default:
		vout.OpReturnProtocol = "unknown"
	}
}

func calculateTxSizes(msgTx *wire.MsgTx) (sizeBytes int64, weight int64, vbytes int64, err error) {
	fullBuf := new(bytes.Buffer)
	if err := msgTx.Serialize(fullBuf); err != nil {
		return 0, 0, 0, err
	}
	fullSize := fullBuf.Len()

	nonWitnessBuf := new(bytes.Buffer)
	if err := msgTx.SerializeNoWitness(nonWitnessBuf); err != nil {
		return 0, 0, 0, err
	}
	nonWitnessSize := nonWitnessBuf.Len()

	weight = int64((nonWitnessSize * 3) + fullSize)

	vbytes = int64(math.Ceil(float64(weight) / 4.0))

	sizeBytes = int64(fullSize)

	return sizeBytes, weight, vbytes, nil
}
