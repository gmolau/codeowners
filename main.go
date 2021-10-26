package main

import (
	"flag"
	"fmt"
	"log"
	"os"
)

func main() {
	flag.Usage = usage
	flag.Parse()

	root, err := parseDir()
	if err != nil {
		log.Fatal(fmt.Errorf("error while parsing root dir: %w", err))
	}

	rewrittenCodeownerRules, err := RewriteCodeownersRules(root)
	if err != nil {
		log.Fatal(fmt.Errorf("error while rewriting codeowner rules in %s: %w", root, err))
	}

	if len(rewrittenCodeownerRules) == 0 {
		log.Fatal(fmt.Errorf("no CODEOWNER rules found in %s", root))
	}

	generatedCodeownersFile := GenerateCodeownersFile(rewrittenCodeownerRules)

	_, err = fmt.Printf(generatedCodeownersFile)
	if err != nil {
		log.Fatal(fmt.Errorf("error while printing generated filed: %w", err))
	}
}

func usage() {
	_, err := fmt.Fprintf(flag.CommandLine.Output(), "usage: %s [dir]\n", os.Args[0])
	if err != nil {
		log.Fatal(fmt.Errorf("error while printing usage info: %w", err))
	}
}

func parseDir() (string, error) {
	narg := flag.NArg()
	switch {
	case narg < 1:
		return "", fmt.Errorf("no dir given")
	case narg > 1:
		return "", fmt.Errorf("can only process one dir at a time, got %d: %s", narg, flag.Args())
	default:
		return flag.Arg(0), nil
	}
}
