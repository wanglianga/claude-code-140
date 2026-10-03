package main

import (
	"net/http"
	"time"
)

// ---------- 公告 ----------
func hListAnnouncements(w http.ResponseWriter, r *http.Request, m map[string]string) {
	id := atoi64(m["id"])
	if _, ok := residentOf(w, r, id); !ok {
		return
	}
	rows, err := db.Query(`SELECT a.id,a.title,a.content,a.created_at,COALESCE(r.owner_name,'管理员')
		FROM announcements a LEFT JOIN residents r ON r.id=a.created_by
		WHERE a.building_id=$1 ORDER BY a.id DESC`, id)
	if err != nil {
		serverErr(w, err)
		return
	}
	defer rows.Close()
	type an struct {
		ID        int64  `json:"id"`
		Title     string `json:"title"`
		Content   string `json:"content"`
		Author    string `json:"author"`
		CreatedAt string `json:"created_at"`
	}
	out := []an{}
	for rows.Next() {
		var x an
		var t time.Time
		if rows.Scan(&x.ID, &x.Title, &x.Content, &t, &x.Author) == nil {
			x.CreatedAt = t.Format("2006-01-02 15:04")
			out = append(out, x)
		}
	}
	writeOK(w, out)
}

func hCreateAnnouncement(w http.ResponseWriter, r *http.Request, m map[string]string) {
	id := atoi64(m["id"])
	s, ok := canManage(w, r, id)
	if !ok {
		return
	}
	var req struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := decode(r, &req); err != nil {
		badRequest(w, err)
		return
	}
	if req.Title == "" {
		fail(w, 400, "公告标题必填")
		return
	}
	var uid interface{}
	if s.Kind == "RESIDENT" {
		uid = s.ResidentID
	}
	_, err := db.Exec(`INSERT INTO announcements (building_id,title,content,created_by)
		VALUES ($1,$2,$3,$4)`, id, req.Title, req.Content, uid)
	if err != nil {
		serverErr(w, err)
		return
	}
	writeOK(w, map[string]bool{"ok": true})
}

// ---------- 异议 ----------
func listObjections(buildingID int64) ([]map[string]interface{}, error) {
	rows, err := db.Query(`SELECT o.id,o.resident_id,r.unit_no,r.owner_name,o.title,o.content,
		o.status,o.created_at,o.closed_at
		FROM objections o JOIN residents r ON r.id=o.resident_id
		WHERE o.building_id=$1 ORDER BY o.id DESC`, buildingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]interface{}{}
	for rows.Next() {
		var id, rid int64
		var unit, name, title, content, status string
		var created time.Time
		var closed interface{}
		if err := rows.Scan(&id, &rid, &unit, &name, &title, &content, &status, &created, &closed); err != nil {
			continue
		}
		// 投票统计
		var approve, reject, abstain, totalVoters int
		_ = db.QueryRow(`SELECT
			COUNT(*) FILTER (WHERE vote='APPROVE'),
			COUNT(*) FILTER (WHERE vote='REJECT'),
			COUNT(*) FILTER (WHERE vote='ABSTAIN'),
			(SELECT COUNT(*) FROM residents WHERE building_id=$1 AND active=TRUE)
			FROM votes WHERE building_id=$1 AND topic_type='OBJECTION' AND topic_id=$2`,
			buildingID, id).Scan(&approve, &reject, &abstain, &totalVoters)
		out = append(out, map[string]interface{}{
			"id": id, "resident_id": rid, "unit_no": unit, "owner_name": name,
			"title": title, "content": content, "status": status,
			"created_at": created.Format("2006-01-02 15:04"),
			"votes": map[string]interface{}{
				"approve": approve, "reject": reject, "abstain": abstain,
				"total_voters":  totalVoters,
				"participation": pct(approve+reject+abstain, totalVoters),
			},
		})
	}
	return out, nil
}

func hListObjections(w http.ResponseWriter, r *http.Request, m map[string]string) {
	id := atoi64(m["id"])
	if _, ok := residentOf(w, r, id); !ok {
		return
	}
	out, err := listObjections(id)
	if err != nil {
		serverErr(w, err)
		return
	}
	writeOK(w, out)
}

