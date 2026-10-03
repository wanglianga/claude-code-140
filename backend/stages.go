package main

import "net/http"

// ---------- 表决 ----------
func hListVotes(w http.ResponseWriter, r *http.Request, m map[string]string) {
	id := atoi64(m["id"])
	if _, ok := residentOf(w, r, id); !ok {
		return
	}
	var threshold float64
	_ = db.QueryRow(`SELECT vote_threshold_pct FROM buildings WHERE id=$1`, id).Scan(&threshold)

	rows, err := db.Query(`SELECT v.resident_id,v.vote,v.opinion,v.created_at,r.unit_no,r.owner_name,r.area_owned
		FROM votes v JOIN residents r ON r.id=v.resident_id
		WHERE v.building_id=$1 AND v.topic_type='PROJECT' ORDER BY r.unit_no`, id)
	if err != nil {
		serverErr(w, err)
		return
	}
	defer rows.Close()
	type vitem struct {
		ResidentID int64   `json:"resident_id"`
		UnitNo     string  `json:"unit_no"`
		OwnerName  string  `json:"owner_name"`
		Vote       string  `json:"vote"`
		Opinion    string  `json:"opinion"`
		Area       float64 `json:"area_owned"`
		CreatedAt  string  `json:"created_at"`
	}
	list := []vitem{}
	var approveN, rejectN, abstainN int
	var approveArea, totalArea float64
	for rows.Next() {
		var x vitem
		var t interface{}
		if err := rows.Scan(&x.ResidentID, &x.Vote, &x.Opinion, &t, &x.UnitNo, &x.OwnerName, &x.Area); err != nil {
			serverErr(w, err)
			return
		}
		x.CreatedAt = fmtTime(t)
		list = append(list, x)
		switch x.Vote {
		case "APPROVE":
			approveN++
			approveArea += x.Area
		case "REJECT":
			rejectN++
		default:
			abstainN++
		}
	}
	var totalN int
	_ = db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(area_owned),0) FROM residents WHERE building_id=$1 AND active=TRUE`, id).
		Scan(&totalN, &totalArea)
	housePct, areaPct := 0.0, 0.0
	if totalN > 0 {
		housePct = round2(float64(approveN) / float64(totalN) * 100)
	}
	if totalArea > 0 {
		areaPct = round2(approveArea / totalArea * 100)
	}
	writeOK(w, map[string]interface{}{
		"votes": list,
		"tally": map[string]interface{}{
			"approve": approveN, "reject": rejectN, "abstain": abstainN, "total": totalN,
			"household_pct": housePct, "area_pct": areaPct, "threshold_pct": threshold,
			"passed": housePct >= threshold && areaPct >= threshold,
		},
	})
}

func hCastVote(w http.ResponseWriter, r *http.Request, m map[string]string) {
	id := atoi64(m["id"])
	s, ok := residentOf(w, r, id)
	if !ok {
		return
	}
	var req struct {
		Vote    string `json:"vote"`
		Opinion string `json:"opinion"`
	}
	if err := decode(r, &req); err != nil {
		badRequest(w, err)
		return
	}
	if req.Vote != "APPROVE" && req.Vote != "REJECT" && req.Vote != "ABSTAIN" {
		fail(w, 400, "票值必须为 APPROVE/REJECT/ABSTAIN")
		return
	}
	var stage string
	_ = db.QueryRow(`SELECT stage FROM buildings WHERE id=$1`, id).Scan(&stage)
	if stage != "INIT" && stage != "VOTING" {
		fail(w, 400, "当前阶段不在表决期")
		return
	}
	var voterID int64
	if s.Kind == "ADMIN" {
		fail(w, 403, "管理员不参与住户表决")
		return
	}
	voterID = s.ResidentID
	_, err := db.Exec(`INSERT INTO votes (building_id,resident_id,topic_type,topic_id,vote,opinion)
		VALUES ($1,$2,'PROJECT',0,$3,$4)
		ON CONFLICT (building_id,resident_id,topic_type,topic_id)
		DO UPDATE SET vote=$3, opinion=$4, created_at=now()`,
		id, voterID, req.Vote, req.Opinion)
	if err != nil {
		serverErr(w, err)
		return
	}
	if stage == "INIT" {
		_, _ = db.Exec(`UPDATE buildings SET stage='VOTING' WHERE id=$1 AND stage='INIT'`, id)
	}
	writeOK(w, map[string]bool{"ok": true})
}

// ---------- 阶段推进/回退 ----------
func hAdvanceStage(w http.ResponseWriter, r *http.Request, m map[string]string) {
	id := atoi64(m["id"])
	if _, ok := canManage(w, r, id); !ok {
		return
	}
	var req struct {
		Action string `json:"action"` // NEXT / BACK
		Note   string `json:"note"`
	}
	if err := decode(r, &req); err != nil {
		badRequest(w, err)
		return
	}
	if req.Action == "" {
		req.Action = "NEXT"
	}
	var stage string
	_ = db.QueryRow(`SELECT stage FROM buildings WHERE id=$1`, id).Scan(&stage)
	idx := stageIndex(stage)

	if req.Action == "BACK" {
		if idx == 0 {
			fail(w, 400, "已处于最初阶段")
			return
		}
		target := stages[idx-1]
		_, _ = db.Exec(`UPDATE buildings SET stage=$2 WHERE id=$1`, id, target)
		_, _ = db.Exec(`INSERT INTO schedule_events (building_id,kind,title,detail) VALUES ($1,'MILESTONE',$2,$3)`,
			id, "阶段调整:回退至"+stageNames[target], req.Note)
		writeOK(w, map[string]interface{}{"stage": target, "stage_name": stageNames[target]})
		return
	}

	if idx >= len(stages)-1 {
		fail(w, 400, "已处于最终运营阶段")
		return
	}
	target := stages[idx+1]

	// 门槛校验
	switch target {
	case "VOTING":
		// 启动表决无门槛
	case "SUBSCRIBED":
		var threshold float64
		_ = db.QueryRow(`SELECT vote_threshold_pct FROM buildings WHERE id=$1`, id).Scan(&threshold)
		var approveN, totalN int
		var approveArea, totalArea float64
		_ = db.QueryRow(`SELECT COUNT(*) FILTER (WHERE v.vote='APPROVE'),
			COALESCE(SUM(r.area_owned) FILTER (WHERE v.vote='APPROVE'),0)
			FROM residents r LEFT JOIN votes v ON v.resident_id=r.id AND v.topic_type='PROJECT'
			WHERE r.building_id=$1 AND r.active=TRUE`, id).Scan(&approveN, &approveArea)
		_ = db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(area_owned),0) FROM residents WHERE building_id=$1 AND active=TRUE`, id).
			Scan(&totalN, &totalArea)
		housePct, areaPct := 0.0, 0.0
		if totalN > 0 {
			housePct = float64(approveN) / float64(totalN) * 100
		}
		if totalArea > 0 {
			areaPct = approveArea / totalArea * 100
		}
		if housePct < threshold || areaPct < threshold {
			fail(w, 400, "表决未通过门槛(户数与面积双过半要求): "+
				"户数赞成"+f2(housePct)+"% / 面积赞成"+f2(areaPct)+"% / 门槛"+f2(threshold)+"%")
			return
		}
	case "SCHEDULED":
		var pending, total int
		_ = db.QueryRow(`SELECT COUNT(*) FILTER (WHERE status='PENDING'), COUNT(*)
			FROM subscriptions WHERE building_id=$1`, id).Scan(&pending, &total)
		if total == 0 {
			fail(w, 400, "尚无任何认购记录,无法进入施工排期")
			return
		}
		if pending > 0 {
			fail(w, 400, "仍有 "+itoa(pending)+" 笔认购未缴费")
			return
		}
	case "GRID_CONNECTED":
		var devN int
		_ = db.QueryRow(`SELECT COUNT(*) FROM devices WHERE building_id=$1`, id).Scan(&devN)
		if devN == 0 {
			fail(w, 400, "尚未登记逆变器/组件组,无法并网验收")
			return
		}
	}
	_, err := db.Exec(`UPDATE buildings SET stage=$2 WHERE id=$1`, id, target)
	if err != nil {
		serverErr(w, err)
		return
	}
	_, _ = db.Exec(`INSERT INTO schedule_events (building_id,kind,title,detail) VALUES ($1,'MILESTONE',$2,$3)`,
		id, "进入阶段:"+stageNames[target], req.Note)
	writeOK(w, map[string]interface{}{"stage": target, "stage_name": stageNames[target]})
}

