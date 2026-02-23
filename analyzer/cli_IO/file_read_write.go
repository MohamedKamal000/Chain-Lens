package cli_IO

import (
	"encoding/json"
	"log/slog"
	"os"
)

func ToJsonFileName(txid string) string {
	return txid + ".json"
}

func ReadTransactionFixture(filePath string) (TransactionInput, error) {
	f, err := os.ReadFile(filePath)

	if err != nil {
		return TransactionInput{}, err
	}

	var transaction TransactionInput
	err = json.Unmarshal(f, &transaction)
	if err != nil {
		return TransactionInput{}, err
	}

	return transaction, nil
}

func WriteTransactionReportToFile(transactionReport []byte, filePath string) {
	err := os.WriteFile(filePath, transactionReport, 0644)
	if err != nil {
		slog.Error("ErrorDetails happen", err)
	}
}
