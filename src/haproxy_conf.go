// Code generated by go generate; DO NOT EDIT.
// 2020-07-23 04:34:11.830063 -0700 PDT m=+0.000539196
package main

import "text/template"

//go:generate go run gen.go

var haproxyConf *template.Template = template.Must(template.New("").Funcs(template.FuncMap{
	"add": func(a, b int) int { return a + b },
	"subtract": func(a, b int) int { return a - b },
}).Parse(`global
  log stdout format raw local0
  stats socket {{ .HaproxyAdminSocket }} mode 660 level admin
  nbproc 1
  nbthread {{ .HaproxyThreads }}
  maxconn {{ .HaproxyMaxconn }}

defaults
  mode http
  retries 3
  timeout connect {{ .HaproxyTimeoutConnect }}
  timeout client {{ .HaproxyTimeoutClient }}
  timeout server {{ .HaproxyTimeoutServer }}
  timeout check {{ .HaproxyTimeoutCheck }}
{{ if .HaproxyEnableLogs }}
  log global
  option httplog
{{ end }}
  option srvtcpka

listen upstream
  bind {{ .HaproxyBind }}
  hash-type consistent
  balance {{ .HaproxyBalance }}
  option httpchk
  http-check send {{ .HaproxyHttpCheck }}
  server-template be 0-{{ subtract .HaproxySlots 1 }} 127.0.0.1:1 init-addr none check disabled

{{ if ne .HaproxyStatsBind "" }}
listen stats
  bind {{ .HaproxyStatsBind }}
  stats enable
  stats refresh 30s
  stats show-node
  stats uri /
{{ end }}

{{ if ne .HaproxyHealthBind "" }}
listen health
  bind {{ .HaproxyHealthBind }}
  monitor-uri /
{{ end }}
`))