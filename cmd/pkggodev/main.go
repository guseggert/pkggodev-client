package main

import (
	"fmt"
	"os"

	pkggodevclient "github.com/guseggert/pkggodev-client"
)

func main() {
	action := os.Args[1]
	switch action {
	case "imports":
		imports()
	default:
		fmt.Fprintf(os.Stderr, "unknown action '%s'", action)
		os.Exit(1)
	}
}

func imports() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "must provide at least one package\n")
		os.Exit(1)
	}
	pkgs := os.Args[2:]
	imports := []string{}
	importSet := map[string]bool{}
	for _, pkg := range pkgs {
		c := pkggodevclient.New()
		imps, err := c.Imports(pkg)
		if err != nil {
			panic(err)
		}
		for _, imp := range imps {
			if _, ok := importSet[imp]; !ok {
				imports = append(imports, imp)
				importSet[imp] = true
			}
		}
	}
	for _, imp := range imports {
		fmt.Println(imp)
	}
}
