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
	serverMode := flag.Bool("server", false, "start web server mode")
	port := flag.String("port", "8080", "port for web server (default: 8080)")

	flag.Parse()

	if *serverMode {
		//slog.Info("Starting ChainLens API server", "port", *port)
		if err := StartServer(*port); err != nil {
			// slog.Error("Server failed to start", "error", err)
			os.Exit(2)
		}
		os.Exit(1)
	}

	if *blockMode {
		args := flag.Args()
		if len(args) < 3 {
			slog.Error("Block mode requires 3 arguments: <blk.dat> <rev.dat> <xor.dat>")
			os.Exit(1)
		}
		blkFile := args[0]
		revFile := args[1]
		xorFile := args[2]

		finished := ProcessBlocks(blkFile, revFile, xorFile, true)
		if !finished {
			os.Exit(1)
			// for debugging
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
		cli_IO.WriteTransactionReportToFile(b, "../out"+"/"+fileName)
	}

	os.Exit(0)
}