func hCreateObjection(w http.ResponseWriter, r *http.Request, m map[string]string) {
	id := atoi64(m["id"])
	s, ok := residentOf(w, r, id)
	if !ok {
		return
	}
	var req struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := decode(r, &req); err != nil {
		badRequest(w, err)
		return
	}
	if s.Kind != "RESIDENT" {
		fail(w, 403, "只有住户可以提出异议")
		return
	}
	if req.Title == "" {
		fail(w, 400, "异议标题必填")
		return
	}
	var oid int64
	err := db.QueryRow(`INSERT INTO objections (building_id,resident_id,title,content)
		VALUES ($1,$2,$3,$4) RETURNING id`, id, s.ResidentID, req.Title, req.Content).Scan(&oid)
	if err != nil {
		serverErr(w, err)
		return
	}
	writeOK(w, map[string]interface{}{"id": oid})
}

func hObjectionVoteStart(w http.ResponseWriter, r *http.Request, m map[string]string) {
	oid := atoi64(m["oid"])
	var bid int64
	var status string
	if err := db.QueryRow(`SELECT building_id,status FROM objections WHERE id=$1`, oid).Scan(&bid, &status); err != nil {
		fail(w, 404, "异议不存在")
		return
	}
	if _, ok := canManage(w, r, bid); !ok {
		return
	}
	if status != "OPEN" {
		fail(w, 400, "仅受理中的异议可发起投票")
		return
	}
	_, _ = db.Exec(`UPDATE objections SET status='VOTING' WHERE id=$1`, oid)
	writeOK(w, map[string]bool{"ok": true})
}

func hObjectionVote(w http.ResponseWriter, r *http.Request, m map[string]string) {
	oid := atoi64(m["oid"])
	var bid int64
	var status string
	if err := db.QueryRow(`SELECT building_id,status FROM objections WHERE id=$1`, oid).Scan(&bid, &status); err != nil {
		fail(w, 404, "异议不存在")
		return
	}
	s, ok := residentOf(w, r, bid)
	if !ok {
		return
	}
	if s.Kind != "RESIDENT" {
		fail(w, 403, "管理员不参与住户投票")
		return
	}
	if status != "VOTING" {
		fail(w, 400, "该异议不在投票中")
		return
	}
	var req struct {
		Vote string `json:"vote"`
	}
	if err := decode(r, &req); err != nil {
		badRequest(w, err)
		return
	}
	if req.Vote != "APPROVE" && req.Vote != "REJECT" && req.Vote != "ABSTAIN" {
		fail(w, 400, "票值非法")
		return
	}
	_, err := db.Exec(`INSERT INTO votes (building_id,resident_id,topic_type,topic_id,vote)
		VALUES ($1,$2,'OBJECTION',$3,$4)
		ON CONFLICT (building_id,resident_id,topic_type,topic_id)
		DO UPDATE SET vote=$4, created_at=now()`, bid, s.ResidentID, oid, req.Vote)
	if err != nil {
		serverErr(w, err)
		return
	}
	writeOK(w, map[string]bool{"ok": true})
}

// 关闭异议:依据投票结果判定异议成立/驳回
func hCloseObjection(w http.ResponseWriter, r *http.Request, m map[string]string) {
	oid := atoi64(m["oid"])
	var bid int64
	var status string
	if err := db.QueryRow(`SELECT building_id,status FROM objections WHERE id=$1`, oid).Scan(&bid, &status); err != nil {
		fail(w, 404, "异议不存在")
		return
	}
	if _, ok := canManage(w, r, bid); !ok {
		return
	}
	if status != "VOTING" {
		fail(w, 400, "仅投票中的异议可裁决关闭")
		return
	}
	var approve, reject, total int
	_ = db.QueryRow(`SELECT
		COUNT(*) FILTER (WHERE vote='APPROVE'),
		COUNT(*) FILTER (WHERE vote='REJECT'),
		(SELECT COUNT(*) FROM residents WHERE building_id=$1 AND active=TRUE)
		FROM votes WHERE building_id=$1 AND topic_type='OBJECTION' AND topic_id=$2`,
		bid, oid).Scan(&approve, &reject, &total)
	result := "REJECTED"
	if total > 0 && approve*2 > total { // 参与口径下赞成过半(全体住户过半为严格门槛)
		result = "APPROVED"
	}
	_, _ = db.Exec(`UPDATE objections SET status=$2,closed_at=now() WHERE id=$1`, oid, result)
	writeOK(w, map[string]interface{}{"id": oid, "result": result, "approve": approve, "reject": reject})
}

