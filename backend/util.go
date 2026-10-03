package main

import (
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"strings"
)

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeOK(w http.ResponseWriter, v interface{}) { writeJSON(w, 200, v) }

func fail(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func badRequest(w http.ResponseWriter, err error) { fail(w, 400, err.Error()) }

func serverErr(w http.ResponseWriter, err error) {
	// 演示项目直接透出错误，便于排查
	fail(w, 500, err.Error())
}

func decode(r *http.Request, v interface{}) error {
	dec := json.NewDecoder(r.Body)
	return dec.Decode(v)
}

func round2(x float64) float64 { return math.Round(x*100) / 100 }

func atoi64(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}

// ---- 轻量路由(Go 1.19 的 ServeMux 不支持路径参数) ----
type ctxKey string

type router struct {
	routes []rt
}
type rt struct {
	method  string
	parts   []string // 静态段或 {xxx}
	handler func(http.ResponseWriter, *http.Request, map[string]string)
}

func (ro *router) Handle(method, pattern string, h func(http.ResponseWriter, *http.Request, map[string]string)) {
	ro.routes = append(ro.routes, rt{method, strings.Split(strings.Trim(pattern, "/"), "/"), h})
}

func (ro *router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == "OPTIONS" {
		w.WriteHeader(204)
		return
	}
	segs := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	for _, rt := range ro.routes {
		if rt.method != r.Method || len(rt.parts) != len(segs) {
			continue
		}
		m := map[string]string{}
		ok := true
		for i, p := range rt.parts {
			if strings.HasPrefix(p, "{") && strings.HasSuffix(p, "}") {
				m[p[1:len(p)-1]] = segs[i]
			} else if p != segs[i] {
				ok = false
				break
			}
		}
		if ok {
			rt.handler(w, r, m)
			return
		}
	}
	fail(w, 404, "接口不存在: "+r.Method+" "+r.URL.Path)
}
