package main

import (
	"container/list"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/denormal/go-gitignore"
)

const (
	codeownersFileName      = "CODEOWNERS"
	codeownersCommentPrefix = "#"
	generatedFileName       = ".github/CODEOWNERS"
	generatedFileWarning    = "# GENERATED FILE, DO NOT EDIT!"
)

// validateRoot takes a path and constructs a root from it. The path will be resolved to a clean,
// absolute path. If path doesn't represent a dir or can't be resolved for other reasons,
// an error is returned.
func validateRoot(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("error while resolving path %s: %w", path, err)
	}

	absPathInfo, err := os.Stat(absPath)
	if err != nil {
		return "", fmt.Errorf("error while reading resolved path %s: %w", absPath, err)
	}

	if !absPathInfo.IsDir() {
		return "", fmt.Errorf("resolved path %s is not a dir: %w", absPath, err)
	}

	return absPath, nil
}

// RewriteCodeownersRules visits every CODEOWNERS file under path (respecting .gitignore files
// and rewrites its rules for inclusion in the root CO file.
func RewriteCodeownersRules(path string) ([]string, error) {
	root, err := validateRoot(path)
	if err != nil {
		return nil, fmt.Errorf("error while validating path %s: %w", path, err)
	}

	var rewrittenRules []string

	err = walkCodeownersFiles(root, func(coPath string) error {
		rules, procErr := processCodeownersFile(root, coPath)
		if procErr != nil {
			return procErr
		}

		rewrittenRules = append(rewrittenRules, rules...)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("error while processing CODEOWNERS files: %w", err)
	}

	return rewrittenRules, nil
}

// procFn gets the path to a CODEOWNERS file and processes it.
type procFn = func(coPath string) error

// walkCodeownersFiles walks visits every CODEOWNERS file under root and calls
// procFn with the files absolute path as argument.
func walkCodeownersFiles(root string, procFn procFn) error {
	ignore := initGitignore(root)

	dirQueue := newStringQueue()
	dirQueue.Enqueue(root)

	for dirQueue.Len() > 0 {
		currentDir := dirQueue.Dequeue()

		if shouldIgnoreDir(ignore, currentDir) {
			continue
		}

		dir, err := os.Open(currentDir)
		if err != nil {
			return fmt.Errorf("error while opening dir %s: %w", currentDir, err)
		}

		dirEntries, err := dir.ReadDir(-1)
		dir.Close()
		if err != nil {
			return fmt.Errorf("error while reading dir %s: %w", currentDir, err)
		}

		// Ensure lexicographic order
		sort.Slice(dirEntries, func(i, j int) bool { return dirEntries[i].Name() < dirEntries[j].Name() })

		for _, dirEntry := range dirEntries {
			if isCodeownersFile(dirEntry) {
				path := filepath.Join(currentDir, dirEntry.Name())

				// Skip the target file
				if strings.HasSuffix(path, generatedFileName) {
					continue
				}

				err = procFn(path)
				if err != nil {
					return err
				}
			} else if dirEntry.IsDir() {
				dirEntryPath := filepath.Join(currentDir, dirEntry.Name())
				dirQueue.Enqueue(dirEntryPath)
			}
		}
	}

	return nil
}

// initGitignore parses the .gitignore files under root, including nested ones.
// If none are found or parsing errors, nil is returned.
func initGitignore(root string) gitignore.GitIgnore {
	ignore, _ := gitignore.NewRepository(root) // Ignore errors as ignore is an optional feature

	return ignore
}

// shouldIgnoreDir tests whether a dir should be ignored.
func shouldIgnoreDir(ignore gitignore.GitIgnore, path string) bool {
	if filepath.Base(path) == ".git" {
		return true
	}

	if ignore == nil || ignore.Base() == path { // Don't ignore the root itself
		return false
	}

	match := ignore.Match(path)
	if match != nil {
		return match.Ignore()
	}

	return false
}

// isCodeownersFile checks whether a direntry is a CODEOWNERS file.
func isCodeownersFile(d fs.DirEntry) bool {
	return !d.IsDir() && d.Name() == codeownersFileName
}

// processCodeownersFile reads and rewrites the codeowner rules.
func processCodeownersFile(root, path string) ([]string, error) {
	lines, err := readCodeownersFile(path)
	if err != nil {
		return nil, err
	}

	rewrittenPath, err := rewriteCodeownersPath(root, path)
	if err != nil {
		return nil, err
	}

	var rewrittenRules []string
	for _, line := range lines {
		if isCodeownersRule(line) {
			rewritten, err := rewriteCodeownersRule(rewrittenPath, line)
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
		return nil, fmt.Errorf("can't open CODEOWNERS file %s: %w", path, err)
	}
	defer file.Close()

	bytes, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("can't read CODEOWNERS file %s: %w", path, err)
	}

	content := string(bytes)
	return strings.Split(content, "\n"), nil
}

// isCodeownersRule decides whether a line from a CO file should be processed.
// False for whitespace and comment lines.
func isCodeownersRule(line string) bool {
	isWhitespace := strings.TrimSpace(line) == ""
	isComment := strings.HasPrefix(line, codeownersCommentPrefix)
	return !isWhitespace && !isComment
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

// rewriteCodeownersRule rewrites a valid CO rule for inclusion in the root CO file.
// It performs these transformations:
//   - Directory ownership rules (i.e. just a GitHub user, group or email) are prepended
//     with the CO files path absolute to the root: "@org/user" becomes
//     "/path/to/dir @org/user"
//   - File and glob ownership rules have the CO file path prepended to the file:
//     "main.go @org/user" becomes "/path/to/dir/main.go @org/user"
func rewriteCodeownersRule(rewrittenPath, rule string) (string, error) {
	if isDirRule(rule) {
		return rewriteDirRule(rewrittenPath, rule), nil
	} else {
		return rewriteNonDirRule(rewrittenPath, rule), nil
	}
}

// isDirRule checks whether a CO rule concerns a directory. This is the
// standard case, it is assumed when the first token of the rule contains an "@"
// (as codeowners can only be GitHub groups or users or email addresses).
func isDirRule(rule string) bool {
	tokens := strings.SplitN(rule, " ", 2)
	return len(tokens) >= 1 && strings.Contains(tokens[0], "@")
}

func rewriteDirRule(path, rule string) string {
	// Edge case: If the path is "/.", i.e. we are processing a CO file in
	// root the path should be a glob according to the CODEOWNERS syntax
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

func GenerateCodeownersFile(rules []string) string {
	body := strings.Join(rules, "\n")
	return fmt.Sprintf("%s\n\n%s\n", generatedFileWarning, body)
}

// stringQueue is the queue for BFS traversal
type stringQueue interface {
	Enqueue(s string)
	Dequeue() string
	Len() int
}

type stringQueueImpl struct {
	*list.List
}

func (q *stringQueueImpl) Enqueue(s string) {
	q.PushBack(s)
}

func (q *stringQueueImpl) Dequeue() string {
	elem := q.Front()
	q.Remove(elem)
	return elem.Value.(string)
}

func newStringQueue() stringQueue {
	return &stringQueueImpl{list.New()}
}
