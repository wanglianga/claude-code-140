package main

import (
	"database/sql"
	"net/http"
	"time"
)

// ---------- 维修工单 ----------
func hListRepairs(w http.ResponseWriter, r *http.Request, m map[string]string) {
	id := atoi64(m["id"])
	if _, ok := residentOf(w, r, id); !ok {
		return
	}
	rows, err := db.Query(`SELECT ro.id,ro.code,COALESCE(ro.device_id,0),COALESCE(d.code,''),
		COALESCE(ro.reporter_id,0),COALESCE(r.owner_name,'管理员'),ro.category,ro.title,ro.description,ro.status,
		ro.responsibility,ro.cost,COALESCE(ro.affect_period,''),ro.affected_kwh,ro.deductible_share,
		ro.resolution,ro.created_at,ro.resolved_at
		FROM repair_orders ro
		LEFT JOIN devices d ON d.id=ro.device_id
		LEFT JOIN residents r ON r.id=ro.reporter_id
		WHERE ro.building_id=$1 ORDER BY ro.id DESC`, id)
	if err != nil {
		serverErr(w, err)
		return
	}
	defer rows.Close()
	type ro2 struct {
		ID              int64   `json:"id"`
		Code            string  `json:"code"`
		DeviceID        int64   `json:"device_id"`
		DeviceCode      string  `json:"device_code"`
		ReporterID      int64   `json:"reporter_id"`
		Reporter        string  `json:"reporter"`
		Category        string  `json:"category"`
		Title           string  `json:"title"`
		Description     string  `json:"description"`
		Status          string  `json:"status"`
		Responsibility  string  `json:"responsibility"`
		Cost            float64 `json:"cost"`
		AffectPeriod    string  `json:"affect_period"`
		AffectedKwh     float64 `json:"affected_kwh"`
		DeductibleShare float64 `json:"deductible_share"`
		Resolution      string  `json:"resolution"`
		CreatedAt       string  `json:"created_at"`
		ResolvedAt      string  `json:"resolved_at"`
		DeductionAmount float64 `json:"deduction_amount"`
	}
	out := []ro2{}
	var price, subsidy float64
	_ = db.QueryRow(`SELECT electricity_price,subsidy_rate FROM buildings WHERE id=$1`, id).Scan(&price, &subsidy)
	for rows.Next() {
		var x ro2
		var created time.Time
		var resolved sql.NullTime
		if err := rows.Scan(&x.ID, &x.Code, &x.DeviceID, &x.DeviceCode, &x.ReporterID, &x.Reporter,
			&x.Category, &x.Title, &x.Description, &x.Status, &x.Responsibility, &x.Cost,
			&x.AffectPeriod, &x.AffectedKwh, &x.DeductibleShare, &x.Resolution, &created, &resolved); err != nil {
			serverErr(w, err)
			return
		}
		x.Cost = round2(x.Cost)
		x.CreatedAt = created.Format("2006-01-02 15:04")
		x.ResolvedAt = dateOnly(resolved)
		// 责任赔偿 = 影响电量 × (电价+补贴) × 承担比例
		if x.Status == "RESOLVED" {
			x.DeductionAmount = round2(x.AffectedKwh * (price + subsidy) * x.DeductibleShare / 100)
		}
		out = append(out, x)
	}
	writeOK(w, out)
}

