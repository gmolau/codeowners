# Codeowners

Tool to generate a [GitHub CODEOWNERS file](https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/customizing-your-repository/about-code-owners) from multiple CODEOWNERS files throughout the repo. This makes it easier to manage code ownership in large repos and thereby reduces the number of irrelevant review requests and blocked PRs.

## Example

By default, GitHub expects one `CODEOWNERS` file in the repos `.github` dir like this:

```
# File: .github/CODEOWNERS

* @org/admin-user
src/go @org/go-developer
src/go/lib.go @org/lib-specialist
```
This file tends to get messy and outdated in large repos with many contributors, leading to lots of unnecessary approval requests in pull requests.

With this tool these files can instead be placed into the directories to which they refer:

```
# File: CODEOWNERS
# Root CODEOWNERS file that sets the default owner of everything in this repo

@org/admin-user  # No glob required here, target is taken from the location of this CODEOWNERS file
```

```
# File: src/go/CODEOWNERS
# Tiny nested file that sets the owner of everything under src/go

@org/go-developer
lib.go @org/lib-specialist
```

Note that in the second file, ownership for an individual file can still be assigned as expected. Patterns like `*.go` can be used as well, though they should only refer to the subdirectory they are located in.

`codeowners` has to be invoked with the path to the repo, i.e. `codeowners path/to/repo`. It will traverse all CODEOWNERS files within it (but respecting `.gitignore` files) and print the correct root CODEOWNERS file for GitHub to stdout. The `.github/CODEOWNERS` file itself is not modified, to overwrite it use `codeowners path/to/repo > path/to/repo/.github/CODEOWNERS`.

## Installation

Install as a Go tool via `go get github.com/gmolau/codeowners`.

## Use as GitHub Action

For maximum convenience it is recommended to run this tool automatically in a GitHub Action like this:

```yaml
# File: .github/workflows/CODEOWNERS.yaml

name: Update CODEOWNERS
on:
  pull_request:
    paths:
      - "**/CODEOWNERS"        # Trigger for every CODEOWNERS file in the repo
      - "!.github/CODEOWNERS"  # except for the generated file itself
jobs:
  update-codeowners:
    runs-on: ubuntu-20.04
    name: Update CODEOWNERS
    steps:
      - name: Checkout repo
        uses: actions/checkout@v2
      - name: Update CODEOWNERS file
        uses: gmolau/codeowners@v0.1.1
    - name: Commit CODEOWNERS file
        uses: EndBug/add-and-commit@v7
        with:
          message: Update CODEOWNERS file (CODEOWNERS Bot)
```

This workflow runs on every change to a `CODEOWNERS` file and regenerates and commits the root CODEOWNERS file.
