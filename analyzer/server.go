package main

import (
	"analyzer/cli_IO"
	"encoding/json"
	"net/http"
	"os"
)

// AnalyzeRequest represents the JSON body for the /api/analyze endpoint
type AnalyzeRequest struct {
	Mode string `json:"mode"` // "transaction" or "block"

	// Transaction mode fields
	Network  string           `json:"network,omitempty"`
	RawTx    string           `json:"raw_tx,omitempty"`
	Prevouts []cli_IO.Prevout `json:"prevouts,omitempty"`

	// Block mode fields (file paths)
	BlkFile string `json:"blk_file,omitempty"`
	RevFile string `json:"rev_file,omitempty"`
	XorFile string `json:"xor_file,omitempty"`
}

// AnalyzeResponse represents the response from the /api/analyze endpoint
type AnalyzeResponse struct {
	Ok    bool        `json:"ok"`
	Data  interface{} `json:"data,omitempty"`
	Error string      `json:"error,omitempty"`
}

// HealthResponse represents the response from the /api/health endpoint
type HealthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

// enableCORS adds CORS headers to allow cross-origin requests
func enableCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

// healthHandler handles GET /api/health
func healthHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	response := HealthResponse{
		Status:  "ok",
		Version: "1.0.0",
	}

	json.NewEncoder(w).Encode(response)
}

// analyzeHandler handles POST /api/analyze
func analyzeHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	var req AnalyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response := AnalyzeResponse{
			Ok:    false,
			Error: "Invalid JSON body: " + err.Error(),
		}
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	switch req.Mode {
	case "transaction", "":
		handleTransactionAnalysis(w, req)
	case "block":
		handleBlockAnalysis(w, req)
	default:
		response := AnalyzeResponse{
			Ok:    false,
			Error: "Invalid mode. Use 'transaction' or 'block'",
		}
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
	}
}

// handleTransactionAnalysis processes a transaction analysis request
func handleTransactionAnalysis(w http.ResponseWriter, req AnalyzeRequest) {
	if req.RawTx == "" {
		response := AnalyzeResponse{
			Ok:    false,
			Error: "raw_tx is required for transaction analysis",
		}
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	transactionInput := cli_IO.TransactionInput{
		Network:  req.Network,
		RawTx:    req.RawTx,
		Prevouts: req.Prevouts,
	}

	if transactionInput.Prevouts == nil {
		transactionInput.Prevouts = []cli_IO.Prevout{}
	}

	report, cliErr := GenerateTransactionReport(transactionInput)
	if !cliErr.Ok {
		response := AnalyzeResponse{
			Ok:    false,
			Error: cliErr.Error.Message,
		}
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	response := AnalyzeResponse{
		Ok:   true,
		Data: report,
	}
	json.NewEncoder(w).Encode(response)
}

// handleBlockAnalysis processes a block analysis request
func handleBlockAnalysis(w http.ResponseWriter, req AnalyzeRequest) {
	if req.BlkFile == "" || req.RevFile == "" || req.XorFile == "" {
		response := AnalyzeResponse{
			Ok:    false,
			Error: "blk_file, rev_file, and xor_file are required for block analysis",
		}
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	// ProcessBlocks writes to stdout, but for the API we want to capture the result
	// For now, we'll return success/failure status
	finished := ProcessBlocks(req.BlkFile, req.RevFile, req.XorFile, false)

	if !finished {
		response := AnalyzeResponse{
			Ok:    false,
			Error: "Block processing failed",
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	response := AnalyzeResponse{
		Ok: true,
		Data: map[string]string{
			"message": "Block processing completed successfully",
		},
	}
	json.NewEncoder(w).Encode(response)
}

// StartServer starts the HTTP server on the specified port
func StartServer(port string) error {
	http.HandleFunc("/api/health", healthHandler)
	http.HandleFunc("/api/analyze", analyzeHandler)

	// Serve static files for the web UI
	webDir := "./WebVisualizer/dist"
	if _, err := os.Stat(webDir); err == nil {
		fs := http.FileServer(http.Dir(webDir))
		http.Handle("/", http.StripPrefix("/", fs))
	}

	return http.ListenAndServe(":"+port, nil)
}