// ---------- 排期事件 ----------
func hListSchedule(w http.ResponseWriter, r *http.Request, m map[string]string) {
	id := atoi64(m["id"])
	if _, ok := residentOf(w, r, id); !ok {
		return
	}
	rows, err := db.Query(`SELECT id,kind,title,detail,planned_on,actual_on,created_at
		FROM schedule_events WHERE building_id=$1 ORDER BY id DESC`, id)
	if err != nil {
		serverErr(w, err)
		return
	}
	defer rows.Close()
	writeOK(w, scanEvents(rows))
}

func hAddSchedule(w http.ResponseWriter, r *http.Request, m map[string]string) {
	id := atoi64(m["id"])
	s, ok := canManage(w, r, id)
	if !ok {
		return
	}
	var req struct {
		Kind      string `json:"kind"`
		Title     string `json:"title"`
		Detail    string `json:"detail"`
		PlannedOn string `json:"planned_on"`
		ActualOn  string `json:"actual_on"`
	}
	if err := decode(r, &req); err != nil {
		badRequest(w, err)
		return
	}
	if req.Title == "" {
		fail(w, 400, "事件标题必填")
		return
	}
	var uid interface{}
	if s.Kind == "RESIDENT" {
		uid = s.ResidentID
	}
	_, err := db.Exec(`INSERT INTO schedule_events (building_id,kind,title,detail,planned_on,actual_on,created_by)
		VALUES ($1,$2,$3,$4,NULLIF($5,'')::date,NULLIF($6,'')::date,$7)`,
		id, req.Kind, req.Title, req.Detail, req.PlannedOn, req.ActualOn, uid)
	if err != nil {
		serverErr(w, err)
		return
	}
	writeOK(w, map[string]bool{"ok": true})
}
