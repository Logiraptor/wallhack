package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/doc"
	"go/parser"
	"go/token"
	ht "html/template"
	"io"
	"log"
	"os"
	"os/exec"
	"text/template"
)

func main() {
	var htmlOut = flag.String("html", "", "generated documentation output")
	flag.Parse()
	if len(os.Args) < 2 {
		log.Fatal("Usage: wallhack PACKAGE")
		flag.Usage()
		return
	}

	pkgPath := os.Args[len(os.Args)-1]

	gopath := os.Getenv("GOPATH")
	fullPath := gopath + "/src/" + pkgPath

	fs := token.NewFileSet()

	pkgs, err := parser.ParseDir(fs, fullPath, nil, parser.ParseComments)
	if err != nil {
		log.Fatal(err.Error())
		return
	}

	var endpointMap = map[string]interface{}{}
	for pkgName, pkg := range pkgs {
		d := doc.New(pkg, "root/"+pkgName, 0)
		funcDocs := map[string]string{}
		for _, f := range d.Funcs {
			funcDocs[f.Name] = f.Doc
		}
		for _, t := range d.Vars {
			for i, name := range t.Names {
				spec := t.Decl.Specs[i]
				if v, ok := spec.(*ast.ValueSpec); ok {
					for i, val := range v.Values {
						if _, ok := isWallhackRouter(val); ok {
							endpoints, err := readSpec(pkgPath, v.Names[i].Name, pkg.Name)
							if err != nil {
								log.Fatal("could not run inspector", err.Error())
								return
							}
							for i := range endpoints {
								endpoints[i].Doc = funcDocs[endpoints[i].Func]
							}
							endpointMap[name] = map[string]interface{}{
								"Endpoints": endpoints,
								"Doc":       t.Doc,
							}
						}
					}
				}
			}
		}
	}

	if *htmlOut != "" {
		htmlTmpl, err := ht.New("doc").Funcs(ht.FuncMap{
			"json": func(x interface{}) ht.HTML {
				buf, err := json.MarshalIndent(x, "    ", "\t")
				if err != nil {
					return ht.HTML(err.Error())
				}
				return ht.HTML(string(buf))
			},
		}).Parse(html)
		if err != nil {
			log.Fatal(err)
		}
		htmlOutput, err := os.OpenFile(*htmlOut, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0666)
		if err != nil {
			log.Fatal(err)
		}
		err = htmlTmpl.Execute(htmlOutput, endpointMap)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		json.NewEncoder(os.Stdout).Encode(endpointMap)
	}
}

type endpoint struct {
	Method   string
	URL      string
	Package  string
	Func     string
	Doc      string
	Response interface{}
}

func readSpec(imp, routeVar, pkg string) ([]endpoint, error) {
	tmpl, err := template.New("printer").Parse(printer)
	if err != nil {
		return nil, fmt.Errorf("template: %s", err.Error())
	}

	output, err := os.OpenFile("/tmp/tmp_.go", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0666)
	if err != nil {
		return nil, fmt.Errorf("openfile: %s", err.Error())
	}
	defer os.Remove("/tmp/tmp_.go")

	tmpl.Execute(output, map[string]string{
		"Import":  imp,
		"Package": pkg,
		"Var":     routeVar,
	})

	output.Close()

	goRun := exec.Command("go", "run", "/tmp/tmp_.go")
	out, err := goRun.StdoutPipe()
	if err != nil {
		return nil, err
	}
	errPipe, err := goRun.StderrPipe()
	if err != nil {
		return nil, err
	}
	go io.Copy(os.Stderr, errPipe)
	err = goRun.Start()
	if err != nil {
		return nil, fmt.Errorf("go run: %s\n", err.Error())
	}

	var endpoints []endpoint
	d := json.NewDecoder(out)
	d.UseNumber()
	err = d.Decode(&endpoints)
	if err != nil {
		return nil, fmt.Errorf("json error: %s", err.Error())
	}

	return endpoints, nil
}

func isWallhackRouter(exp ast.Expr) (*ast.CompositeLit, bool) {
	if lit, ok := exp.(*ast.CompositeLit); ok {
		if sel, ok := lit.Type.(*ast.SelectorExpr); ok {
			if sel.Sel.Name == "Router" {
				if ident, ok := sel.X.(*ast.Ident); ok {
					if ident.Name == "router" {
						return lit, true
					}
				}
			}
		}
	}
	return nil, false
}

var printer = `
package main

import (
	"encoding/json"
	"os"
	"reflect"
	"runtime"
	"strings"

	"github.com/Logiraptor/wallhack/router"

	"{{.Import}}"
)

type endpoint struct {
	Method   string
	URL      string
	Package  string
	Func     string
	Response interface{}
}

func main() {
	var endpoints = []endpoint{}
	for _, v := range {{.Package}}.{{.Var}} {
		val := reflect.ValueOf(v.H.(router.Wrapper).F)
		fName := runtime.FuncForPC(val.Pointer()).Name()
		nameStart := strings.LastIndex(fName, ".")
		pkgName := fName[:nameStart]
		fName = fName[nameStart+1:]

		response := val.Type().Out(0)
		for response.Kind() == reflect.Ptr {
			response = response.Elem()
		}
		responseInstance := reflect.New(response).Elem()
		os.Stderr.Write([]byte(response.String()))
		respInt := responseInstance.Interface()
		if ex, ok := respInt.(router.Exampler); ok {
			respInt = ex.Example(v.M, v.U, fName)
		}

		endpoints = append(endpoints, endpoint{
			Method:   v.M,
			URL:      v.U,
			Package:  pkgName,
			Func:     fName,
			Response: respInt,
		})
	}

	json.NewEncoder(os.Stdout).Encode(endpoints)
}

`
