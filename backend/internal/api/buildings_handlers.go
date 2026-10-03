package api

import (
	"encoding/json"
	"net/http"

	"pvplatform/internal/auth"
)

type buildingIn struct {
	Name                 string          `json:"name"`
	Address              string          `json:"address"`
	RoofArea             float64         `json:"roof_area"`
	Orientation          string          `json:"orientation"`
	LoadAssessment       string          `json:"load_assessment"`
	Shading              string          `json:"shading"`
	OwnershipNote        string          `json:"ownership_note"`
	PublicRevenueRule    json.RawMessage `json:"public_revenue_rule"`
	GridCondition        string          `json:"grid_condition"`
	ConstructionImpact   string          `json:"construction_impact"`
	TotalCapacityKw      float64         `json:"total_capacity_kw"`
	PanelTotal           int             `json:"panel_total"`
	UnitPrice            float64         `json:"unit_price"`
	MaintenanceFundRatio float64         `json:"maintenance_fund_ratio"`
}

func (s *Server) createBuilding(w http.ResponseWriter, r *http.Request) {
	if u := auth.RequireRole(w, r, "admin", "property"); u == nil {
		return
	}
	var in buildingIn
	if err := decodeJSON(r, &in); err != nil || in.Name == "" {
		writeErr(w, 400, "楼栋名称为必填项")
		return
	}
	if len(in.PublicRevenueRule) == 0 {
		in.PublicRevenueRule = json.RawMessage(`{}`)
	}
	if in.MaintenanceFundRatio <= 0 {
		in.MaintenanceFundRatio = 20
	}
	// 公共收益规则中持久化维修基金计提比例
	var rule map[string]any
	_ = json.Unmarshal(in.PublicRevenueRule, &rule)
	if rule == nil {
		rule = map[string]any{}
	}
	rule["maintenance_fund_ratio"] = in.MaintenanceFundRatio
	ruleBytes, _ := json.Marshal(rule)

	var id int64
	err := s.Pool.QueryRow(r.Context(), `
		INSERT INTO buildings(name, address, roof_area, orientation, load_assessment, shading,
			ownership_note, public_revenue_rule, grid_condition, construction_impact,
			total_capacity_kw, panel_total, unit_price, stage)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,'draft') RETURNING id`,
		in.Name, in.Address, in.RoofArea, in.Orientation, in.LoadAssessment, in.Shading,
		in.OwnershipNote, string(ruleBytes), in.GridCondition, in.ConstructionImpact,
		in.TotalCapacityKw, in.PanelTotal, in.UnitPrice).Scan(&id)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	_, _ = s.Pool.Exec(r.Context(),
		`INSERT INTO stage_logs(building_id, from_stage, to_stage, reason, operator_id) VALUES ($1,NULL,'draft','项目立项建档',$2)`,
		id, auth.CurrentUser(r).ID)
	writeJSON(w, 201, map[string]any{"id": id})
}

func scanBuilding(row interface {
	Scan(dest ...any) error
}) (map[string]any, error) {
	var (
		id                            int64
		name, address, orientation    string
		load, shading, ownership      string
		ruleJSON, gridCond, impact    string
		roofArea, capacity, unitPrice float64
		subRatio                      float64
		panelTotal                    int
		stage                         string
	)
	if err := row.Scan(&id, &name, &address, &roofArea, &orientation, &load, &shading,
		&ownership, &ruleJSON, &gridCond, &impact, &capacity, &panelTotal, &unitPrice, &subRatio, &stage); err != nil {
		return nil, err
	}
	var rule map[string]any
	_ = json.Unmarshal([]byte(ruleJSON), &rule)
	return map[string]any{
		"id": id, "name": name, "address": address, "roof_area": roofArea,
		"orientation": orientation, "load_assessment": load, "shading": shading,
		"ownership_note": ownership, "public_revenue_rule": rule,
		"grid_condition": gridCond, "construction_impact": impact,
		"total_capacity_kw": capacity, "panel_total": panelTotal,
		"unit_price": unitPrice, "subscribed_ratio": subRatio, "stage": stage,
	}, nil
}

