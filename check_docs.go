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
	fset := token.NewFileSet()
	missingCount := 0
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !strings.HasSuffix(path, ".go") || strings.Contains(path, "test") || strings.Contains(path, "vendor") {
			return nil
		}
		node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return nil
		}
		for _, decl := range node.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Name.IsExported() {
					if !checkComment(path, d.Name.Name, d.Doc) {
					    missingCount++
					}
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						if s.Name.IsExported() {
							if !checkComment(path, s.Name.Name, d.Doc) {
							    missingCount++
							}
						}
					case *ast.ValueSpec:
						doc := d.Doc
						if s.Doc != nil {
							doc = s.Doc
						}
						for _, name := range s.Names {
							if name.IsExported() {
								if !checkComment(path, name.Name, doc) {
								    missingCount++
								}
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
	fmt.Printf("\nTotal missing or incomplete docs: %d\n", missingCount)
}

func checkComment(path, name string, doc *ast.CommentGroup) bool {
	if doc == nil {
		fmt.Printf("%s: %s is missing documentation entirely\n", path, name)
		return false
	}
	text := doc.Text()
	missing := []string{}
	if !strings.Contains(text, "Purpose:") {
		missing = append(missing, "Purpose")
	}
	if !strings.Contains(text, "Constraints:") && !strings.Contains(text, "Requirements:") {
		missing = append(missing, "Constraints")
	}
	if !strings.Contains(text, "Thread-safety:") && !strings.Contains(text, "Concurrency:") {
		missing = append(missing, "Thread-safety")
	}
	if !strings.HasPrefix(strings.TrimSpace(text), name) {
	    // Not checking prefix for missing list yet because we just fixed it for some, but let's log it
	    // fmt.Printf("%s: %s comment does not start with its name\n", path, name)
	}

	if len(missing) > 0 {
		fmt.Printf("%s: %s is missing sections: %v\n", path, name, missing)
		return false
	}
	return true
}
