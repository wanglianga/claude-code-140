package main

import (
	"database/sql"
	"net/http"
	"time"
)

// 计算某周期的结算聚合数字(不落库)
type settleCalc struct {
	TotalKwh       float64    `json:"total_kwh"`
	Gross          float64    `json:"gross"`
	Subsidy        float64    `json:"subsidy"`
	MaintenanceFee float64    `json:"maintenance_fee"`
	Tax            float64    `json:"tax"`
	RepairFund     float64    `json:"repair_fund"`
	RepairCost     float64    `json:"repair_cost"`
	Deduction      float64    `json:"deduction"`
	Net            float64    `json:"net"`
	PublicIncome   float64    `json:"public_income"`
	Items          []itemCalc `json:"items"`
}

type itemCalc struct {
	ResidentID int64   `json:"resident_id"`
	UnitNo     string  `json:"unit_no"`
	OwnerName  string  `json:"owner_name"`
	Qty        int     `json:"qty"`
	SharePct   float64 `json:"share_pct"`
	Kwh        float64 `json:"kwh"`
	Amount     float64 `json:"amount"`
}

func calcSettlement(buildingID int64, period string) (*settleCalc, *buildingJSON, error) {
	b, err := scanBuilding(db.QueryRow(`SELECT `+buildingCols+` FROM buildings WHERE id=$1`, buildingID))
	if err != nil {
		return nil, nil, err
	}
	c := &settleCalc{}
	_ = db.QueryRow(`SELECT COALESCE(SUM(reading_kwh),0) FROM meter_readings
		WHERE building_id=$1 AND period=$2`, buildingID, period).Scan(&c.TotalKwh)

	c.Gross = round2(c.TotalKwh * b.ElectricityPrice)
	c.Subsidy = round2(c.TotalKwh * b.SubsidyRate)
	pool := c.Gross + c.Subsidy
	c.MaintenanceFee = round2(pool * b.MaintenanceFeePct / 100)
	c.Tax = round2((pool - c.MaintenanceFee) * b.TaxPct / 100)
	c.RepairFund = round2(pool * b.RepairFundPct / 100)

	// 该周期已解决工单:公共承担支出 与 责任方赔偿
	rows, err := db.Query(`SELECT COALESCE(responsibility,'PUBLIC'),COALESCE(cost,0),
		COALESCE(affected_kwh,0),COALESCE(deductible_share,0)
		FROM repair_orders WHERE building_id=$1 AND affect_period=$2 AND status='RESOLVED'`,
		buildingID, period)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var resp string
		var cost, affKwh, share float64
		if err := rows.Scan(&resp, &cost, &affKwh, &share); err != nil {
			return nil, nil, err
		}
		if resp == "PUBLIC" || resp == "PROPERTY" {
			c.RepairCost += cost
		}
		if share > 0 && affKwh > 0 {
			c.Deduction += affKwh * (b.ElectricityPrice + b.SubsidyRate) * share / 100
		}
	}
	c.RepairCost = round2(c.RepairCost)
	c.Deduction = round2(c.Deduction)

	c.Net = round2(c.Gross + c.Subsidy + c.Deduction - c.MaintenanceFee - c.Tax - c.RepairFund)

	// 住户分配:按已缴费认购组件比例,再扣除公共部分比例
	srows, err := db.Query(`SELECT s.resident_id,r.unit_no,r.owner_name,s.component_qty,s.subscribe_pct
		FROM subscriptions s JOIN residents r ON r.id=s.resident_id
		WHERE s.building_id=$1 AND s.status='PAID' ORDER BY r.unit_no`, buildingID)
	if err != nil {
		return nil, nil, err
	}
	defer srows.Close()
	residentPartFactor := (100 - b.PublicSharePct) / 100
	for srows.Next() {
		var it itemCalc
		if err := srows.Scan(&it.ResidentID, &it.UnitNo, &it.OwnerName, &it.Qty, &it.SharePct); err != nil {
			return nil, nil, err
		}
		it.Kwh = round2(c.TotalKwh * it.SharePct / 100)
		it.Amount = round2(c.Net * it.SharePct / 100 * residentPartFactor)
		c.Items = append(c.Items, it)
	}
	var residentSum float64
	for _, it := range c.Items {
		residentSum += it.Amount
	}
	c.PublicIncome = round2(c.Net - residentSum)
	return c, b, nil
}

