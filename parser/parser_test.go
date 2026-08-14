package parser_test

import (
	"os"
	"strings"
	"testing"

	"github.com/umutsoysal/ASTParser/parser"
)

func parseFile(t *testing.T, path string) *parser.Tree {
	t.Helper()

	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	tree, err := parser.Parse(source)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	t.Cleanup(tree.Close)

	return tree
}

func TestParseProducesModuleRoot(t *testing.T) {
	tree := parseFile(t, "../testdata/sample.py")

	if got := tree.Root().Kind(); got != "module" {
		t.Errorf("root kind = %q, want %q", got, "module")
	}
	if tree.HasError() {
		t.Errorf("valid file reported a syntax error:\n%s", tree.SExpression())
	}
}

func TestDefinitions(t *testing.T) {
	tree := parseFile(t, "../testdata/sample.py")

	defs, err := tree.Definitions()
	if err != nil {
		t.Fatalf("Definitions: %v", err)
	}

	want := []parser.Definition{
		{Name: "Greeter", Kind: "class", Line: 6},
		{Name: "__init__", Kind: "function", Line: 9},
		{Name: "greet", Kind: "function", Line: 12},
		{Name: "main", Kind: "function", Line: 16},
	}

	if len(defs) != len(want) {
		t.Fatalf("got %d definitions, want %d: %+v", len(defs), len(want), defs)
	}
	for i, w := range want {
		if defs[i] != w {
			t.Errorf("definition %d = %+v, want %+v", i, defs[i], w)
		}
	}
}

func TestImports(t *testing.T) {
	tree := parseFile(t, "../testdata/sample.py")

	imports, err := tree.Imports()
	if err != nil {
		t.Fatalf("Imports: %v", err)
	}

	want := []string{"os", "sys", "collections"}
	if len(imports) != len(want) {
		t.Fatalf("got %d imports, want %d: %v", len(imports), len(want), imports)
	}
	for i, w := range want {
		if imports[i] != w {
			t.Errorf("import %d = %q, want %q", i, imports[i], w)
		}
	}
}

func TestSExpressionShapeOfSimpleFunction(t *testing.T) {
	tree, err := parser.Parse([]byte("def f(x):\n    return x + 1\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	defer tree.Close()

	sexp := tree.SExpression()
	for _, want := range []string{"function_definition", "parameters", "return_statement", "binary_operator"} {
		if !strings.Contains(sexp, want) {
			t.Errorf("s-expression missing %q:\n%s", want, sexp)
		}
	}
}

func TestSyntaxErrorIsDetected(t *testing.T) {
	tree, err := parser.Parse([]byte("def broken(:\n    pass\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	defer tree.Close()

	if !tree.HasError() {
		t.Errorf("broken source did not report an error:\n%s", tree.SExpression())
	}
}

func TestParseEmptySource(t *testing.T) {
	tree, err := parser.Parse(nil)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	defer tree.Close()

	if got := tree.Root().NamedChildCount(); got != 0 {
		t.Errorf("empty source has %d named children, want 0", got)
	}
}
