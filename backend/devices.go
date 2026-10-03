package main

import (
	"database/sql"
	"net/http"
	"time"
)

// ---------- 设备 ----------
func hListDevices(w http.ResponseWriter, r *http.Request, m map[string]string) {
	id := atoi64(m["id"])
	if _, ok := residentOf(w, r, id); !ok {
		return
	}
	rows, err := db.Query(`SELECT d.id,d.code,d.name,d.device_type,d.component_from,d.component_to,
		d.capacity_kw,d.status,d.installed_at,
		COALESCE((SELECT SUM(reading_kwh) FROM meter_readings mr WHERE mr.device_id=d.id),0)
		FROM devices d WHERE d.building_id=$1 ORDER BY d.component_from`, id)
	if err != nil {
		serverErr(w, err)
		return
	}
	defer rows.Close()
	type dev struct {
		ID            int64   `json:"id"`
		Code          string  `json:"code"`
		Name          string  `json:"name"`
		DeviceType    string  `json:"device_type"`
		ComponentFrom int     `json:"component_from"`
		ComponentTo   int     `json:"component_to"`
		CapacityKw    float64 `json:"capacity_kw"`
		Status        string  `json:"status"`
		InstalledAt   string  `json:"installed_at"`
		TotalKwh      float64 `json:"total_kwh"`
	}
	out := []dev{}
	for rows.Next() {
		var x dev
		var inst time.Time
		if err := rows.Scan(&x.ID, &x.Code, &x.Name, &x.DeviceType, &x.ComponentFrom,
			&x.ComponentTo, &x.CapacityKw, &x.Status, &inst, &x.TotalKwh); err != nil {
			serverErr(w, err)
			return
		}
		x.CapacityKw = round2(x.CapacityKw)
		x.TotalKwh = round2(x.TotalKwh)
		x.InstalledAt = inst.Format("2006-01-02")
		out = append(out, x)
	}
	writeOK(w, out)
}

func hCreateDevice(w http.ResponseWriter, r *http.Request, m map[string]string) {
	id := atoi64(m["id"])
	if _, ok := canManage(w, r, id); !ok {
		return
	}
	var req struct {
		Code          string  `json:"code"`
		Name          string  `json:"name"`
		DeviceType    string  `json:"device_type"`
		ComponentFrom int     `json:"component_from"`
		ComponentTo   int     `json:"component_to"`
		CapacityKw    float64 `json:"capacity_kw"`
	}
	if err := decode(r, &req); err != nil {
		badRequest(w, err)
		return
	}
	if req.DeviceType == "" {
		req.DeviceType = "INVERTER"
	}
	if req.Code == "" || req.Name == "" || req.ComponentTo < req.ComponentFrom {
		fail(w, 400, "设备编码/名称必填,组件区间需合法")
		return
	}
	var stage string
	_ = db.QueryRow(`SELECT stage FROM buildings WHERE id=$1`, id).Scan(&stage)
	if stage != "SCHEDULED" && stage != "GRID_CONNECTED" && stage != "OPERATING" {
		fail(w, 400, "需在施工排期阶段及之后登记设备")
		return
	}
	_, err := db.Exec(`INSERT INTO devices
		(building_id,code,name,device_type,component_from,component_to,capacity_kw)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (building_id,code) DO UPDATE
		SET name=$3,device_type=$4,component_from=$5,component_to=$6,capacity_kw=$7`,
		id, req.Code, req.Name, req.DeviceType, req.ComponentFrom, req.ComponentTo, req.CapacityKw)
	if err != nil {
		serverErr(w, err)
		return
	}
	writeOK(w, map[string]bool{"ok": true})
}

// ---------- 计量读数 ----------
func hListMeters(w http.ResponseWriter, r *http.Request, m map[string]string) {
	id := atoi64(m["id"])
	if _, ok := residentOf(w, r, id); !ok {
		return
	}
	rows, err := db.Query(`SELECT mr.id,mr.device_id,d.code,mr.period,mr.reading_kwh,mr.note,mr.created_at
		FROM meter_readings mr JOIN devices d ON d.id=mr.device_id
		WHERE mr.building_id=$1 ORDER BY mr.period DESC, d.code`, id)
	if err != nil {
		serverErr(w, err)
		return
	}
	defer rows.Close()
	type rd struct {
		ID         int64   `json:"id"`
		DeviceID   int64   `json:"device_id"`
		DeviceCode string  `json:"device_code"`
		Period     string  `json:"period"`
		ReadingKwh float64 `json:"reading_kwh"`
		Note       string  `json:"note"`
		CreatedAt  string  `json:"created_at"`
	}
	out := []rd{}
	periodTotals := map[string]float64{}
	for rows.Next() {
		var x rd
		var t time.Time
		if err := rows.Scan(&x.ID, &x.DeviceID, &x.DeviceCode, &x.Period, &x.ReadingKwh, &x.Note, &t); err != nil {
			serverErr(w, err)
			return
		}
		x.ReadingKwh = round2(x.ReadingKwh)
		x.CreatedAt = t.Format("2006-01-02 15:04")
		out = append(out, x)
		periodTotals[x.Period] += x.ReadingKwh
	}
	for k := range periodTotals {
		periodTotals[k] = round2(periodTotals[k])
	}
	writeOK(w, map[string]interface{}{"readings": out, "period_totals": periodTotals})
}

