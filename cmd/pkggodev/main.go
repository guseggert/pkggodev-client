package main

import (
	"encoding/json"
	"fmt"
	"os"

	pkggodevclient "github.com/guseggert/pkggodev-client"
)

var commands = map[string]func(){
	"imported-by":  importedBy,
	"package-info": packageInfo,
	"versions":     versions,
	"search":       search,
}

func main() {
	cmd := os.Args[1]
	cmdFunc, ok := commands[cmd]
	if !ok {
		complainAndDie(fmt.Sprintf("unknown command '%s'", cmd))
	}
	cmdFunc()
}

func search() {
	if len(os.Args) != 3 {
		complainAndDie("usage: pkggodev search <query>")
	}
	query := os.Args[2]
	c := pkggodevclient.New()
	res, err := c.Search(query)
	if err != nil {
		complainAndDie(err)
	}
	b, err := json.Marshal(res)
	if err != nil {
		complainAndDie(err)
	}
	fmt.Println(string(b))
}

func versions() {
	if len(os.Args) < 3 {
		complainAndDie("must provide at least one package")
	}
	pkgs := os.Args[2:]
	c := pkggodevclient.New()
	for _, pkg := range pkgs {
		versions, err := c.Versions(pkg)
		if err != nil {
			complainAndDie(err)
		}
		b, err := json.Marshal(versions)
		if err != nil {
			complainAndDie(err)
		}
		fmt.Println(string(b))
	}
}

func importedBy() {
	if len(os.Args) < 3 {
		complainAndDie("must provide at least one package")
	}
	pkgs := os.Args[2:]
	c := pkggodevclient.New()
	for _, pkg := range pkgs {
		importedBy, err := c.ImportedBy(pkg)
		if err != nil {
			complainAndDie(err)
		}
		b, err := json.Marshal(importedBy)
		if err != nil {
			complainAndDie(err)
		}
		fmt.Println(string(b))
	}
}

func packageInfo() {
	if len(os.Args) < 3 {
		complainAndDie("must provide at least one package")
	}
	pkgs := os.Args[2:]
	c := pkggodevclient.New()
	for _, pkg := range pkgs {
		d, err := c.DescribePackage(pkg)
		if err != nil {
			complainAndDie(err)
		}
		b, err := json.Marshal(d)
		if err != nil {
			complainAndDie(err)
		}
		fmt.Println(string(b))
	}
}

func complainAndDie(v interface{}) {
	fmt.Fprintf(os.Stderr, "%v\n", v)
	os.Exit(1)
}
