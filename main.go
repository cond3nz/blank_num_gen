package main

import (
	"fmt"
	"os"
)

// run — точка входа: с аргументами работает консольный режим, без них — GUI.
func run() error {
	if len(os.Args) > 1 {
		return runCLI(os.Args[1:], os.Stdout)
	}
	return runGUI()
}

func main() {
	if err := run(); err != nil {
		showFatalError(err)
		fmt.Fprintln(os.Stderr, "blank_num_gen:", err)
		os.Exit(1)
	}
}
