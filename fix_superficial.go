package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/ioutil"
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

		content, _ := ioutil.ReadFile(path)
		strContent := string(content)
		modified := false

		for _, decl := range node.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Name.IsExported() && d.Doc != nil {
					text := d.Doc.Text()
					lines := strings.Split(text, "\n")
					if len(lines) > 0 {
						firstLine := strings.TrimSpace(lines[0])
						words := strings.Fields(firstLine)
						if len(words) >= 2 && words[0] == d.Name.Name {
							verb := words[1]
							if verb == strings.ToLower(d.Name.Name) + "s" {
								fmt.Printf("SUPERFICIAL DETECTED: %s %s -> %s\n", d.Name.Name, verb, path)
							}
						}
					}
				}
			}
		}
		if modified {
			_ = strContent
		}
		return nil
	})
}
