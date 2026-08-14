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
enough; on Debian/Ubuntu, `build-essential`.

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
