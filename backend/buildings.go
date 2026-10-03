package main

import (
	"net/http"
	"strings"
)

// 阶段顺序
var stages = []string{"INIT", "VOTING", "SUBSCRIBED", "SCHEDULED", "GRID_CONNECTED", "OPERATING"}

var stageNames = map[string]string{
	"INIT":           "立项评估",
	"VOTING":         "楼栋表决",
	"SUBSCRIBED":     "认购缴费",
	"SCHEDULED":      "施工排期",
	"GRID_CONNECTED": "并网验收",
	"OPERATING":      "发电运营",
}

func stageIndex(s string) int {
	for i, v := range stages {
		if v == s {
			return i
		}
	}
	return 0
}

const carbonFactor = 0.581 // kgCO2/kWh(华东电网参考值)

// ---------- 仪表盘 ----------
func hDashboard(w http.ResponseWriter, r *http.Request, m map[string]string) {
	var buildings, devices, openRepairs int
	var capacity, kwh, income, publicBal float64
	err := db.QueryRow(`SELECT
		COUNT(*), COALESCE(SUM(total_capacity_kw),0),
		COALESCE((SELECT COUNT(*) FROM devices),0),
		COALESCE((SELECT SUM(reading_kwh) FROM meter_readings),0),
		COALESCE((SELECT SUM(net_income) FROM settlements WHERE status='CONFIRMED'),0),
		COALESCE((SELECT COUNT(*) FROM repair_orders WHERE status='OPEN'),0)
		FROM buildings`).
		Scan(&buildings, &capacity, &devices, &kwh, &income, &openRepairs)
	if err != nil {
		serverErr(w, err)
		return
	}
	// 各楼公共账户余额 = 最近一笔流水 balance_after
	rows, err := db.Query(`SELECT DISTINCT ON (building_id) balance_after
		FROM public_transactions ORDER BY building_id, id DESC`)
	if err != nil {
		serverErr(w, err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var b float64
		if err := rows.Scan(&b); err == nil {
			publicBal += b
		}
	}
	writeOK(w, map[string]interface{}{
		"building_count": buildings,
		"total_capacity": round2(capacity),
		"device_count":   devices,
		"total_kwh":      round2(kwh),
		"total_income":   round2(income),
		"public_balance": round2(publicBal),
		"open_repairs":   openRepairs,
		"carbon_kg":      round2(kwh * carbonFactor),
		"carbon_t":       round2(kwh * carbonFactor / 1000),
	})
}

// ---------- 楼栋 ----------
type buildingJSON struct {
	ID                 int64   `json:"id"`
	Code               string  `json:"code"`
	Name               string  `json:"name"`
	Address            string  `json:"address"`
	RooftopArea        float64 `json:"rooftop_area"`
	Orientation        string  `json:"orientation"`
	LoadAssessment     string  `json:"load_assessment"`
	Shading            string  `json:"shading"`
	PropertyRights     string  `json:"property_rights"`
	PublicBenefitRule  string  `json:"public_benefit_rule"`
	GridCondition      string  `json:"grid_condition"`
	ConstructionImpact string  `json:"construction_impact"`
	TotalCapacityKw    float64 `json:"total_capacity_kw"`
	ComponentCount     int     `json:"component_count"`
	ComponentUnitPrice float64 `json:"component_unit_price"`
	PublicSharePct     float64 `json:"public_share_pct"`
	VoteThresholdPct   float64 `json:"vote_threshold_pct"`
	ElectricityPrice   float64 `json:"electricity_price"`
	SubsidyRate        float64 `json:"subsidy_rate"`
	MaintenanceFeePct  float64 `json:"maintenance_fee_pct"`
	TaxPct             float64 `json:"tax_pct"`
	RepairFundPct      float64 `json:"repair_fund_pct"`
	Stage              string  `json:"stage"`
	StageName          string  `json:"stage_name"`
}

func scanBuilding(sc interface{ Scan(...interface{}) error }) (*buildingJSON, error) {
	var b buildingJSON
	err := sc.Scan(&b.ID, &b.Code, &b.Name, &b.Address, &b.RooftopArea, &b.Orientation,
		&b.LoadAssessment, &b.Shading, &b.PropertyRights, &b.PublicBenefitRule, &b.GridCondition,
		&b.ConstructionImpact, &b.TotalCapacityKw, &b.ComponentCount, &b.ComponentUnitPrice,
		&b.PublicSharePct, &b.VoteThresholdPct, &b.ElectricityPrice, &b.SubsidyRate,
		&b.MaintenanceFeePct, &b.TaxPct, &b.RepairFundPct, &b.Stage)
	b.StageName = stageNames[b.Stage]
	return &b, err
}

const buildingCols = `id, code, name, address, rooftop_area, orientation, load_assessment,
	shading, property_rights, public_benefit_rule, grid_condition, construction_impact,
	total_capacity_kw, component_count, component_unit_price, public_share_pct,
	vote_threshold_pct, electricity_price, subsidy_rate, maintenance_fee_pct, tax_pct,
	repair_fund_pct, stage`

func hListBuildings(w http.ResponseWriter, r *http.Request, m map[string]string) {
	s := curUser(r)
	q := `SELECT ` + buildingCols + ` FROM buildings`
	args := []interface{}{}
	if s.Kind == "RESIDENT" {
		q += ` WHERE id=$1 ORDER BY id`
		args = append(args, s.BuildingID)
	} else {
		q += ` ORDER BY id`
	}
	rows, err := db.Query(q, args...)
	if err != nil {
		serverErr(w, err)
		return
	}
	defer rows.Close()
	type item struct {
		buildingJSON
		ResidentCount  int     `json:"resident_count"`
		PaidComponents int     `json:"paid_components"`
		SubscribePct   float64 `json:"subscribe_pct"`
		TotalKwh       float64 `json:"total_kwh"`
		PublicBalance  float64 `json:"public_balance"`
	}
	out := []item{}
	for rows.Next() {
		b, err := scanBuilding(rows)
		if err != nil {
			serverErr(w, err)
			return
		}
		var paidQty, totalKwh, publicBal float64
		var residentCount int
		_ = db.QueryRow(`SELECT COALESCE(SUM(component_qty),0) FROM subscriptions WHERE building_id=$1 AND status='PAID'`, b.ID).Scan(&paidQty)
		_ = db.QueryRow(`SELECT COUNT(*) FROM residents WHERE building_id=$1`, b.ID).Scan(&residentCount)
		_ = db.QueryRow(`SELECT COALESCE(SUM(reading_kwh),0) FROM meter_readings WHERE building_id=$1`, b.ID).Scan(&totalKwh)
		_ = db.QueryRow(`SELECT balance_after FROM public_transactions WHERE building_id=$1 ORDER BY id DESC LIMIT 1`, b.ID).Scan(&publicBal)
		pct := 0.0
		if b.ComponentCount > 0 {
			pct = round2(paidQty / float64(b.ComponentCount) * 100)
		}
		out = append(out, item{*b, residentCount, int(paidQty), pct, round2(totalKwh), round2(publicBal)})
	}
	writeOK(w, out)
}

func hGetBuilding(w http.ResponseWriter, r *http.Request, m map[string]string) {
	id := atoi64(m["id"])
	if _, ok := residentOf(w, r, id); !ok {
		return
	}
	b, err := scanBuilding(db.QueryRow(`SELECT `+buildingCols+` FROM buildings WHERE id=$1`, id))
	if err != nil {
		fail(w, 404, "楼栋不存在")
		return
	}
	writeOK(w, b)
}

func hCreateBuilding(w http.ResponseWriter, r *http.Request, m map[string]string) {
	if _, ok := requireAdmin(w, r); !ok {
		return
	}
	var b buildingJSON
	if err := decode(r, &b); err != nil {
		badRequest(w, err)
		return
	}
	if strings.TrimSpace(b.Code) == "" || strings.TrimSpace(b.Name) == "" {
		fail(w, 400, "楼栋编号与名称必填")
		return
	}
	var id int64
	row := db.QueryRow(`INSERT INTO buildings
		(code,name,address,rooftop_area,orientation,load_assessment,shading,property_rights,
		public_benefit_rule,grid_condition,construction_impact,total_capacity_kw,component_count,
		component_unit_price,public_share_pct,vote_threshold_pct,electricity_price,subsidy_rate,
		maintenance_fee_pct,tax_pct,repair_fund_pct)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21)
		RETURNING id`,
		b.Code, b.Name, b.Address, b.RooftopArea, b.Orientation, b.LoadAssessment, b.Shading,
		b.PropertyRights, b.PublicBenefitRule, b.GridCondition, b.ConstructionImpact,
		b.TotalCapacityKw, b.ComponentCount, b.ComponentUnitPrice, b.PublicSharePct,
		b.VoteThresholdPct, b.ElectricityPrice, b.SubsidyRate, b.MaintenanceFeePct, b.TaxPct,
		b.RepairFundPct)
	if err := row.Scan(&id); err != nil {
		serverErr(w, err)
		return
	}
	// 公共账户初始流水
	insertPublicTx(id, nil, "", "INIT", "IN", 0, "", 0, "公共账户开立")
	writeOK(w, map[string]interface{}{"id": id})
}

func hUpdateBuilding(w http.ResponseWriter, r *http.Request, m map[string]string) {
	id := atoi64(m["id"])
	if _, ok := canManage(w, r, id); !ok {
		return
	}
	var b buildingJSON
	if err := decode(r, &b); err != nil {
		badRequest(w, err)
		return
	}
	_, err := db.Exec(`UPDATE buildings SET name=$2,address=$3,rooftop_area=$4,orientation=$5,
		load_assessment=$6,shading=$7,property_rights=$8,public_benefit_rule=$9,grid_condition=$10,
		construction_impact=$11,total_capacity_kw=$12,component_count=$13,component_unit_price=$14,
		public_share_pct=$15,vote_threshold_pct=$16,electricity_price=$17,subsidy_rate=$18,
		maintenance_fee_pct=$19,tax_pct=$20,repair_fund_pct=$21 WHERE id=$1`,
		id, b.Name, b.Address, b.RooftopArea, b.Orientation, b.LoadAssessment, b.Shading,
		b.PropertyRights, b.PublicBenefitRule, b.GridCondition, b.ConstructionImpact,
		b.TotalCapacityKw, b.ComponentCount, b.ComponentUnitPrice, b.PublicSharePct,
		b.VoteThresholdPct, b.ElectricityPrice, b.SubsidyRate, b.MaintenanceFeePct, b.TaxPct,
		b.RepairFundPct)
	if err != nil {
		serverErr(w, err)
		return
	}
	writeOK(w, map[string]bool{"ok": true})
}

// ---------- 住户 ----------
func hListResidents(w http.ResponseWriter, r *http.Request, m map[string]string) {
	id := atoi64(m["id"])
	if _, ok := residentOf(w, r, id); !ok {
		return
	}
	rows, err := db.Query(`SELECT r.id,r.unit_no,r.owner_name,r.phone,r.role,r.area_owned,r.is_top_floor,r.active,
		COALESCE(s.id,0),COALESCE(s.mode,''),COALESCE(s.component_qty,0),COALESCE(s.amount_paid,0),COALESCE(s.status,''),
		COALESCE(s.subscribe_pct,0)
		FROM residents r LEFT JOIN subscriptions s ON s.resident_id=r.id
		WHERE r.building_id=$1 ORDER BY r.unit_no`, id)
	if err != nil {
		serverErr(w, err)
		return
	}
	defer rows.Close()
	type res struct {
		ID           int64   `json:"id"`
		UnitNo       string  `json:"unit_no"`
		OwnerName    string  `json:"owner_name"`
		Phone        string  `json:"phone"`
		Role         string  `json:"role"`
		AreaOwned    float64 `json:"area_owned"`
		IsTopFloor   bool    `json:"is_top_floor"`
		Active       bool    `json:"active"`
		SubID        int64   `json:"subscription_id"`
		SubMode      string  `json:"sub_mode"`
		ComponentQty int     `json:"component_qty"`
		AmountPaid   float64 `json:"amount_paid"`
		SubStatus    string  `json:"sub_status"`
		SubscribePct float64 `json:"subscribe_pct"`
	}
	out := []res{}
	for rows.Next() {
		var x res
		if err := rows.Scan(&x.ID, &x.UnitNo, &x.OwnerName, &x.Phone, &x.Role, &x.AreaOwned,
			&x.IsTopFloor, &x.Active, &x.SubID, &x.SubMode, &x.ComponentQty, &x.AmountPaid,
			&x.SubStatus, &x.SubscribePct); err != nil {
			serverErr(w, err)
			return
		}
		out = append(out, x)
	}
	writeOK(w, out)
}

func hCreateResident(w http.ResponseWriter, r *http.Request, m map[string]string) {
	id := atoi64(m["id"])
	if _, ok := canManage(w, r, id); !ok {
		return
	}
	var req struct {
		UnitNo     string  `json:"unit_no"`
		OwnerName  string  `json:"owner_name"`
		Phone      string  `json:"phone"`
		Role       string  `json:"role"`
		AreaOwned  float64 `json:"area_owned"`
		IsTopFloor bool    `json:"is_top_floor"`
		Password   string  `json:"password"`
	}
	if err := decode(r, &req); err != nil {
		badRequest(w, err)
		return
	}
	if req.Role == "" {
		req.Role = "RESIDENT"
	}
	if req.Password == "" {
		req.Password = "123456"
	}
	var rid int64
	err := db.QueryRow(`INSERT INTO residents
		(building_id,unit_no,owner_name,phone,role,area_owned,is_top_floor,password)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`,
		id, req.UnitNo, req.OwnerName, req.Phone, req.Role, req.AreaOwned, req.IsTopFloor, req.Password).
		Scan(&rid)
	if err != nil {
		serverErr(w, err)
		return
	}
	writeOK(w, map[string]interface{}{"id": rid})
}
