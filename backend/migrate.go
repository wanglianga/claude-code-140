package main

import (
	"database/sql"
	"embed"
	"fmt"
	"log"
	"os"
)

//go:embed schema.sql seed.sql
var sqlFS embed.FS

func runMigrate() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://pv:pv123@localhost:5432/rooftoppv?sslmode=disable"
	}
	conn, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal(err)
	}
	if err := conn.Ping(); err != nil {
		log.Fatal("连接数据库失败: ", err)
	}
	runMigrateWith(conn)
}

// runMigrateWith 在给定连接上执行建表与种子数据
func runMigrateWith(conn *sql.DB) {
	for _, f := range []string{"schema.sql", "seed.sql"} {
		b, err := sqlFS.ReadFile(f)
		if err != nil {
			log.Fatal(err)
		}
		if _, err := conn.Exec(string(b)); err != nil {
			log.Fatalf("执行 %s 失败: %v", f, err)
		}
		fmt.Println("已执行:", f)
	}
}
