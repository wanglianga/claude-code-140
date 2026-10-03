package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"pvplatform/internal/api"
	"pvplatform/internal/config"
	"pvplatform/internal/db"
	"pvplatform/internal/seed"
)

func main() {
	cfg := config.Load()
	ctx := context.Background()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("连接数据库失败: %v", err)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool); err != nil {
		log.Fatalf("数据库迁移失败: %v", err)
	}
	if cfg.SeedOnStart {
		if err := seed.Run(ctx, pool); err != nil {
			log.Fatalf("种子数据失败: %v", err)
		}
	}

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           api.NewServer(pool, cfg.JWTSecret).Router(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("光伏平台监听 %s", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP 服务失败: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Println("正在关闭服务…")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}