func hCreateRepair(w http.ResponseWriter, r *http.Request, m map[string]string) {
	id := atoi64(m["id"])
	s, ok := residentOf(w, r, id)
	if !ok {
		return
	}
	var req struct {
		DeviceID    int64  `json:"device_id"`
		Category    string `json:"category"`
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	if err := decode(r, &req); err != nil {
		badRequest(w, err)
		return
	}
	if req.Title == "" || req.Category == "" {
		fail(w, 400, "工单类别与标题必填")
		return
	}
	if req.Category != "INVERTER_FAULT" && req.Category != "LEAK" && req.Category != "OTHER" {
		fail(w, 400, "类别必须为 INVERTER_FAULT/LEAK/OTHER")
		return
	}
	tx, err := db.Begin()
	if err != nil {
		serverErr(w, err)
		return
	}
	defer tx.Rollback()
	var seq int64
	_ = tx.QueryRow(`SELECT COUNT(*) FROM repair_orders WHERE building_id=$1`, id).Scan(&seq)
	code := "WX-" + pad4(seq+1)
	var reporter interface{}
	if s.Kind == "RESIDENT" {
		reporter = s.ResidentID
	}
	var dev interface{}
	if req.DeviceID != 0 {
		dev = req.DeviceID
	}
	var rid int64
	err = tx.QueryRow(`INSERT INTO repair_orders
		(building_id,device_id,reporter_id,code,category,title,description)
		VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id`,
		id, dev, reporter, code, req.Category, req.Title, req.Description).Scan(&rid)
	if err != nil {
		serverErr(w, err)
		return
	}
	if req.DeviceID != 0 {
		_, _ = tx.Exec(`UPDATE devices SET status='FAULT' WHERE id=$1 AND building_id=$2`, req.DeviceID, id)
	}
	if err := tx.Commit(); err != nil {
		serverErr(w, err)
		return
	}
	writeOK(w, map[string]interface{}{"id": rid, "code": code})
}

func hResolveRepair(w http.ResponseWriter, r *http.Request, m map[string]string) {
	rid := atoi64(m["rid"])
	var req struct {
		Responsibility  string  `json:"responsibility"`
		Cost            float64 `json:"cost"`
		AffectPeriod    string  `json:"affect_period"`
		AffectedKwh     float64 `json:"affected_kwh"`
		DeductibleShare float64 `json:"deductible_share"`
		Resolution      string  `json:"resolution"`
	}
	if err := decode(r, &req); err != nil {
		badRequest(w, err)
		return
	}
	var bid, deviceID int64
	var status string
	err := db.QueryRow(`SELECT building_id,COALESCE(device_id,0),status FROM repair_orders WHERE id=$1`, rid).
		Scan(&bid, &deviceID, &status)
	if err != nil {
		fail(w, 404, "工单不存在")
		return
	}
	if _, ok := canManage(w, r, bid); !ok {
		return
	}
	if status != "OPEN" {
		fail(w, 400, "工单已处理完成")
		return
	}
	switch req.Responsibility {
	case "PROPERTY", "SUPPLIER", "INSTALLER", "RESIDENT", "PUBLIC":
	default:
		fail(w, 400, "责任方必须为 PROPERTY/SUPPLIER/INSTALLER/RESIDENT/PUBLIC")
		return
	}
	if req.AffectPeriod != "" && (len(req.AffectPeriod) != 7 || req.AffectPeriod[4] != '-') {
		fail(w, 400, "影响周期格式应为 YYYY-MM")
		return
	}
	tx, err := db.Begin()
	if err != nil {
		serverErr(w, err)
		return
	}
	defer tx.Rollback()
	var per interface{}
	if req.AffectPeriod != "" {
		per = req.AffectPeriod
	}
	_, err = tx.Exec(`UPDATE repair_orders SET status='RESOLVED',responsibility=$2,cost=$3,
		affect_period=$4,affected_kwh=$5,deductible_share=$6,resolution=$7,resolved_at=now()
		WHERE id=$1 AND status='OPEN'`,
		rid, req.Responsibility, req.Cost, per, req.AffectedKwh, req.DeductibleShare, req.Resolution)
	if err != nil {
		serverErr(w, err)
		return
	}
	if deviceID != 0 {
		_, _ = tx.Exec(`UPDATE devices SET status='REPAIRED' WHERE id=$1`, deviceID)
	}
	// 公共账户承担的维修费即时出账(责任方为 PUBLIC/PROPERTY 时由公共账户垫付)
	if req.Cost > 0 && (req.Responsibility == "PUBLIC" || req.Responsibility == "PROPERTY") {
		insertPublicTx(bid, tx, req.AffectPeriod, "REPAIR", "OUT", req.Cost,
			"REPAIR_ORDER", rid, "维修支出("+respName(req.Responsibility)+"承担)")
	}
	if err := tx.Commit(); err != nil {
		serverErr(w, err)
		return
	}
	writeOK(w, map[string]bool{"ok": true})
}

func respName(s string) string {
	return map[string]string{
		"PROPERTY": "物业", "SUPPLIER": "设备供应商", "INSTALLER": "施工方",
		"RESIDENT": "住户", "PUBLIC": "公共",
	}[s]
}

func pad4(n int64) string {
	s := itoa(int(n))
	for len(s) < 4 {
		s = "0" + s
	}
	return s
}