// ---------- 结算列表 ----------
func hListSettlements(w http.ResponseWriter, r *http.Request, m map[string]string) {
	id := atoi64(m["id"])
	if _, ok := residentOf(w, r, id); !ok {
		return
	}
	rows, err := db.Query(`SELECT id,period,total_kwh,gross_income,subsidy,maintenance_fee,tax,
		repair_fund,repair_cost,deduction,net_income,public_income,status,note,created_at,confirmed_at
		FROM settlements WHERE building_id=$1 ORDER BY period DESC`, id)
	if err != nil {
		serverErr(w, err)
		return
	}
	defer rows.Close()
	type st struct {
		ID             int64       `json:"id"`
		Period         string      `json:"period"`
		TotalKwh       float64     `json:"total_kwh"`
		GrossIncome    float64     `json:"gross_income"`
		Subsidy        float64     `json:"subsidy"`
		MaintenanceFee float64     `json:"maintenance_fee"`
		Tax            float64     `json:"tax"`
		RepairFund     float64     `json:"repair_fund"`
		RepairCost     float64     `json:"repair_cost"`
		Deduction      float64     `json:"deduction"`
		NetIncome      float64     `json:"net_income"`
		PublicIncome   float64     `json:"public_income"`
		Status         string      `json:"status"`
		Note           string      `json:"note"`
		CreatedAt      string      `json:"created_at"`
		ConfirmedAt    string      `json:"confirmed_at"`
		Items          interface{} `json:"items"`
	}
	out := []st{}
	for rows.Next() {
		var x st
		var created time.Time
		var confirmed sql.NullTime
		if err := rows.Scan(&x.ID, &x.Period, &x.TotalKwh, &x.GrossIncome, &x.Subsidy,
			&x.MaintenanceFee, &x.Tax, &x.RepairFund, &x.RepairCost, &x.Deduction,
			&x.NetIncome, &x.PublicIncome, &x.Status, &x.Note, &created, &confirmed); err != nil {
			serverErr(w, err)
			return
		}
		x.CreatedAt = created.Format("2006-01-02 15:04")
		x.ConfirmedAt = dateOnly(confirmed)
		// 明细
		irows, err := db.Query(`SELECT COALESCE(resident_id,0),kind,share_pct,allocated_kwh,amount,
			COALESCE((SELECT owner_name FROM residents WHERE id=settlement_items.resident_id),'公共账户')
			FROM settlement_items WHERE settlement_id=$1 ORDER BY kind DESC, id`, x.ID)
		if err == nil {
			type it struct {
				ResidentID int64   `json:"resident_id"`
				Kind       string  `json:"kind"`
				SharePct   float64 `json:"share_pct"`
				Kwh        float64 `json:"allocated_kwh"`
				Amount     float64 `json:"amount"`
				Name       string  `json:"name"`
			}
			items := []it{}
			for irows.Next() {
				var z it
				if irows.Scan(&z.ResidentID, &z.Kind, &z.SharePct, &z.Kwh, &z.Amount, &z.Name) == nil {
					items = append(items, z)
				}
			}
			irows.Close()
			x.Items = items
		}
		out = append(out, x)
	}
	writeOK(w, out)
}

