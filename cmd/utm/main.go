package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
)

// utm generates a man page from the usage output of a command.
func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Generate a man page from the usage output of a command\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s <command>\n", os.Args[0])
	}

	if len(os.Args) != 2 || os.Args[1] == "-h" || os.Args[1] == "--help" {
		if len(os.Args) == 2 {
			flag.Usage()
			os.Exit(0)
		}

		flag.Usage()
		os.Exit(2)
	}

	name := os.Args[1]

	fmt.Printf(".TH %s 1\n", strings.ToUpper(name))
	fmt.Println(".SH NAME")
	fmt.Printf("%s \\- command-line tool\n", name)

	fmt.Println(".SH SYNOPSIS")
	fmt.Printf(".B %s\n", name)

	fmt.Println(".SH DESCRIPTION")

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "Usage:") {
			fmt.Println(".SH SYNOPSIS")
			fmt.Printf(".B %s\n", strings.TrimSpace(strings.TrimPrefix(line, "Usage:")))
			continue
		}

		if line == "Flags:" {
			fmt.Println(".SH OPTIONS")
			continue
		}

		if line == "" {
			continue
		}

		fmt.Println(line)
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "error reading input:", err)
		os.Exit(1)
	}
}
