// Package web 内嵌前端构建产物（frontend/dist），实现单镜像部署。
package web

import "embed"

//go:embed all:dist
var DistFS embed.FS
