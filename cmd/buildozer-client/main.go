package main

import (
"os"

"github.com/Manu343726/buildozer/cmd/buildozer-client/cmd"
)

func main() {
	rootCmd := cmd.NewRootCommand()
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
