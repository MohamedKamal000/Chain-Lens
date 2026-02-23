package main

import (
	"analyzer/cli_IO"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
)

func main() {
	blockMode := flag.Bool("block", false, "block mode")

	flag.Parse()

	if *blockMode {
		args := flag.Args()
		if len(args) < 3 {
			slog.Error("Block mode requires 3 arguments: <blk.dat> <rev.dat> <xor.dat>")
			os.Exit(1)
		}
		blkFile := args[0]
		revFile := args[1]
		xorFile := args[2]

		finished := ProcessBlocks(blkFile, revFile, xorFile)
		if finished {
			slog.Info("Block processing completed successfully.")
		}
		os.Exit(0)
	} else {
		args := flag.Args()
		TransactionFixturePath := args[0]
		tf, err := cli_IO.ReadTransactionFixture(TransactionFixturePath)
		if err != nil {
			slog.Error("ErrorDetails happen", err)
			os.Exit(1)
		}
		tr, _ := GenerateTransactionReport(tf)
		b, err := json.MarshalIndent(tr, "", "  ")
		if err != nil {
			slog.Error("ErrorDetails happen", err)
		}
		fmt.Println(string(b))
		fileName := cli_IO.ToJsonFileName(tr.Txid)
		cli_IO.WriteTransactionReportToFile(b, "../"+SINGLE_TRANSACTION_DIRECTORY+fileName)
	}

	os.Exit(0)
}