const buildingCols = `id, name, address, roof_area, orientation, load_assessment, shading,
	ownership_note, public_revenue_rule::text, grid_condition, construction_impact,
	total_capacity_kw, panel_total, unit_price, subscribed_ratio, stage`

func (s *Server) getBuilding(w http.ResponseWriter, r *http.Request) {
	if u := auth.Require(w, r); u == nil {
		return
	}
	id := urlID(r, "id")
	row := s.Pool.QueryRow(r.Context(), `SELECT `+buildingCols+` FROM buildings WHERE id=$1`, id)
	b, err := scanBuilding(row)
	if isNoRows(err) {
		writeErr(w, 404, "楼栋不存在")
		return
	} else if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, b)
}

func (s *Server) listBuildings(w http.ResponseWriter, r *http.Request) {
	if u := auth.Require(w, r); u == nil {
		return
	}
	rows, err := s.Pool.Query(r.Context(),
		`SELECT `+buildingCols+` FROM buildings ORDER BY id`)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		b, err := scanBuilding(rows)
		if err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		out = append(out, b)
	}
	writeJSON(w, 200, map[string]any{"items": out})
}

func (s *Server) updateBuilding(w http.ResponseWriter, r *http.Request) {
	u := auth.RequireRole(w, r, "admin", "property")
	if u == nil {
		return
	}
	id := urlID(r, "id")
	var in buildingIn
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, 400, "请求格式错误")
		return
	}
	var rule map[string]any
	if len(in.PublicRevenueRule) > 0 {
		_ = json.Unmarshal(in.PublicRevenueRule, &rule)
	} else {
		_ = s.Pool.QueryRow(r.Context(), `SELECT public_revenue_rule::text FROM buildings WHERE id=$1`, id).Scan(&rule)
	}
	if rule == nil {
		rule = map[string]any{}
	}
	if in.MaintenanceFundRatio > 0 {
		rule["maintenance_fund_ratio"] = in.MaintenanceFundRatio
	}
	ruleBytes, _ := json.Marshal(rule)
	_, err := s.Pool.Exec(r.Context(), `
		UPDATE buildings SET name=COALESCE(NULLIF($2,''),name), address=$3, roof_area=CASE WHEN $4=0 THEN roof_area ELSE $4 END,
			orientation=$5, load_assessment=$6, shading=$7, ownership_note=$8, public_revenue_rule=$9,
			grid_condition=$10, construction_impact=$11, total_capacity_kw=CASE WHEN $12=0 THEN total_capacity_kw ELSE $12 END,
			panel_total=CASE WHEN $13=0 THEN panel_total ELSE $13 END, unit_price=CASE WHEN $14=0 THEN unit_price ELSE $14 END
		WHERE id=$1`,
		id, in.Name, in.Address, in.RoofArea, in.Orientation, in.LoadAssessment, in.Shading,
		in.OwnershipNote, string(ruleBytes), in.GridCondition, in.ConstructionImpact,
		in.TotalCapacityKw, in.PanelTotal, in.UnitPrice)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

// allowedTransitions 正常推进路径；因异议/外部因素允许回退（需 reason）
var stageOrder = map[string]int{
	"draft": 0, "voting": 1, "subscribing": 2, "construction": 3, "grid_acceptance": 4, "operating": 5,
}

