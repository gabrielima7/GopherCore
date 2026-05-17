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
	count := 0
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") || strings.Contains(path, "vendor") {
			return nil
		}
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return err
		}

		for _, decl := range node.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Name.IsExported() {
					count++
					check(path, d.Name.Name, d.Doc, d)
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						if s.Name.IsExported() {
							count++
							doc := s.Doc
							if doc == nil && d.Doc != nil {
								doc = d.Doc
							}
							check(path, s.Name.Name, doc, d)
						}
					case *ast.ValueSpec:
						for _, name := range s.Names {
							if name.IsExported() {
								count++
								doc := s.Doc
								if doc == nil && d.Doc != nil {
									doc = d.Doc
								}
								// As per memory: comments for variables/constants defined in grouped blocks
								// are attached to the individual *ast.ValueSpec node (s.Doc), not parent.
								// But here we use s.Doc and fallback to d.Doc. This is fine to just see them.
								check(path, name.Name, doc, d)
							}
						}
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		fmt.Println(err)
	}
	fmt.Printf("Checked %d exported symbols\n", count)
}

func check(path, name string, doc *ast.CommentGroup, node ast.Node) {
	if doc == nil {
		fmt.Printf("MISSING DOC: %s:%s\n", path, name)
		return
	}
}
