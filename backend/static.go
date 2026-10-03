package main

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// registerStatic 托管前端 SPA:/assets 等静态文件直接返回,
// 其余非 /api 路径回退到 index.html,交由前端路由处理。
func registerStatic(mux *http.ServeMux, dir string) {
	fs := http.FileServer(http.Dir(dir))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		p := filepath.Join(dir, filepath.Clean(r.URL.Path))
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			fs.ServeHTTP(w, r)
			return
		}
		// SPA 回退
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/"
		fs.ServeHTTP(w, r2)
	})
}
