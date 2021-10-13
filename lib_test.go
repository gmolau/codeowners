package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestE2E(t *testing.T) {
	// Create a test env
	tmpdir, err := os.MkdirTemp("", "test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpdir)

	// Create a repo in the test env so that we have a predictable name
	repoPath := filepath.Join(tmpdir, "repo")
	err = os.Mkdir(repoPath, 0700)
	require.NoError(t, err)

	// Create an existing CODEOWNERS file that should not be processed
	existingCOFile := generatedFileWarning + `
/src/foobar @org/previousUser
`
	writeFile(t, repoPath, generatedFileName, existingCOFile)

	// Create a simple CODEOWNERS file for the happy path
	simpleCOFile := `
@org/user
`
	writeFile(t, repoPath, "src/dir1/CODEOWNERS", simpleCOFile)

	// Create a complex CODEOWNERS file with edge cases
	complexCOFile := `
# Dir rule
@org/user @singleUser email@server.com

# File rules
main.go @org/gopher
package/nested.go @org/nestedUser

# Glob rule (nonsensical but allowed)
*.js @org/frontend @fullstackUser
`
	writeFile(t, repoPath, "src/dir2/CODEOWNERS", complexCOFile)

	// Create a CODEOWNERS file in root which should be processed as well.
	// Also tests the global owner edge case (should be assigned by glob, not path).
	rootCOFile := `
# Default owner for the entire repo
@org/admin

go.mod @org/gopher
`
	writeFile(t, repoPath, "CODEOWNERS", rootCOFile)

	// Create a CODEOWNERS file in an ignored directory
	ignoreFile := `
/src/shouldBeIgnored
`
	ignoredCOFile := `
@org/shouldNotBeSeen
`
	writeFile(t, repoPath, ".gitignore", ignoreFile)
	writeFile(t, repoPath, "src/shouldBeIgnored/CODEOWNERS", ignoredCOFile)

	// Test rule rewriting
	expectedRules := []string{
		"* @org/admin",
		"/go.mod @org/gopher",
		"/src/dir1 @org/user",
		"/src/dir2 @org/user @singleUser email@server.com",
		"/src/dir2/main.go @org/gopher",
		"/src/dir2/package/nested.go @org/nestedUser",
		"/src/dir2/*.js @org/frontend @fullstackUser",
	}

	rewrittenRules, err := walkRepo(repoPath)
	require.NoError(t, err)
	require.Equal(t, expectedRules, rewrittenRules)

	// Test file generation
	expectedFile := generatedFileWarning + `

* @org/admin
/go.mod @org/gopher
/src/dir1 @org/user
/src/dir2 @org/user @singleUser email@server.com
/src/dir2/main.go @org/gopher
/src/dir2/package/nested.go @org/nestedUser
/src/dir2/*.js @org/frontend @fullstackUser
`

	generatedFile := generateCodeownersFile(rewrittenRules)
	require.Equal(t, expectedFile, generatedFile)
}

func writeFile(t *testing.T, root, path, content string) {
	// Construct the abspath to the file's dir first so that we can
	// create the parent dirs
	relDir := filepath.Dir(path)
	absDir := filepath.Join(root, relDir)
	err := os.MkdirAll(absDir, 0700)
	require.NoError(t, err)

	// Now create the file in that directory
	fileName := filepath.Base(path)
	file := filepath.Join(absDir, fileName)
	err = os.WriteFile(file, []byte(content), 0600)
	require.NoError(t, err)
}
