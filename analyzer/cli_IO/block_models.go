package cli_IO

type BlockHeader struct {
	Version         int32  `json:"version"`
	PrevBlockHash   string `json:"prev_block_hash"`
	MerkleRoot      string `json:"merkle_root"`
	MerkleRootValid bool   `json:"merkle_root_valid"`
	Timestamp       int64  `json:"timestamp"`
	Bits            string `json:"bits"`
	Nonce           uint32 `json:"nonce"`
	BlockHash       string `json:"block_hash"`
}

type Coinbase struct {
	Bip34Height       uint32 `json:"bip34_height"`
	CoinbaseScriptHex string `json:"coinbase_script_hex"`
	TotalOutputSats   int64  `json:"total_output_sats"`
}

type ScriptTypeSummary struct {
	P2Wpkh   int `json:"p2wpkh"`
	P2Tr     int `json:"p2tr"`
	P2Sh     int `json:"p2sh"`
	P2Pkh    int `json:"p2pkh"`
	P2Wsh    int `json:"p2wsh"`
	OpReturn int `json:"op_return"`
	Unknown  int `json:"unknown"`
}

type BlockStats struct {
	TotalFeesSats     int64             `json:"total_fees_sats"`
	TotalWeight       int64             `json:"total_weight"`
	AvgFeeRateSatVb   float64           `json:"avg_fee_rate_sat_vb"`
	ScriptTypeSummary ScriptTypeSummary `json:"script_type_summary"`
}

type Block struct {
	Ok           bool                 `json:"ok"`
	Mode         string               `json:"mode"`
	BlockHeader  BlockHeader          `json:"block_header"`
	TxCount      int                  `json:"tx_count"`
	Coinbase     Coinbase             `json:"coinbase"`
	Transactions []*TransactionReport `json:"transactions"`
	BlockStats   BlockStats           `json:"block_stats"`
}
