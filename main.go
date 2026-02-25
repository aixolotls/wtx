package main

import (
	"fmt"
	"os"

	wtxcmd "github.com/aixolotls/wtx/cmd"
)

func main() {
	if err := wtxcmd.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, "wtx error:", err)
		os.Exit(1)
	}
}
