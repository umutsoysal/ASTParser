# ASTParser

A small Go project that parses Python source with [tree-sitter](https://tree-sitter.github.io/)
via the official Go bindings.

- `parser/` — a thin wrapper around the tree-sitter Python grammar: parse a
  file, dump its s-expression, and run tree-sitter queries for definitions and
  imports.
- `main.go` — a CLI that prints what it found.
- `testdata/sample.py` — the Python script the tests parse.

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
go run . testdata/sample.py
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
go run . -sexp testdata/sample.py
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

The image bundles `testdata/sample.py`, so a bare `docker run` is a smoke test:

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
