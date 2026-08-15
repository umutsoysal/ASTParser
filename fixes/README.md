# Fixes

Corrected version of `codegen_ast_extractor.go` from
`ansys/aali-flowkit`, at
`pkg/privatefunctions/discovery/codegen/codegen_ast_extractor.go`.

Written up in [issue #1](https://github.com/umutsoysal/ASTParser/issues/1).

`fixes/codegen/` is a real package in this module, so `go vet ./...` and
`go test ./...` cover it — the file in this repo is known to compile and pass
its tests rather than being an untested paste.

## 1. The segfault (`SIGSEGV` at `0x1100000000f`)

```go
pythonLanguage = (*sitter.Language)(raw)   // crashes
pythonLanguage = sitter.NewLanguage(raw)   // correct
```

`sitter.Language` is a Go wrapper struct, not the C type:

```go
type Language struct { Inner *C.TSLanguage }
func NewLanguage(ptr unsafe.Pointer) *Language { return &Language{Inner: (*C.TSLanguage)(ptr)} }
```

Casting reinterprets the C struct as the Go struct, so reading `lang.Inner`
reads the first eight bytes of

```c
struct TSLanguage { uint32_t abi_version; uint32_t symbol_count; ... }
```

as a pointer. For the Python grammar those fields are `15` and `272`, giving
exactly the reported address:

```
symbol_count 0x110 << 32 | abi_version 0xf  ==  0x1100000000f
```

It then segfaults inside `ts_language_abi_version` during `SetLanguage`.

This is platform-independent. It is not caused by Alpine, by `python:3.11-alpine`,
by static linking, or by missing C libraries — the grammar is compiled into the
binary and has no runtime dependency on CPython or on any external C library.
Verified failing and then passing on both `debian/glibc` and `alpine/musl`,
`linux/amd64`.

Note also that `recover()` does not catch a SIGSEGV raised during cgo
execution, so panic recovery would not have contained this.

## 2. The `MethodCalls` query never compiled

`NewQuery` returned the error `call_expression`. In the Python grammar the node
is `call`, and the attribute's field is `attribute:`. The names `call_expression`
and `attr:` come from the JavaScript and Rust grammars.

```
(call
    function: (attribute
        object: (_) @receiver
        attribute: (identifier) @method
    )
    arguments: (argument_list) @args
)
```

## 3. `ArgCount` counted the parentheses

`argument_list` includes `(` and `)` as children. Filtering only `,` overcounts
by two:

| Call | Before | After |
|---|---|---|
| `Create(25, "Celsius")` | 4 | 2 |
| `GetByLabel("Heat")` | 3 | 1 |
| `reset()` | 2 | 0 |

`argsNode.NamedChildCount()` skips anonymous punctuation nodes and is correct.

## 4. Nil dereference on `argsNode`

`receiver` and `method` were guarded, then `argsNode.StartPosition()` was called
unconditionally. `argsNode` is now checked in the same condition.

## Also changed

`langErr` was assigned but never surfaced — a nil language reported
"not initialized" and discarded the real reason. Added `LanguageErr()`, which
`Parse` now wraps.
