package cli_IO

type Prevout struct {
	Txid            string `json:"txid"`
	Vout            uint32 `json:"vout"`
	ValueSats       int64  `json:"value_sats"`
	ScriptPubkeyHex string `json:"script_pubkey_hex"`
}

type TransactionInput struct {
	Network  string    `json:"network"`
	RawTx    string    `json:"raw_tx"`
	Prevouts []Prevout `json:"prevouts"`
}

type SegwitSavings struct {
	WitnessBytes    int     `json:"witness_bytes"`
	NonWitnessBytes int     `json:"non_witness_bytes"`
	TotalBytes      int     `json:"total_bytes"`
	WeightActual    int     `json:"weight_actual"`
	WeightIfLegacy  int     `json:"weight_if_legacy"`
	SavingsPct      float64 `json:"savings_pct"`
}

type RelativeTimelock struct {
	Enabled bool   `json:"enabled"`
	Type    string `json:"type,omitempty"`
	Value   uint32 `json:"value,omitempty"`
}

type ValidPrevOut struct {
	ValueSats       int64  `json:"value_sats"`
	ScriptPubkeyHex string `json:"script_pubkey_hex"`
}

type Vin struct {
	Txid             string           `json:"txid"`
	Index            uint32           `json:"vout"`
	Sequence         uint32           `json:"sequence"`
	ScriptSigHex     string           `json:"script_sig_hex"`
	ScriptAsm        string           `json:"script_asm"`
	Witness          []string         `json:"witness"`
	ScriptType       string           `json:"script_type"`
	Address          *string          `json:"address"`
	PrevOut          ValidPrevOut     `json:"prevout"`
	RelativeTimelock RelativeTimelock `json:"relative_timelock"`
}

type Vout struct {
	N                int     `json:"n"`
	ValueSats        int64   `json:"value_sats"`
	ScriptPubkeyHex  string  `json:"script_pubkey_hex"`
	ScriptAsm        string  `json:"script_asm"`
	ScriptType       string  `json:"script_type"`
	Address          *string `json:"address"`
	OpReturnDataHex  string  `json:"op_return_data_hex,omitempty"`
	OpReturnDataUtf8 *string `json:"op_return_data_utf8,omitempty"`
	OpReturnProtocol string  `json:"op_return_protocol,omitempty"`
}

type Warning struct {
	Code string `json:"code"`
}

type TransactionReport struct {
	Ok              bool           `json:"ok"`
	Network         string         `json:"network"`
	Segwit          bool           `json:"segwit"` // seg
	Txid            string         `json:"txid"`
	Wtxid           *string        `json:"wtxid"` // seg
	Version         int32          `json:"version"`
	Locktime        uint32         `json:"locktime"`
	SizeBytes       int64          `json:"size_bytes"` // size_bytes (weight-related interpretation changes)
	Weight          int64          `json:"weight"`     // seg
	Vbytes          int64          `json:"vbytes"`     // seg
	TotalInputSats  int64          `json:"total_input_sats"`
	TotalOutputSats int64          `json:"total_output_sats"`
	FeeSats         int64          `json:"fee_sats"`
	FeeRateSatVb    float64        `json:"fee_rate_sat_vb"`
	RbfSignaling    bool           `json:"rbf_signaling"`
	LocktimeType    string         `json:"locktime_type"`
	LocktimeValue   uint32         `json:"locktime_value"`
	SegwitSavings   *SegwitSavings `json:"segwit_savings"` // seg
	Vin             []Vin          `json:"vin"`
	Vout            []Vout         `json:"vout"`
	Warnings        []Warning      `json:"warnings"`
}
