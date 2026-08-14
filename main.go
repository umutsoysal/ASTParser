// Command astparser parses a Python file with tree-sitter and prints its
// syntax tree, definitions and imports.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/umutsoysal/ASTParser/parser"
)

func main() {
	sexp := flag.Bool("sexp", false, "print the full s-expression tree")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: astparser [-sexp] <file.py>\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}

	if err := run(flag.Arg(0), *sexp); err != nil {
		fmt.Fprintln(os.Stderr, "astparser:", err)
		os.Exit(1)
	}
}

func run(path string, sexp bool) error {
	source, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	tree, err := parser.Parse(source)
	if err != nil {
		return err
	}
	defer tree.Close()

	if sexp {
		fmt.Println(tree.SExpression())
		fmt.Println()
	}

	imports, err := tree.Imports()
	if err != nil {
		return err
	}
	fmt.Println("imports:")
	for _, m := range imports {
		fmt.Printf("  %s\n", m)
	}

	defs, err := tree.Definitions()
	if err != nil {
		return err
	}
	fmt.Println("definitions:")
	for _, d := range defs {
		fmt.Printf("  %s %s (line %d)\n", d.Kind, d.Name, d.Line)
	}

	if tree.HasError() {
		fmt.Println("\nnote: the file contains syntax errors")
	}
	return nil
}
