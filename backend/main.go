package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	_ "github.com/lib/pq"
)

var db *sql.DB

// ---------- 会话(演示项目使用内存 token) ----------
type session struct {
	Kind       string // ADMIN / RESIDENT
	AdminID    int64
	AdminName  string
	ResidentID int64
	BuildingID int64
	Name       string
	Role       string // PROPERTY / RESIDENT
}

var (
	sessMu   sync.RWMutex
	sessions = map[string]*session{}
)

func newToken() string {
	b := make([]byte, 18)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func saveSession(s *session) string {
	t := newToken()
	sessMu.Lock()
	sessions[t] = s
	sessMu.Unlock()
	return t
}

func getSession(r *http.Request) *session {
	tok := r.Header.Get("Authorization")
	tok = trimPrefix(tok, "Bearer ")
	if tok == "" {
		return nil
	}
	sessMu.RLock()
	defer sessMu.RUnlock()
	return sessions[tok]
}

func trimPrefix(s, p string) string {
	if len(s) >= len(p) && s[:len(p)] == p {
		return s[len(p):]
	}
	return s
}

type ctxU struct{}

func auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s := getSession(r)
		if s == nil {
			fail(w, 401, "未登录或登录已失效")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxU{}, s)))
	})
}

func curUser(r *http.Request) *session { return r.Context().Value(ctxU{}).(*session) }

func cors(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization")
		h.ServeHTTP(w, r)
	})
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		runMigrate()
		return
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://pv:pv123@localhost:5432/rooftoppv?sslmode=disable"
	}
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	var err error
	db, err = sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal(err)
	}
	db.SetMaxOpenConns(20)
	db.SetConnMaxLifetime(30 * time.Minute)
	// 等待数据库就绪(compose 中后端可能先于 postgres 启动)
	for i := 0; i < 30; i++ {
		if err = db.Ping(); err == nil {
			break
		}
		log.Println("等待数据库就绪…", err)
		time.Sleep(2 * time.Second)
	}
	if err := db.Ping(); err != nil {
		log.Fatal("连接数据库失败: ", err)
	}
	if os.Getenv("AUTO_MIGRATE") == "1" || os.Getenv("AUTO_MIGRATE") == "true" {
		runMigrateWith(db)
	}

	mux := http.NewServeMux()

	// 公开接口
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		writeOK(w, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/api/login", handleLogin)

	// 需登录接口(注册到 /api/ 前缀下的自定义路由表)
	api := &router{}
	registerRoutes(api)
	mux.Handle("/api/", auth(api))

	// 可选:托管前端构建产物(FRONTEND_DIST 指向 vite build 的 dist)
	if dist := os.Getenv("FRONTEND_DIST"); dist != "" {
		registerStatic(mux, dist)
	}

	log.Println("服务已启动: http://localhost" + addr)
	srv := &http.Server{Addr: addr, Handler: cors(mux), ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second}
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

func registerRoutes(a *router) {
	// 仪表盘
	a.Handle("GET", "/api/dashboard", hDashboard)
	a.Handle("GET", "/api/buildings", hListBuildings)
	a.Handle("POST", "/api/buildings", hCreateBuilding)
	a.Handle("GET", "/api/buildings/{id}", hGetBuilding)
	a.Handle("PUT", "/api/buildings/{id}", hUpdateBuilding)

	// 楼栋内住户
	a.Handle("GET", "/api/buildings/{id}/residents", hListResidents)
	a.Handle("POST", "/api/buildings/{id}/residents", hCreateResident)

	// 表决与阶段
	a.Handle("GET", "/api/buildings/{id}/votes", hListVotes)
	a.Handle("POST", "/api/buildings/{id}/votes", hCastVote)
	a.Handle("POST", "/api/buildings/{id}/stage", hAdvanceStage)
	a.Handle("GET", "/api/buildings/{id}/schedule", hListSchedule)
	a.Handle("POST", "/api/buildings/{id}/schedule", hAddSchedule)

	// 认购与缴费
	a.Handle("GET", "/api/buildings/{id}/subscriptions", hListSubscriptions)
	a.Handle("POST", "/api/buildings/{id}/subscriptions", hCreateSubscription)
	a.Handle("POST", "/api/subscriptions/{sid}/pay", hPaySubscription)
	a.Handle("GET", "/api/buildings/{id}/payments", hListPayments)

	// 设备与计量
	a.Handle("GET", "/api/buildings/{id}/devices", hListDevices)
	a.Handle("POST", "/api/buildings/{id}/devices", hCreateDevice)
	a.Handle("GET", "/api/buildings/{id}/meters", hListMeters)
	a.Handle("POST", "/api/buildings/{id}/meters", hUpsertMeter)
	a.Handle("GET", "/api/buildings/{id}/generation-notes", hListGenNotes)
	a.Handle("POST", "/api/buildings/{id}/generation-notes", hCreateGenNote)

	// 维修
	a.Handle("GET", "/api/buildings/{id}/repairs", hListRepairs)
	a.Handle("POST", "/api/buildings/{id}/repairs", hCreateRepair)
	a.Handle("POST", "/api/repairs/{rid}/resolve", hResolveRepair)

	// 结算
	a.Handle("GET", "/api/buildings/{id}/settlements", hListSettlements)
	a.Handle("POST", "/api/buildings/{id}/settlements", hCreateSettlement) // 生成/重算草稿
	a.Handle("POST", "/api/settlements/{sid}/confirm", hConfirmSettlement)

	// 公共账户
	a.Handle("GET", "/api/buildings/{id}/public-transactions", hListPublicTx)

	// 转让
	a.Handle("GET", "/api/buildings/{id}/transfers", hListTransfers)
	a.Handle("POST", "/api/buildings/{id}/transfers", hCreateTransfer)
	a.Handle("POST", "/api/transfers/{tid}/process", hProcessTransfer)

	// 公告与异议
	a.Handle("GET", "/api/buildings/{id}/announcements", hListAnnouncements)
	a.Handle("POST", "/api/buildings/{id}/announcements", hCreateAnnouncement)
	a.Handle("GET", "/api/buildings/{id}/objections", hListObjections)
	a.Handle("POST", "/api/buildings/{id}/objections", hCreateObjection)
	a.Handle("POST", "/api/objections/{oid}/vote-start", hObjectionVoteStart)
	a.Handle("POST", "/api/objections/{oid}/vote", hObjectionVote)
	a.Handle("POST", "/api/objections/{oid}/close", hCloseObjection)

	// 年度报告 / 碳减排 / 当前用户
	a.Handle("GET", "/api/buildings/{id}/annual-report", hAnnualReport)
	a.Handle("GET", "/api/me", hMe)
}

func hMe(w http.ResponseWriter, r *http.Request, m map[string]string) {
	writeOK(w, curUser(r))
}

var _ = fmt.Sprintf