// ---------- 生成/重算草稿 ----------
func hCreateSettlement(w http.ResponseWriter, r *http.Request, m map[string]string) {
	id := atoi64(m["id"])
	if _, ok := canManage(w, r, id); !ok {
		return
	}
	var req struct {
		Period string `json:"period"`
		Note   string `json:"note"`
	}
	if err := decode(r, &req); err != nil {
		badRequest(w, err)
		return
	}
	if len(req.Period) != 7 || req.Period[4] != '-' {
		fail(w, 400, "周期格式应为 YYYY-MM")
		return
	}
	var existing int64
	var status string
	err := db.QueryRow(`SELECT id,status FROM settlements WHERE building_id=$1 AND period=$2`,
		id, req.Period).Scan(&existing, &status)
	if err == nil && status == "CONFIRMED" {
		fail(w, 400, req.Period+" 结算已确认入账,不可重算(可通过后续周期调整说明处理)")
		return
	}
	c, _, err := calcSettlement(id, req.Period)
	if err != nil {
		serverErr(w, err)
		return
	}
	if c.TotalKwh <= 0 {
		fail(w, 400, req.Period+" 无任何发电读数,无法生成结算")
		return
	}
	tx, err := db.Begin()
	if err != nil {
		serverErr(w, err)
		return
	}
	defer tx.Rollback()
	var sid int64
	if existing != 0 {
		sid = existing
		_, _ = tx.Exec(`DELETE FROM settlement_items WHERE settlement_id=$1`, sid)
		_, err = tx.Exec(`UPDATE settlements SET total_kwh=$2,gross_income=$3,subsidy=$4,
			maintenance_fee=$5,tax=$6,repair_fund=$7,repair_cost=$8,deduction=$9,net_income=$10,
			public_income=$11,status='DRAFT',note=$12,created_at=now(),confirmed_at=NULL WHERE id=$1`,
			sid, c.TotalKwh, c.Gross, c.Subsidy, c.MaintenanceFee, c.Tax, c.RepairFund,
			c.RepairCost, c.Deduction, c.Net, c.PublicIncome, req.Note)
	} else {
		err = tx.QueryRow(`INSERT INTO settlements
			(building_id,period,total_kwh,gross_income,subsidy,maintenance_fee,tax,repair_fund,
			repair_cost,deduction,net_income,public_income,status,note)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,'DRAFT',$13) RETURNING id`,
			id, req.Period, c.TotalKwh, c.Gross, c.Subsidy, c.MaintenanceFee, c.Tax, c.RepairFund,
			c.RepairCost, c.Deduction, c.Net, c.PublicIncome, req.Note).Scan(&sid)
	}
	if err != nil {
		serverErr(w, err)
		return
	}
	for _, it := range c.Items {
		_, err := tx.Exec(`INSERT INTO settlement_items
			(settlement_id,building_id,resident_id,kind,share_pct,allocated_kwh,amount)
			VALUES ($1,$2,$3,'SUBSCRIBER',$4,$5,$6)`,
			sid, id, it.ResidentID, it.SharePct, it.Kwh, it.Amount)
		if err != nil {
			serverErr(w, err)
			return
		}
	}
	_, err = tx.Exec(`INSERT INTO settlement_items
		(settlement_id,building_id,resident_id,kind,share_pct,allocated_kwh,amount)
		VALUES ($1,$2,NULL,'PUBLIC',0,$3,$4)`,
		sid, id, round2(c.TotalKwh-c.sumKwh()), c.PublicIncome)
	if err != nil {
		serverErr(w, err)
		return
	}
	if err := tx.Commit(); err != nil {
		serverErr(w, err)
		return
	}
	writeOK(w, map[string]interface{}{
		"id": sid, "period": req.Period, "status": "DRAFT", "calc": c,
	})
}

func (c *settleCalc) sumKwh() float64 {
	var s float64
	for _, it := range c.Items {
		s += it.Kwh
	}
	return round2(s)
}

