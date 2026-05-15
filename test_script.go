package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	wd, _ := os.Getwd()
	path := filepath.Join(wd, "testdata/migrations")
	path = filepath.ToSlash(path) // Replace \ with /
	if !strings.HasPrefix(path, "/") {
		path = "/" + path // Ensure it starts with / for absolute paths on Windows (e.g. /C:/foo)
	}

	fmt.Println("file://" + path)
}