// ---------- 年度收益报告 / 碳减排 ----------
func hAnnualReport(w http.ResponseWriter, r *http.Request, m map[string]string) {
	id := atoi64(m["id"])
	if _, ok := residentOf(w, r, id); !ok {
		return
	}
	year := r.URL.Query().Get("year")
	if year == "" {
		year = "2026"
	}
	rows, err := db.Query(`SELECT period,total_kwh,gross_income,subsidy,maintenance_fee,tax,
		repair_fund,repair_cost,deduction,net_income,public_income,status
		FROM settlements WHERE building_id=$1 AND period LIKE $2 ORDER BY period`,
		id, year+"-%")
	if err != nil {
		serverErr(w, err)
		return
	}
	defer rows.Close()
	months := []map[string]interface{}{}
	var yKwh, yGross, ySubsidy, yMaint, yTax, yFund, yRepair, yDeduct, yNet, yPublic float64
	for rows.Next() {
		var period, status string
		var kwh, gross, subsidy, maint, tax, fund, repair, deduct, net, public float64
		if err := rows.Scan(&period, &kwh, &gross, &subsidy, &maint, &tax, &fund,
			&repair, &deduct, &net, &public, &status); err != nil {
			continue
		}
		if status == "CONFIRMED" {
			yKwh += kwh
			yGross += gross
			ySubsidy += subsidy
			yMaint += maint
			yTax += tax
			yFund += fund
			yRepair += repair
			yDeduct += deduct
			yNet += net
			yPublic += public
		}
		months = append(months, map[string]interface{}{
			"period": period, "kwh": round2(kwh), "gross": round2(gross),
			"subsidy": round2(subsidy), "net": round2(net),
			"public_income": round2(public), "status": status,
		})
	}
	// 住户年度收益排行
	resRows, err := db.Query(`SELECT r.unit_no,r.owner_name,COALESCE(SUM(si.amount),0),COALESCE(SUM(si.allocated_kwh),0)
		FROM settlement_items si
		JOIN settlements s ON s.id=si.settlement_id
		JOIN residents r ON r.id=si.resident_id
		WHERE si.building_id=$1 AND s.status='CONFIRMED' AND s.period LIKE $2
		GROUP BY r.id ORDER BY 3 DESC`, id, year+"-%")
	rank := []map[string]interface{}{}
	if err == nil {
		defer resRows.Close()
		for resRows.Next() {
			var unit, name string
			var amt, kwh float64
			if resRows.Scan(&unit, &name, &amt, &kwh) == nil {
				rank = append(rank, map[string]interface{}{
					"unit_no": unit, "owner_name": name,
					"amount": round2(amt), "kwh": round2(kwh),
				})
			}
		}
	}
	// 年度公共支出
	var pubOut float64
	_ = db.QueryRow(`SELECT COALESCE(SUM(amount),0) FROM public_transactions
		WHERE building_id=$1 AND direction='OUT' AND period LIKE $2`, id, year+"-%").Scan(&pubOut)
	// 年度维修数
	var repairN, resolvedN int
	_ = db.QueryRow(`SELECT COUNT(*),COUNT(*) FILTER (WHERE status='RESOLVED')
		FROM repair_orders WHERE building_id=$1 AND to_char(created_at,'YYYY')=$2`, id, year).
		Scan(&repairN, &resolvedN)

	writeOK(w, map[string]interface{}{
		"year":   year,
		"months": months,
		"summary": map[string]interface{}{
			"total_kwh":       round2(yKwh),
			"gross_income":    round2(yGross),
			"subsidy":         round2(ySubsidy),
			"maintenance":     round2(yMaint),
			"tax":             round2(yTax),
			"repair_fund":     round2(yFund),
			"repair_cost":     round2(yRepair),
			"deduction":       round2(yDeduct),
			"net_income":      round2(yNet),
			"public_income":   round2(yPublic),
			"public_out":      round2(pubOut),
			"repair_orders":   repairN,
			"repair_resolved": resolvedN,
			"carbon_kg":       round2(yKwh * carbonFactor),
			"carbon_t":        round2(yKwh * carbonFactor / 1000),
			"coal_kg":         round2(yKwh * 0.309),               // 节约标煤
			"trees":           round2(yKwh * carbonFactor / 18.3), // 等效成年树年吸收(约18.3kg CO2/棵)
		},
		"resident_rank": rank,
	})
}