// ---------- 确认结算并入账 ----------
func hConfirmSettlement(w http.ResponseWriter, r *http.Request, m map[string]string) {
	sid := atoi64(m["sid"])
	var bid int64
	var period, status string
	var gross, subsidy, maintFee, tax, deduction, net, publicInc float64
	err := db.QueryRow(`SELECT building_id,period,status,gross_income,subsidy,maintenance_fee,
		tax,deduction,net_income,public_income FROM settlements WHERE id=$1`, sid).
		Scan(&bid, &period, &status, &gross, &subsidy, &maintFee, &tax, &deduction, &net, &publicInc)
	if err != nil {
		fail(w, 404, "结算单不存在")
		return
	}
	if _, ok := canManage(w, r, bid); !ok {
		return
	}
	if status == "CONFIRMED" {
		fail(w, 400, "该结算单已确认")
		return
	}
	tx, err := db.Begin()
	if err != nil {
		serverErr(w, err)
		return
	}
	defer tx.Rollback()

	// 公共账户:发电收入/补贴/赔偿入账,运维/税费/住户收益发放出账
	insertPublicTx(bid, tx, period, "SETTLEMENT", "IN", gross, "SETTLEMENT", sid, period+"发电收入入账")
	insertPublicTx(bid, tx, period, "SUBSIDY", "IN", subsidy, "SETTLEMENT", sid, period+"度电补贴")
	if deduction > 0 {
		insertPublicTx(bid, tx, period, "COMPENSATION", "IN", deduction, "SETTLEMENT", sid, period+"责任方赔偿")
	}
	if maintFee > 0 {
		insertPublicTx(bid, tx, period, "MAINTENANCE", "OUT", maintFee, "SETTLEMENT", sid, period+"运维费用")
	}
	if tax > 0 {
		insertPublicTx(bid, tx, period, "TAX", "OUT", tax, "SETTLEMENT", sid, period+"税费")
	}
	// 住户收益发放
	rows, err := tx.Query(`SELECT resident_id,amount FROM settlement_items
		WHERE settlement_id=$1 AND kind='SUBSCRIBER'`, sid)
	if err != nil {
		serverErr(w, err)
		return
	}
	type pair struct {
		rid int64
		amt float64
	}
	pairs := []pair{}
	for rows.Next() {
		var p pair
		if rows.Scan(&p.rid, &p.amt) == nil && p.amt > 0 {
			pairs = append(pairs, p)
		}
	}
	rows.Close()
	var residentSum float64
	for _, p := range pairs {
		_, err := tx.Exec(`INSERT INTO payments (building_id,resident_id,direction,amount,reason)
			VALUES ($1,$2,'OUT_GAIN',$3,$4)`, bid, p.rid, p.amt, period+"发电收益发放")
		if err != nil {
			serverErr(w, err)
			return
		}
		residentSum += p.amt
	}
	if residentSum > 0 {
		insertPublicTx(bid, tx, period, "BENEFIT", "OUT", residentSum, "SETTLEMENT", sid, period+"住户收益发放")
	}
	_, err = tx.Exec(`UPDATE settlements SET status='CONFIRMED',confirmed_at=now() WHERE id=$1`, sid)
	if err != nil {
		serverErr(w, err)
		return
	}
	if err := tx.Commit(); err != nil {
		serverErr(w, err)
		return
	}
	writeOK(w, map[string]interface{}{
		"id": sid, "status": "CONFIRMED",
		"public_income": publicInc, "resident_payout": round2(residentSum),
	})
}

// ---------- 公共账户流水 ----------
func hListPublicTx(w http.ResponseWriter, r *http.Request, m map[string]string) {
	id := atoi64(m["id"])
	if _, ok := residentOf(w, r, id); !ok {
		return
	}
	rows, err := db.Query(`SELECT id,COALESCE(period,''),type,direction,amount,balance_after,
		COALESCE(ref_type,''),COALESCE(ref_id,0),remark,created_at
		FROM public_transactions WHERE building_id=$1 ORDER BY id DESC LIMIT 300`, id)
	if err != nil {
		serverErr(w, err)
		return
	}
	defer rows.Close()
	type tx2 struct {
		ID           int64   `json:"id"`
		Period       string  `json:"period"`
		Type         string  `json:"type"`
		Direction    string  `json:"direction"`
		Amount       float64 `json:"amount"`
		BalanceAfter float64 `json:"balance_after"`
		RefType      string  `json:"ref_type"`
		RefID        int64   `json:"ref_id"`
		Remark       string  `json:"remark"`
		CreatedAt    string  `json:"created_at"`
	}
	out := []tx2{}
	for rows.Next() {
		var x tx2
		var t time.Time
		if err := rows.Scan(&x.ID, &x.Period, &x.Type, &x.Direction, &x.Amount,
			&x.BalanceAfter, &x.RefType, &x.RefID, &x.Remark, &t); err != nil {
			serverErr(w, err)
			return
		}
		x.CreatedAt = t.Format("2006-01-02 15:04")
		out = append(out, x)
	}
	bal := publicBalanceQ(db, id)
	// 维修基金累计(已确认结算计提)
	var fund float64
	_ = db.QueryRow(`SELECT COALESCE(SUM(repair_fund),0) FROM settlements WHERE building_id=$1 AND status='CONFIRMED'`, id).Scan(&fund)
	writeOK(w, map[string]interface{}{
		"transactions":      out,
		"balance":           round2(bal),
		"repair_fund_total": round2(fund),
	})
}
