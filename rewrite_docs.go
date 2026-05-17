package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") || strings.Contains(path, "vendor") {
			return nil
		}
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil { return err }

		for _, decl := range node.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Name.IsExported() && d.Doc != nil {
					checkSuperficial(d.Name.Name, path, d.Doc.Text())
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						if s.Name.IsExported() {
							doc := s.Doc
							if doc == nil { doc = d.Doc }
							if doc != nil { checkSuperficial(s.Name.Name, path, doc.Text()) }
						}
					}
				}
			}
		}
		return nil
	})
}

func checkSuperficial(name, path, text string) {
	lines := strings.Split(text, "\n")
	if len(lines) > 0 {
		firstLine := strings.TrimSpace(lines[0])
		words := strings.Fields(firstLine)
		if len(words) >= 2 && words[0] == name {
			verb := words[1]
			if verb == strings.ToLower(name) + "s" {
				fmt.Printf("SUPERFICIAL: %s in %s -> %s\n", name, path, firstLine)
			}
		}
	}
}
