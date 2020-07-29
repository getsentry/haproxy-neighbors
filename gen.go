// +build ignore
package main

import (
	"io/ioutil"
	"log"
	"os"
	"text/template"
	"time"
)

func main() {
	fp, err := os.Create("src/haproxy_conf.go")
	if err != nil {
		log.Fatal(err)
	}
	defer fp.Close()

	body, err := ioutil.ReadFile("haproxy.cfg")
	if err != nil {
		log.Fatal(err)
	}

	tmpl.Execute(fp, struct {
		Timestamp time.Time
		Body      string
	}{
		Timestamp: time.Now(),
		Body:      string(body),
	})
}

var tmpl = template.Must(template.New("").Parse(`// Code generated by go generate; DO NOT EDIT.
// {{ .Timestamp }}
package main

import "text/template"

//go:generate go run gen.go

var haproxyConf *template.Template = template.Must(template.New("").Funcs(template.FuncMap{
	"add": func(a, b int) int { return a + b },
	"subtract": func(a, b int) int { return a - b },
}).Parse(` + "`" + `{{ .Body }}` + "`))"))