func hUpsertMeter(w http.ResponseWriter, r *http.Request, m map[string]string) {
	id := atoi64(m["id"])
	s, ok := canManage(w, r, id)
	if !ok {
		return
	}
	var req struct {
		DeviceID   int64   `json:"device_id"`
		Period     string  `json:"period"`
		ReadingKwh float64 `json:"reading_kwh"`
		Note       string  `json:"note"`
	}
	if err := decode(r, &req); err != nil {
		badRequest(w, err)
		return
	}
	if len(req.Period) != 7 || req.Period[4] != '-' || req.ReadingKwh < 0 || req.DeviceID == 0 {
		fail(w, 400, "请填写设备、周期(YYYY-MM)和非负发电量")
		return
	}
	var belong int
	_ = db.QueryRow(`SELECT COUNT(*) FROM devices WHERE id=$1 AND building_id=$2`, req.DeviceID, id).Scan(&belong)
	if belong == 0 {
		fail(w, 400, "设备不存在")
		return
	}
	var uid interface{}
	if s.Kind == "RESIDENT" {
		uid = s.ResidentID
	}
	_, err := db.Exec(`INSERT INTO meter_readings (building_id,device_id,period,reading_kwh,note,recorded_by)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (device_id,period) DO UPDATE SET reading_kwh=$4,note=$5,created_at=now()`,
		id, req.DeviceID, req.Period, req.ReadingKwh, req.Note, uid)
	if err != nil {
		serverErr(w, err)
		return
	}
	// 有发电数据后进入运营阶段
	_, _ = db.Exec(`UPDATE buildings SET stage='OPERATING' WHERE id=$1 AND stage IN ('GRID_CONNECTED')`, id)
	writeOK(w, map[string]bool{"ok": true})
}

// ---------- 发电异常解释 ----------
func hListGenNotes(w http.ResponseWriter, r *http.Request, m map[string]string) {
	id := atoi64(m["id"])
	if _, ok := residentOf(w, r, id); !ok {
		return
	}
	rows, err := db.Query(`SELECT g.id,g.period,g.title,g.explanation,g.created_at,COALESCE(r.owner_name,'管理员')
		FROM generation_notes g LEFT JOIN residents r ON r.id=g.created_by
		WHERE g.building_id=$1 ORDER BY g.period DESC, g.id DESC`, id)
	if err != nil {
		serverErr(w, err)
		return
	}
	defer rows.Close()
	type n struct {
		ID          int64  `json:"id"`
		Period      string `json:"period"`
		Title       string `json:"title"`
		Explanation string `json:"explanation"`
		Author      string `json:"author"`
		CreatedAt   string `json:"created_at"`
	}
	out := []n{}
	for rows.Next() {
		var x n
		var t time.Time
		if err := rows.Scan(&x.ID, &x.Period, &x.Title, &x.Explanation, &t, &x.Author); err != nil {
			serverErr(w, err)
			return
		}
		x.CreatedAt = t.Format("2006-01-02 15:04")
		out = append(out, x)
	}
	writeOK(w, out)
}

func hCreateGenNote(w http.ResponseWriter, r *http.Request, m map[string]string) {
	id := atoi64(m["id"])
	s, ok := canManage(w, r, id)
	if !ok {
		return
	}
	var req struct {
		Period      string `json:"period"`
		Title       string `json:"title"`
		Explanation string `json:"explanation"`
	}
	if err := decode(r, &req); err != nil {
		badRequest(w, err)
		return
	}
	if req.Title == "" || len(req.Period) != 7 {
		fail(w, 400, "标题和周期(YYYY-MM)必填")
		return
	}
	var uid interface{}
	if s.Kind == "RESIDENT" {
		uid = s.ResidentID
	}
	_, err := db.Exec(`INSERT INTO generation_notes (building_id,period,title,explanation,created_by)
		VALUES ($1,$2,$3,$4,$5)`, id, req.Period, req.Title, req.Explanation, uid)
	if err != nil {
		serverErr(w, err)
		return
	}
	writeOK(w, map[string]bool{"ok": true})
}

var _ = sql.ErrNoRows
