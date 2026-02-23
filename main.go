package main

import (
	"fmt"
	"os"

	wtxcmd "github.com/mrbonezy/wtx/cmd"
)

func main() {
	if err := wtxcmd.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, "wtx error:", err)
		os.Exit(1)
	}
}