func (s *Server) transitionStage(w http.ResponseWriter, r *http.Request) {
	u := auth.RequireRole(w, r, "admin", "property")
	if u == nil {
		return
	}
	id := urlID(r, "id")
	var in struct {
		ToStage string `json:"to_stage"`
		Reason  string `json:"reason"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, 400, "请求格式错误")
		return
	}
	newOrd, ok := stageOrder[in.ToStage]
	if !ok {
		writeErr(w, 400, "目标阶段不合法")
		return
	}
	var oldStage string
	err := s.Pool.QueryRow(r.Context(), `SELECT stage FROM buildings WHERE id=$1`, id).Scan(&oldStage)
	if isNoRows(err) {
		writeErr(w, 404, "楼栋不存在")
		return
	} else if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if oldStage == in.ToStage {
		writeErr(w, 400, "楼栋已处于该阶段")
		return
	}
	// 推进到认购阶段前必须有通过的楼栋表决
	if in.ToStage == "subscribing" {
		var approved int
		_ = s.Pool.QueryRow(r.Context(),
			`SELECT count(*) FROM votes WHERE building_id=$1 AND result='approved'`, id).Scan(&approved)
		if approved == 0 {
			writeErr(w, 409, "楼栋立项表决尚未通过，不能进入认购缴费阶段")
			return
		}
	}
	// 回退必须填写调整原因（漏水担忧/维护通道/收益质疑/供货延迟/并网时间变化等）
	if newOrd < stageOrder[oldStage] && in.Reason == "" {
		writeErr(w, 409, "阶段回退属于计划调整，必须填写调整原因")
		return
	}
	tx, err := s.Pool.Begin(r.Context())
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	defer tx.Rollback(r.Context())
	if _, err := tx.Exec(r.Context(), `UPDATE buildings SET stage=$2 WHERE id=$1`, id, in.ToStage); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if _, err := tx.Exec(r.Context(),
		`INSERT INTO stage_logs(building_id, from_stage, to_stage, reason, operator_id) VALUES ($1,$2,$3,$4,$5)`,
		id, oldStage, in.ToStage, in.Reason, u.ID); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "stage": in.ToStage})
}

func (s *Server) listStageLogs(w http.ResponseWriter, r *http.Request) {
	if u := auth.Require(w, r); u == nil {
		return
	}
	id := urlID(r, "id")
	rows, err := s.Pool.Query(r.Context(), `
		SELECT l.id, COALESCE(l.from_stage,''), l.to_stage, l.reason, l.created_at, COALESCE(u.name,'系统')
		FROM stage_logs l LEFT JOIN users u ON u.id=l.operator_id
		WHERE l.building_id=$1 ORDER BY l.id`, id)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	defer rows.Close()
	out := []any{}
	for rows.Next() {
		var lid int64
		var from, to, reason, who string
		var at any
		if err := rows.Scan(&lid, &from, &to, &reason, &at, &who); err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		out = append(out, map[string]any{"id": lid, "from": from, "to": to, "reason": reason, "at": at, "operator": who})
	}
	writeJSON(w, 200, map[string]any{"items": out})
}

func (s *Server) listMembers(w http.ResponseWriter, r *http.Request) {
	if u := auth.Require(w, r); u == nil {
		return
	}
	id := urlID(r, "id")
	rows, err := s.Pool.Query(r.Context(), `
		SELECT u.id, u.name, COALESCE(u.unit_no,''), u.role, COALESCE(bm.member_type,'resident'), COALESCE(u.balance,0)
		FROM building_members bm JOIN users u ON u.id=bm.user_id
		WHERE bm.building_id=$1 ORDER BY u.id`, id)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	defer rows.Close()
	out := []any{}
	for rows.Next() {
		var uid int64
		var name, unit, role, mtype string
		var balance float64
		if err := rows.Scan(&uid, &name, &unit, &role, &mtype, &balance); err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		out = append(out, map[string]any{
			"id": uid, "name": name, "unit_no": unit, "role": role,
			"member_type": mtype, "balance": balance,
		})
	}
	writeJSON(w, 200, map[string]any{"items": out})
}

func (s *Server) addMember(w http.ResponseWriter, r *http.Request) {
	u := auth.RequireRole(w, r, "admin", "property")
	if u == nil {
		return
	}
	id := urlID(r, "id")
	var in struct {
		UserID     int64  `json:"user_id"`
		MemberType string `json:"member_type"`
	}
	if err := decodeJSON(r, &in); err != nil || in.UserID == 0 {
		writeErr(w, 400, "user_id 为必填项")
		return
	}
	if in.MemberType == "" {
		in.MemberType = "resident"
	}
	if _, err := s.Pool.Exec(r.Context(),
		`INSERT INTO building_members(building_id,user_id,member_type) VALUES ($1,$2,$3)
		 ON CONFLICT DO NOTHING`, id, in.UserID, in.MemberType); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 201, map[string]any{"ok": true})
}
