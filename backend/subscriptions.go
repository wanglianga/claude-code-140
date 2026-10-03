package main

import (
	"database/sql"
	"net/http"
	"time"
)

// ---------- 认购 ----------
func hListSubscriptions(w http.ResponseWriter, r *http.Request, m map[string]string) {
	id := atoi64(m["id"])
	if _, ok := residentOf(w, r, id); !ok {
		return
	}
	rows, err := db.Query(`SELECT s.id,s.resident_id,r.unit_no,r.owner_name,s.mode,s.component_qty,
		s.amount_paid,s.status,s.subscribe_pct,s.created_at,s.paid_at
		FROM subscriptions s JOIN residents r ON r.id=s.resident_id
		WHERE s.building_id=$1 ORDER BY r.unit_no`, id)
	if err != nil {
		serverErr(w, err)
		return
	}
	defer rows.Close()
	type sub struct {
		ID           int64   `json:"id"`
		ResidentID   int64   `json:"resident_id"`
		UnitNo       string  `json:"unit_no"`
		OwnerName    string  `json:"owner_name"`
		Mode         string  `json:"mode"`
		ComponentQty int     `json:"component_qty"`
		AmountPaid   float64 `json:"amount_paid"`
		Status       string  `json:"status"`
		SubscribePct float64 `json:"subscribe_pct"`
		CreatedAt    string  `json:"created_at"`
		PaidAt       string  `json:"paid_at"`
	}
	out := []sub{}
	var qtySum int
	for rows.Next() {
		var x sub
		var created time.Time
		var paid sql.NullTime
		if err := rows.Scan(&x.ID, &x.ResidentID, &x.UnitNo, &x.OwnerName, &x.Mode,
			&x.ComponentQty, &x.AmountPaid, &x.Status, &x.SubscribePct, &created, &paid); err != nil {
			serverErr(w, err)
			return
		}
		x.CreatedAt = created.Format("2006-01-02 15:04")
		x.PaidAt = dateOnly(paid)
		qtySum += x.ComponentQty // 含待缴费(已锁定容量)
		out = append(out, x)
	}
	var paidQtySum int
	_ = db.QueryRow(`SELECT COALESCE(SUM(component_qty),0) FROM subscriptions WHERE building_id=$1 AND status='PAID' AND mode='SUBSCRIBE'`, id).Scan(&paidQtySum)
	var totalComponents int
	_ = db.QueryRow(`SELECT component_count FROM buildings WHERE id=$1`, id).Scan(&totalComponents)
	// 公共剩余 = 总组件 - 已认购组件
	writeOK(w, map[string]interface{}{
		"subscriptions":        out,
		"total_components":     totalComponents,
		"subscribed_qty":       qtySum,     // 已锁定(含待缴费)
		"paid_qty":             paidQtySum, // 已缴费
		"public_qty":           totalComponents - qtySum,
		"subscribed_pct":       pct(qtySum, totalComponents),
		"public_component_pct": pct(totalComponents-qtySum, totalComponents),
	})
}

func pct(part, total int) float64 {
	if total == 0 {
		return 0
	}
	return round2(float64(part) / float64(total) * 100)
}

func hCreateSubscription(w http.ResponseWriter, r *http.Request, m map[string]string) {
	id := atoi64(m["id"])
	if _, ok := residentOf(w, r, id); !ok {
		return
	}
	var req struct {
		ResidentID   int64  `json:"resident_id"`
		Mode         string `json:"mode"`
		ComponentQty int    `json:"component_qty"`
	}
	if err := decode(r, &req); err != nil {
		badRequest(w, err)
		return
	}
	s := curUser(r)
	if s.Kind == "RESIDENT" && s.Role == "RESIDENT" {
		req.ResidentID = s.ResidentID // 住户只能给自己认购
	}
	if req.ResidentID == 0 {
		fail(w, 400, "请选择住户")
		return
	}
	if req.Mode == "" {
		req.Mode = "SUBSCRIBE"
	}
	// 校验楼栋阶段与住户
	var stage string
	var totalComponents int
	var unitPrice float64
	err := db.QueryRow(`SELECT stage,component_count,component_unit_price FROM buildings WHERE id=$1`, id).
		Scan(&stage, &totalComponents, &unitPrice)
	if err != nil {
		fail(w, 404, "楼栋不存在")
		return
	}
	if stage != "VOTING" && stage != "SUBSCRIBED" {
		fail(w, 400, "当前阶段不开放认购(需在楼栋表决通过后的认购缴费期)")
		return
	}
	var belongs int
	_ = db.QueryRow(`SELECT COUNT(*) FROM residents WHERE id=$1 AND building_id=$2 AND active=TRUE`,
		req.ResidentID, id).Scan(&belongs)
	if belongs == 0 {
		fail(w, 400, "住户不存在或已停用")
		return
	}
	// 已存在认购
	var existing string
	err = db.QueryRow(`SELECT status FROM subscriptions WHERE building_id=$1 AND resident_id=$2`,
		id, req.ResidentID).Scan(&existing)
	if err == nil {
		fail(w, 400, "该住户已有认购记录(待缴费或已缴费)")
		return
	}

	if req.Mode == "PUBLIC_ONLY" {
		req.ComponentQty = 0
	}
	if req.ComponentQty <= 0 && req.Mode == "SUBSCRIBE" {
		fail(w, 400, "认购组件数量必须大于0;如只参与公共收益分配请选择公共模式")
		return
	}
	// 容量校验:已 PAID + 已 PENDING 的占用 + 本次 <= 总数
	var used int
	_ = db.QueryRow(`SELECT COALESCE(SUM(component_qty),0) FROM subscriptions WHERE building_id=$1`, id).Scan(&used)
	if used+req.ComponentQty > totalComponents {
		fail(w, 400, "认购数量超过剩余组件容量(剩余 "+itoa(totalComponents-used)+" 块)")
		return
	}
	amount := round2(float64(req.ComponentQty) * unitPrice)
	sp := 0.0
	if totalComponents > 0 {
		sp = round2(float64(req.ComponentQty) / float64(totalComponents) * 100)
	}
	// 只参与公共收益分配无需出资,直接视为已确认
	status := "PENDING"
	if req.Mode == "PUBLIC_ONLY" {
		status = "PAID"
	}
	var sid int64
	err = db.QueryRow(`INSERT INTO subscriptions
		(building_id,resident_id,mode,component_qty,amount_paid,status,subscribe_pct,paid_at)
		VALUES ($1,$2,$3,$4,0,$5,$6,CASE WHEN $5='PAID' THEN now() ELSE NULL END) RETURNING id`,
		id, req.ResidentID, req.Mode, req.ComponentQty, status, sp).Scan(&sid)
	if err != nil {
		serverErr(w, err)
		return
	}
	writeOK(w, map[string]interface{}{"id": sid, "amount_due": amount, "status": status})
}

