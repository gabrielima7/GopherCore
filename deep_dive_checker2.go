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
	missing := 0
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if path != "." && strings.HasPrefix(info.Name(), ".") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(info.Name(), ".go") || strings.HasSuffix(info.Name(), "_test.go") || info.Name() == "deep_dive_checker2.go" {
			return nil
		}

		f, _ := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if f == nil {
			return nil
		}

		for _, decl := range f.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.IsExported() {
				stmtCount := 0
				hasConcurrency := false

				ast.Inspect(fn.Body, func(n ast.Node) bool {
					switch node := n.(type) {
					case ast.Stmt:
                        if _, isBlock := node.(*ast.BlockStmt); !isBlock {
                            stmtCount++
                        }
					case *ast.GoStmt, *ast.SelectStmt, *ast.SendStmt, *ast.SelectClause:
						hasConcurrency = true
					}
					return true
				})

				if stmtCount > 15 || hasConcurrency {
					doc := ""
					if fn.Doc != nil {
						doc = fn.Doc.Text()
					}
					if !strings.Contains(doc, "Internal Logic Deep-Dive:") {
						hasInlineDeepDive := false
						for _, c := range f.Comments {
							if c.Pos() > fn.Pos() && c.End() < fn.End() {
								if strings.Contains(c.Text(), "Internal Logic Deep-Dive:") {
									hasInlineDeepDive = true
									break
								}
							}
						}

						if !hasInlineDeepDive {
							fmt.Printf("Needs deep dive: %s | Func: %s\n", path, fn.Name.Name)
							missing++
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
	fmt.Printf("Total complex functions missing Deep-Dive: %d\n", missing)
}
