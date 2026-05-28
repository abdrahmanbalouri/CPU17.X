package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Corewar assembler")
		fmt.Println("Usage: asm <file.s> [file.s ...]")
		os.Exit(1)
	}

	for _, input := range os.Args[1:] {
		if !strings.HasSuffix(input, ".s") {
			fmt.Fprintln(os.Stderr, "Error:", input, "must be a .s file")
			os.Exit(1)
		}

		output := strings.TrimSuffix(input, ".s") + ".cor"
		if err := Assemble(input, output); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		fmt.Printf("Writing output program to %s\n", output)
	}
}
