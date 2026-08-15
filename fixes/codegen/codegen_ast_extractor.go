// Copyright (C) 2025 - 2026 ANSYS, Inc. and/or its affiliates.
// SPDX-License-Identifier: MIT
//
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package codegen

import (
	"errors"
	"fmt"
	"sync"

	sitter "github.com/tree-sitter/go-tree-sitter"
	python "github.com/tree-sitter/tree-sitter-python/bindings/go"
)

var (
	pythonLanguage *sitter.Language
	langOnce       sync.Once
	langErr        error
)

func initLanguage() {
	langOnce.Do(func() {
		raw := python.Language()
		if raw == nil {
			langErr = errors.New("python.Language() returned nil")
			return
		}
		// python.Language() returns a raw *C.TSLanguage as unsafe.Pointer.
		// sitter.Language is a Go wrapper struct (struct{ Inner *C.TSLanguage }),
		// NOT the C type, so it must be constructed with NewLanguage.
		//
		// Casting directly -- (*sitter.Language)(raw) -- reinterprets the C
		// struct as the Go struct, so reading lang.Inner reads the first eight
		// bytes of struct TSLanguage { uint32_t abi_version; uint32_t
		// symbol_count; ... } as a pointer. That yields the address
		// 0x1100000000f (symbol_count 0x110, abi_version 0xf) and segfaults
		// inside ts_language_abi_version during SetLanguage.
		pythonLanguage = sitter.NewLanguage(raw)
	})
}

// Language returns the tree-sitter Language for Python, or nil if
// initialization failed. Callers that need the reason should use LanguageErr.
func Language() *sitter.Language {
	initLanguage()
	return pythonLanguage
}

// LanguageErr reports why the Python language failed to initialize, if it did.
func LanguageErr() error {
	initLanguage()
	return langErr
}

// Tree is a parsed Python source file. Close must be called to release the
// memory owned by the underlying C tree.
type Tree struct {
	tree   *sitter.Tree
	source []byte
}

func Parse(source []byte) (*Tree, error) {
	lang := Language()
	if lang == nil {
		if err := LanguageErr(); err != nil {
			return nil, fmt.Errorf("python language not initialized: %w", err)
		}
		return nil, errors.New("python language not initialized")
	}

	p := sitter.NewParser()
	defer p.Close()

	if err := p.SetLanguage(lang); err != nil {
		return nil, fmt.Errorf("set language: %w", err)
	}

	tree := p.Parse(source, nil)
	if tree == nil {
		return nil, errors.New("parse returned no tree")
	}

	return &Tree{tree: tree, source: source}, nil
}

// Close releases the syntax tree.
func (t *Tree) Close() { t.tree.Close() }

// Root returns the root node of the tree.
//
// The returned node points into memory owned by the tree. Do not use it, or
// anything derived from it, after Close.
func (t *Tree) Root() *sitter.Node { return t.tree.RootNode() }

// SExpression returns the whole tree in tree-sitter's s-expression form, which
// is handy for eyeballing structure and for golden tests.
func (t *Tree) SExpression() string { return t.Root().ToSexp() }

// HasError reports whether the parse contained any ERROR or MISSING nodes.
// Tree-sitter always produces a tree, so this is how you detect bad syntax.
func (t *Tree) HasError() bool { return t.Root().HasError() }

// Definition is a named top-level-or-nested definition found in the source.
type Definition struct {
	Name string
	Kind string // "function" or "class"
	Line uint   // 1-based line number of the definition keyword
}

// Definitions returns every function and class definition in the file, in the
// order they appear.
func (t *Tree) Definitions() ([]Definition, error) {
	const q = `
	(function_definition name: (identifier) @function)
	(class_definition    name: (identifier) @class)
	`

	query, qErr := sitter.NewQuery(Language(), q)
	if qErr != nil {
		return nil, fmt.Errorf("compile query: %w", qErr)
	}
	defer query.Close()

	cursor := sitter.NewQueryCursor()
	defer cursor.Close()

	names := query.CaptureNames()
	root := t.Root()

	var defs []Definition
	matches := cursor.Matches(query, root, t.source)
	for m := matches.Next(); m != nil; m = matches.Next() {
		for _, c := range m.Captures {
			node := c.Node
			// The captured identifier is the name; its parent is the
			// definition itself, which is where the line number belongs.
			def := node.Parent()
			if def == nil {
				def = &node
			}
			defs = append(defs, Definition{
				Name: node.Utf8Text(t.source),
				Kind: names[c.Index],
				Line: def.StartPosition().Row + 1,
			})
		}
	}

	return defs, nil
}

