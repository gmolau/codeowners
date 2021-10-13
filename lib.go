package main

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/denormal/go-gitignore"
)

const (
	codeownersFileName      = "CODEOWNERS"
	codeownersCommentPrefix = "#"
	generatedFileName       = ".github/CODEOWNERS"
	generatedFileWarning    = "# GENERATED FILE, DO NOT EDIT!"
)

// walkRepo visits every CODEOWNERS file under root and processed it for inclusion
// in the output file. Respects .gitignore files. Returns the processed CODEOWNERS
// rules in a slice.
func walkRepo(root string) ([]string, error) {
	var rewrittenRules []string

	ignore := initGitignore(root)
	generatedFilePath := filepath.Join(root, generatedFileName)

	err := filepath.WalkDir(root,
		func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return fmt.Errorf("error while visiting %s: %s", path, err)
			}

			if path == generatedFilePath { // Don't process the target file
				return nil
			}

			if d.IsDir() && shouldIgnoreDir(ignore, path) {
				return fs.SkipDir
			}

			if isCodeownersFile(d) {
				rules, procErr := processCodeownersFile(root, path)
				if procErr != nil {
					return procErr
				}

				rewrittenRules = append(rewrittenRules, rules...)
			}

			return nil
		})

	if err != nil {
		return rewrittenRules, fmt.Errorf("error while walking repo: %s", err)
	}

	return rewrittenRules, nil
}

// initGitignore parses the .gitignore files under root, including nested ones.
// If none are found or parsing errors, nil is returned.
func initGitignore(root string) gitignore.GitIgnore {
	ignore, _ := gitignore.NewRepository(root) // Ignore errors as ignore is an optional feature

	return ignore
}

// shouldIgnoreDir tests whether a dir should be ignored.
func shouldIgnoreDir(ignore gitignore.GitIgnore, path string) bool {
	if ignore == nil || ignore.Base() == path { // Don't ignore the root itself
		return false
	}

	match := ignore.Match(path)
	if match != nil {
		return match.Ignore()
	}

	return false
}

// isCodeownersFile check whether a direntry is a CODEOWNERS file.
func isCodeownersFile(d fs.DirEntry) bool {
	return !d.IsDir() && d.Name() == codeownersFileName
}

// processCodeownersFile reads and rewrites the codeowner rules.
func processCodeownersFile(root, path string) ([]string, error) {
	lines, err := readCodeownersFile(path)
	if err != nil {
		return nil, err
	}

	var rewrittenRules []string
	for _, line := range lines {
		if isCodeownerRule(line) {
			rewritten, err := rewriteCodeownerRule(root, path, line)
			if err != nil {
				return nil, err
			}

			if rewritten != "" {
				rewrittenRules = append(rewrittenRules, rewritten)
			}
		}
	}

	return rewrittenRules, nil
}

// readCodeownersFile reads a CO file line-wise into a slice of strings. If an
// error occurs, the returned error contains the file path and the error.
func readCodeownersFile(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("can't open CODEOWNERS file %s: %s", path, err)
	}
	defer file.Close()

	bytes, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("can't read CODEOWNERS file %s: %s", path, err)
	}

	content := string(bytes)
	return strings.Split(content, "\n"), nil
}

// isCodeownerRule decides whether a line from a CO file should be processed.
// It excludes whitespace and comment lines. It does not exclude wildcard patterns
// like "*.go" even though they will likely lead to nonsensical results when included
// in the root CO file.
func isCodeownerRule(line string) bool {
	trimmed := strings.TrimSpace(line)
	return trimmed != "" && !strings.HasPrefix(line, codeownersCommentPrefix)
}

// rewriteCodeownerRule rewrites a valid CO rule for inclusion in the root CO file.
// It performs these transformations:
//   - Directory ownership rules (i.e. just a GitHub user, group or email) are prepended
//     with the CO files path absolute to the root: "@org/user" becomes
//     "/path/to/dir @org/user"
//   - File and glob ownership rules have the CO file path prepended to the file:
//     "main.go @org/user" becomes "/path/to/dir/main.go @org/user"
func rewriteCodeownerRule(root, path, rule string) (string, error) {
	rewrittenPath, err := rewriteCodeownersPath(root, path)
	if err != nil {
		return "", err
	}

	if isDirRule(rule) {
		return rewriteDirRule(rewrittenPath, rule), nil
	} else {
		return rewriteNonDirRule(rewrittenPath, rule), nil
	}
}

// rewriteCodeownersPath takes the absolut path of a CO file and rewrites it
// for usage in the root CO file by taking its parent dir and making it absolute
// to the root.
func rewriteCodeownersPath(root, path string) (string, error) {
	// Get the dir of this CODEOWNERS file
	dir := filepath.Dir(path)

	// Make that dir relative to the root
	relDir, err := filepath.Rel(root, dir)
	if err != nil {
		return "", fmt.Errorf("can't rewrite CODEOWNERS path %s: %s", path, err)
	}

	// Make that path absolute to the root
	return fmt.Sprintf("/%s", relDir), nil
}

// isDirRule checks whether a CO rule concerns a directory. This is the
// standard case, it is assumed when the first token of the rule contains an "@"
// (as codeowners can only be GitHub groups or users or email addresses).
func isDirRule(rule string) bool {
	tokens := strings.SplitN(rule, " ", 2)
	return len(tokens) >= 1 && strings.Contains(tokens[0], "@")
}

func rewriteDirRule(path, rule string) string {
	// Edge case: If the path is "/.", i.e. we are processing a CO file in the
	// repo root the path should be a glob according to the CODEOWNERS syntax
	// https://docs.github.com/en/github/creating-cloning-and-archiving-repositories/creating-a-repository-on-github/about-code-owners#codeowners-syntax
	if path == "/." {
		path = "*"
	}

	return fmt.Sprintf("%s %s", path, rule)
}

func rewriteNonDirRule(path, rule string) string {
	tokens := strings.SplitN(rule, " ", 2)
	if len(tokens) < 2 {
		return ""
	}

	ruleTarget := strings.TrimSpace(tokens[0])
	rule = strings.TrimSpace(tokens[1])
	path = filepath.Join(path, ruleTarget)

	return fmt.Sprintf("%s %s", path, rule)
}

func generateCodeownersFile(rules []string) string {
	body := strings.Join(rules, "\n")
	return fmt.Sprintf("%s\n\n%s\n", generatedFileWarning, body)
}
