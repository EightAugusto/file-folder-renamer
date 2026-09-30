package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

func TestEntrypointIsUIOnly(t *testing.T) {
	data, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "--version") || strings.Contains(string(data), "flag.") || strings.Contains(string(data), "os.Args") {
		t.Fatal("desktop entrypoint contains command-line behavior")
	}
	file, err := parser.ParseFile(token.NewFileSet(), "main.go", data, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Imports) != 1 || !strings.Contains(file.Imports[0].Path.Value, "/internal/ui") {
		t.Fatalf("entrypoint imports must contain only the UI composition root: %v", file.Imports)
	}
	mainFunctions := 0
	ast.Inspect(file, func(node ast.Node) bool {
		if function, ok := node.(*ast.FuncDecl); ok && function.Name.Name == "main" {
			mainFunctions++
		}
		return true
	})
	if mainFunctions != 1 {
		t.Fatalf("got %d main functions, want 1", mainFunctions)
	}
}

func TestMainStartsUI(t *testing.T) {
	previous := run
	t.Cleanup(func() { run = previous })
	called := false
	run = func() { called = true }
	main()
	if !called {
		t.Fatal("main did not start the UI")
	}
}
