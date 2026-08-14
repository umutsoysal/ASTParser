# ASTParser

A small Go project that parses Python source with [tree-sitter](https://tree-sitter.github.io/)
via the official Go bindings.

- `parser/` — a thin wrapper around the tree-sitter Python grammar: parse a
  file, dump its s-expression, and run tree-sitter queries for definitions and
  imports.
- `main.go` — a CLI that prints what it found.
- `parser/testdata/sample.py` — the Python script the tests parse.


Note to the reader: Just a quick tool for my task, if there is any interest, we can flesh this out quickly. cheers. 

## Requirements

Go 1.24+ and a C compiler — the bindings use cgo and compile the grammar's
`parser.c` as part of `go build`. On macOS the Xcode command line tools are
enough; on Debian/Ubuntu, `build-essential`. On Windows you need a MinGW-w64
gcc on `PATH` (from MSYS2, w64devkit, or TDM-GCC), since cgo cannot use MSVC —
or just use the Docker route below and skip the toolchain entirely.

## Run the tests

```bash
go test ./... -v
```

## Run the CLI

```bash
go run . parser/testdata/sample.py
```

```
imports:
  os
  sys
  collections
definitions:
  class Greeter (line 6)
  function __init__ (line 9)
  function greet (line 12)
  function main (line 16)
```

Add `-sexp` to also print the full syntax tree:

```bash
go run . -sexp parser/testdata/sample.py
```

## Docker

No local Go or C toolchain needed. Run the tests:

```bash
docker build --target test --progress=plain .
```

Build and run the CLI:

```bash
docker build -t astparser .
```

The image bundles `parser/testdata/sample.py`, so a bare `docker run` is a smoke test:

```bash
docker run --rm astparser
```

To parse your own files, mount the directory containing them at `/work` (the
image's working directory) and pass a path relative to it:

```bash
docker run --rm -v "$PWD:/work:ro" astparser myscript.py
```

### Windows hosts

Docker Desktop with the WSL2 backend runs these Linux images unchanged, and on
a typical amd64 Windows box `linux/amd64` is the native platform, so builds are
faster there than the emulated build on an Apple Silicon Mac.

Only the shell quoting for volume mounts differs:

```powershell
docker run --rm -v "${PWD}:/work:ro" astparser myscript.py   # PowerShell
```

```bat
docker run --rm -v "%cd%:/work:ro" astparser myscript.py
```

CRLF line endings from a Windows checkout are harmless — both the Go tests and
this Dockerfile build correctly with them.

The build does require BuildKit (for the `--mount=type=cache` lines); it is the
default in Docker 23+, so this only matters if you have explicitly set
`DOCKER_BUILDKIT=0`.

### A note on architecture

Because the bindings are cgo, the grammar is compiled for the target platform —
you cannot cross-compile with `GOOS`/`GOARCH` alone. Docker handles it, but an
emulated build is slower (on an M1, `linux/amd64` takes ~48s against ~10s
native). Build for a non-host platform explicitly:

```bash
docker build --platform linux/amd64 -t astparser:amd64 .
```

## Developing on Windows (VS Code)

`go get` and `go mod tidy` work fine in PowerShell — they only read and write
`go.mod`/`go.sum` and never invoke a compiler. But `go build`, `go test`, and
**gopls** do compile, so on a Windows host without MinGW the VS Code Go
extension reports errors on the tree-sitter imports even though CI is green.

Rather than installing MinGW for code that only ever ships to Linux, open the
repo in the Dev Container (`.devcontainer/devcontainer.json`):

> **Dev Containers: Reopen in Container** from the VS Code command palette.

gopls then runs inside Debian with gcc available and the editor agrees with the
build.

## Adding this to a Linux microservice

The parser is cgo, which has one consequence that breaks most Go service
Dockerfiles: **`CGO_ENABLED=0` will no longer build.** The usual
static-binary-for-`scratch` recipe fails as soon as tree-sitter is in the
import graph.

You do not have to give up the static binary. Alpine plus musl static linking
produces a ~4MB `scratch` image:

```dockerfile
FROM golang:1.24-alpine AS build
RUN apk add --no-cache build-base        # cgo needs a C compiler
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ENV CGO_ENABLED=1                        # was 0
RUN go build -trimpath \
      -ldflags='-linkmode external -extldflags "-static"' \
      -o /out/app .

FROM scratch
COPY --from=build /out/app /app
ENTRYPOINT ["/app"]
```

On a Debian builder (`golang:1.24-bookworm`) gcc is already present, but the
resulting binary links glibc dynamically — so the final stage needs a matching
glibc base such as `debian:bookworm-slim`, not `scratch`.

### Porting the package by copying it in

If you would rather not deal with private-module auth, copy the package
directly. `parser/` is self-contained — its fixture lives in
`parser/testdata/`, so the whole directory moves as one unit:

```powershell
Copy-Item -Recurse ..\ASTParser\parser .\internal\pyparser
```

Then fix the one repo-specific line. `parser_test.go` imports the package by
its old module path, so point it at yours:

```go
// before
import "github.com/umutsoysal/ASTParser/parser"

// after
import "github.com/your-org/your-service/internal/pyparser"
```

The package name inside the files stays `parser` regardless of the directory
name, so either rename the `package parser` declarations to match your
directory or import it with an alias:

```go
import parser "github.com/your-org/your-service/internal/pyparser"
```

`parser.go` itself needs no edits — it imports only tree-sitter, nothing from
this repo.

Add the dependencies **before** running `go mod tidy`, using the *module*
paths:

```bash
go get github.com/tree-sitter/go-tree-sitter@v0.25.0
go get github.com/tree-sitter/tree-sitter-python@v0.25.0
```

Order matters here. Running `go mod tidy` first, on a module that does not yet
require these, makes it try to resolve the *package* path
`tree-sitter-python/bindings/go` as if it were a module, and it fails with:

```
module declares its path as: github.com/tree-sitter/tree-sitter-python
        but was required as: github.com/tree-sitter/tree-sitter-python/bindings/go
```

That error reads like a missing dependency or an auth failure, but it is only
the ordering. `go get` the two module paths, then tidy, then:

```bash
go test ./internal/pyparser/...
```

### Private module access

This repository is private, so `go mod download` inside a Docker build cannot
fetch it without credentials. Set `GOPRIVATE` and pass a token as a BuildKit
secret rather than baking it into a layer:

```dockerfile
ENV GOPRIVATE=github.com/umutsoysal/*
RUN --mount=type=secret,id=ghtoken \
    git config --global url."https://$(cat /run/secrets/ghtoken)@github.com/".insteadOf "https://github.com/" && \
    go mod download && \
    git config --global --unset url."https://$(cat /run/secrets/ghtoken)@github.com/".insteadOf
```

```bash
docker build --secret id=ghtoken,env=GITHUB_TOKEN .
```

## Using the package

```go
tree, err := parser.Parse(source)
if err != nil {
    return err
}
defer tree.Close()

fmt.Println(tree.SExpression())
fmt.Println(tree.HasError())

defs, _ := tree.Definitions() // []parser.Definition{Name, Kind, Line}
mods, _ := tree.Imports()     // []string
```

Note that tree-sitter is error-tolerant: it always returns a tree, even for
invalid Python. Check `tree.HasError()` rather than expecting `Parse` to fail.
