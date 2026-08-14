// Package parser wraps the tree-sitter Python grammar and exposes a few
// convenience helpers for inspecting a parsed script.
package parser

import (
	"errors"
	"fmt"

	sitter "github.com/tree-sitter/go-tree-sitter"
	python "github.com/tree-sitter/tree-sitter-python/bindings/go"
)

// Language returns the tree-sitter Language for Python.
func Language() *sitter.Language {
	return sitter.NewLanguage(python.Language())
}

// Tree is a parsed Python source file. Close must be called to release the
// memory owned by the underlying C tree.
type Tree struct {
	tree   *sitter.Tree
	source []byte
}

// Parse builds a syntax tree for the given Python source.
func Parse(source []byte) (*Tree, error) {
	p := sitter.NewParser()
	defer p.Close()

	if err := p.SetLanguage(Language()); err != nil {
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