// ---------- 缴费 ----------
func hPaySubscription(w http.ResponseWriter, r *http.Request, m map[string]string) {
	sid := atoi64(m["sid"])
	var req struct {
		Amount float64 `json:"amount"`
	}
	_ = decode(r, &req)

	var bid, residentID, qty int64
	var unitPrice, paid float64
	var status string
	err := db.QueryRow(`SELECT s.building_id,s.resident_id,s.component_qty,s.status,s.amount_paid,b.component_unit_price
		FROM subscriptions s JOIN buildings b ON b.id=s.building_id WHERE s.id=$1`, sid).
		Scan(&bid, &residentID, &qty, &status, &paid, &unitPrice)
	if err != nil {
		fail(w, 404, "认购记录不存在")
		return
	}
	s := curUser(r)
	if !(s.Kind == "ADMIN" ||
		(s.Kind == "RESIDENT" && (s.BuildingID == bid && (s.Role == "PROPERTY" || s.ResidentID == residentID)))) {
		fail(w, 403, "无权为此认购缴费")
		return
	}
	if status != "PENDING" {
		fail(w, 400, "该认购已缴费或已退款")
		return
	}
	due := round2(float64(qty) * unitPrice)
	if req.Amount <= 0 {
		req.Amount = due
	}
	if req.Amount < due {
		fail(w, 400, "缴费金额不足,应缴 "+f2(due)+" 元")
		return
	}
	tx, err := db.Begin()
	if err != nil {
		serverErr(w, err)
		return
	}
	defer tx.Rollback()
	_, err = tx.Exec(`UPDATE subscriptions SET status='PAID',amount_paid=$2,paid_at=now() WHERE id=$1 AND status='PENDING'`,
		sid, req.Amount)
	if err != nil {
		serverErr(w, err)
		return
	}
	_, _ = tx.Exec(`INSERT INTO payments (building_id,resident_id,subscription_id,direction,amount,reason)
		VALUES ($1,$2,$3,'IN',$4,'认购组件缴款')`, bid, residentID, sid, req.Amount)
	// 认购本金归集楼栋公共账户(资本池),退款时从同一账户出账
	insertPublicTx(bid, tx, "", "SUBSCRIPTION", "IN", req.Amount,
		"SUBSCRIPTION", sid, "认购组件本金归集")
	if err := tx.Commit(); err != nil {
		serverErr(w, err)
		return
	}
	writeOK(w, map[string]interface{}{"id": sid, "paid": req.Amount, "status": "PAID"})
}

// ---------- 付款流水 ----------
func hListPayments(w http.ResponseWriter, r *http.Request, m map[string]string) {
	id := atoi64(m["id"])
	if _, ok := residentOf(w, r, id); !ok {
		return
	}
	rows, err := db.Query(`SELECT p.id,p.resident_id,r.unit_no,r.owner_name,p.direction,p.amount,p.reason,p.created_at
		FROM payments p JOIN residents r ON r.id=p.resident_id
		WHERE p.building_id=$1 ORDER BY p.id DESC`, id)
	if err != nil {
		serverErr(w, err)
		return
	}
	defer rows.Close()
	type pitem struct {
		ID         int64   `json:"id"`
		ResidentID int64   `json:"resident_id"`
		UnitNo     string  `json:"unit_no"`
		OwnerName  string  `json:"owner_name"`
		Direction  string  `json:"direction"`
		Amount     float64 `json:"amount"`
		Reason     string  `json:"reason"`
		CreatedAt  string  `json:"created_at"`
	}
	out := []pitem{}
	for rows.Next() {
		var x pitem
		var t time.Time
		if err := rows.Scan(&x.ID, &x.ResidentID, &x.UnitNo, &x.OwnerName,
			&x.Direction, &x.Amount, &x.Reason, &t); err != nil {
			serverErr(w, err)
			return
		}
		x.Amount = round2(x.Amount)
		x.CreatedAt = t.Format("2006-01-02 15:04")
		out = append(out, x)
	}
	writeOK(w, out)
}
