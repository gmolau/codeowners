package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

func main() {
	flag.Usage = usage
	flag.Parse()

	repoPath, err := parseRepoPath()
	if err != nil {
		log.Fatal(err)
	}

	rewrittenCodeownerRules, err := walkRepo(repoPath)
	if err != nil {
		log.Fatal(err)
	}

	if len(rewrittenCodeownerRules) == 0 {
		log.Fatal(fmt.Errorf("no CODEOWNER rules found"))
	}

	generatedCodeownersFile := generateCodeownersFile(rewrittenCodeownerRules)

	_, err = fmt.Printf(generatedCodeownersFile)
	if err != nil {
		log.Fatal(err)
	}
}

func usage() {
	_, err := fmt.Fprintf(flag.CommandLine.Output(), "usage: %s [repoPath]\n", os.Args[0])
	if err != nil {
		log.Fatal(err)
	}
}

func parseRepoPath() (string, error) {
	narg := flag.NArg()
	if narg < 1 {
		return "", fmt.Errorf("no repoPath given")
	} else if narg > 1 {
		return "", fmt.Errorf("can only process one repoPath at a time, got %d: %s", narg, flag.Args())
	}

	repoPathArg := flag.Arg(0)

	// Ensure we have an absolute path to start with
	repoPath, err := filepath.Abs(repoPathArg)
	if err != nil {
		return "", fmt.Errorf("can't resolve repoPath path %s: %s", repoPathArg, err)
	}

	return repoPath, nil
}