// Imports returns the module names brought in by `import x` and `from x import
// y` statements.
func (t *Tree) Imports() ([]string, error) {
	const q = `
	(import_statement name: (dotted_name) @mod)
	(import_from_statement module_name: (dotted_name) @mod)
	`

	query, qErr := sitter.NewQuery(Language(), q)
	if qErr != nil {
		return nil, fmt.Errorf("compile query: %w", qErr)
	}
	defer query.Close()

	cursor := sitter.NewQueryCursor()
	defer cursor.Close()

	root := t.Root()

	var mods []string
	matches := cursor.Matches(query, root, t.source)
	for m := matches.Next(); m != nil; m = matches.Next() {
		for _, c := range m.Captures {
			mods = append(mods, c.Node.Utf8Text(t.source))
		}
	}

	return mods, nil
}

// Used Methods starts here.
//
// MethodCallInfo represents a method call extracted from the AST.
type MethodCallInfo struct {
	Receiver string // e.g., "Units.TemperatureQuantity"
	Method   string // e.g., "Create"
	ArgCount int    // Number of arguments in the call
	Line     uint   // 1-based line number
}

// MethodCalls returns all method calls found in the code using AST analysis.
//
// This is more reliable than regex-based extraction, especially for:
//   - Nested parentheses in arguments
//   - Distinguishing actual calls from string literals
//   - Complex multi-line expressions
//
// It queries the syntax tree for call nodes where the function is an attribute
// access (e.g., "obj.method()") and extracts:
//   - Receiver: the object name (e.g., "Units.TemperatureQuantity")
//   - Method: the method name (e.g., "Create")
//   - ArgCount: number of arguments passed
//   - Line: 1-based line number where the call appears
//
// Example:
//
//	code := `
//	temp = Units.TemperatureQuantity.Create(25, "Celsius")
//	result = Conditions.Heat.GetByLabel("Heat")
//	nested = obj.method(getValue(1, 2), unit)
//	`
//	tree, err := Parse([]byte(code))
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer tree.Close()
//
//	calls, err := tree.MethodCalls()
//	// calls[0] = {Receiver: "Units.TemperatureQuantity", Method: "Create",   ArgCount: 2, Line: 2}
//	// calls[1] = {Receiver: "Conditions.Heat",           Method: "GetByLabel", ArgCount: 1, Line: 3}
//	// calls[2] = {Receiver: "obj",                       Method: "method",   ArgCount: 2, Line: 4}
func (t *Tree) MethodCalls() ([]MethodCallInfo, error) {
	// In the Python grammar the node is `call` (not `call_expression`) and the
	// attribute's field is `attribute:` (not `attr:`). The names used by the
	// JavaScript and Rust grammars do not apply here, and a wrong name makes
	// NewQuery fail rather than silently return nothing.
	const q = `
	(call
	    function: (attribute
	        object: (_) @receiver
	        attribute: (identifier) @method
	    )
	    arguments: (argument_list) @args
	)
	`

	query, qErr := sitter.NewQuery(Language(), q)
	if qErr != nil {
		return nil, fmt.Errorf("compile method call query: %w", qErr)
	}
	defer query.Close()

	cursor := sitter.NewQueryCursor()
	defer cursor.Close()

	names := query.CaptureNames()
	root := t.Root()
	var calls []MethodCallInfo

	matches := cursor.Matches(query, root, t.source)
	for m := matches.Next(); m != nil; m = matches.Next() {
		var receiver, method string
		var argsNode *sitter.Node

		for _, c := range m.Captures {
			node := c.Node
			switch names[c.Index] {
			case "receiver":
				receiver = node.Utf8Text(t.source)
			case "method":
				method = node.Utf8Text(t.source)
			case "args":
				argsNode = &node
			}
		}

		if receiver == "" || method == "" || argsNode == nil {
			continue
		}

		// NamedChildCount skips the punctuation children -- "(", ")" and ","
		// are all anonymous nodes. Counting every non-comma child instead
		// includes both parentheses and overcounts by two.
		calls = append(calls, MethodCallInfo{
			Receiver: receiver,
			Method:   method,
			ArgCount: int(argsNode.NamedChildCount()),
			Line:     argsNode.StartPosition().Row + 1,
		})
	}

	return calls, nil
}
