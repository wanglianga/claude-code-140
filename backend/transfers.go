package main

import (
	"database/sql"
	"net/http"
	"time"
)

// ---------- 转让 ----------
func hListTransfers(w http.ResponseWriter, r *http.Request, m map[string]string) {
	id := atoi64(m["id"])
	if _, ok := residentOf(w, r, id); !ok {
		return
	}
	rows, err := db.Query(`SELECT t.id,t.subscription_id,t.from_resident_id,f.unit_no,f.owner_name,
		COALESCE(t.to_resident_id,0),t.new_owner_name,t.transfer_subscription,t.settle_history,
		t.settled_amount,t.refund_amount,t.status,t.remark,t.created_at,t.processed_at
		FROM ownership_transfers t
		JOIN residents f ON f.id=t.from_resident_id
		WHERE t.building_id=$1 ORDER BY t.id DESC`, id)
	if err != nil {
		serverErr(w, err)
		return
	}
	defer rows.Close()
	type tr struct {
		ID                   int64   `json:"id"`
		SubscriptionID       int64   `json:"subscription_id"`
		FromResidentID       int64   `json:"from_resident_id"`
		FromUnit             string  `json:"from_unit"`
		FromName             string  `json:"from_name"`
		ToResidentID         int64   `json:"to_resident_id"`
		NewOwnerName         string  `json:"new_owner_name"`
		TransferSubscription bool    `json:"transfer_subscription"`
		SettleHistory        bool    `json:"settle_history"`
		SettledAmount        float64 `json:"settled_amount"`
		RefundAmount         float64 `json:"refund_amount"`
		Status               string  `json:"status"`
		Remark               string  `json:"remark"`
		CreatedAt            string  `json:"created_at"`
		ProcessedAt          string  `json:"processed_at"`
	}
	out := []tr{}
	for rows.Next() {
		var x tr
		var created time.Time
		var processed sql.NullTime
		if err := rows.Scan(&x.ID, &x.SubscriptionID, &x.FromResidentID, &x.FromUnit, &x.FromName,
			&x.ToResidentID, &x.NewOwnerName, &x.TransferSubscription, &x.SettleHistory,
			&x.SettledAmount, &x.RefundAmount, &x.Status, &x.Remark, &created, &processed); err != nil {
			serverErr(w, err)
			return
		}
		x.CreatedAt = created.Format("2006-01-02 15:04")
		x.ProcessedAt = dateOnly(processed)
		out = append(out, x)
	}
	writeOK(w, out)
}

func hCreateTransfer(w http.ResponseWriter, r *http.Request, m map[string]string) {
	id := atoi64(m["id"])
	s, ok := residentOf(w, r, id)
	if !ok {
		return
	}
	var req struct {
		ResidentID           int64  `json:"resident_id"`
		NewOwnerName         string `json:"new_owner_name"`
		NewPhone             string `json:"new_phone"`
		TransferSubscription bool   `json:"transfer_subscription"`
		SettleHistory        bool   `json:"settle_history"`
		Remark               string `json:"remark"`
	}
	if err := decode(r, &req); err != nil {
		badRequest(w, err)
		return
	}
	if s.Kind == "RESIDENT" && s.Role == "RESIDENT" {
		req.ResidentID = s.ResidentID
	}
	if req.ResidentID == 0 || req.NewOwnerName == "" {
		fail(w, 400, "请选择原住户并填写新业主姓名")
		return
	}
	var subID int64
	var active bool
	err := db.QueryRow(`SELECT COALESCE((SELECT id FROM subscriptions WHERE building_id=$1 AND resident_id=$2 LIMIT 1),0),
		active FROM residents WHERE id=$2 AND building_id=$1`, id, req.ResidentID).Scan(&subID, &active)
	if err != nil {
		fail(w, 404, "原住户不存在")
		return
	}
	if !active {
		fail(w, 400, "该住户已停用(房屋已转让)")
		return
	}
	var sid interface{}
	if subID != 0 {
		sid = subID
	}
	var tid int64
	err = db.QueryRow(`INSERT INTO ownership_transfers
		(building_id,subscription_id,from_resident_id,new_owner_name,new_phone,
		transfer_subscription,settle_history,remark)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`,
		id, sid, req.ResidentID, req.NewOwnerName, req.NewPhone,
		req.TransferSubscription, req.SettleHistory, req.Remark).Scan(&tid)
	if err != nil {
		serverErr(w, err)
		return
	}
	writeOK(w, map[string]interface{}{"id": tid})
}

// 处理转让:
// 1) 历史收益结清:统计原住户已确认结算累计收益(已发放),作为结清金额记录;
// 2) 权益随房转移:新建/复用新业主住户,认购记录改挂新业主;
// 3) 不转移:认购退款(未结算本金),原认购置 REFUNDED。
func hProcessTransfer(w http.ResponseWriter, r *http.Request, m map[string]string) {
	tid := atoi64(m["tid"])
	var bid, fromID, subID int64
	var newName, newPhone, status string
	var transferSub, settleHist bool
	err := db.QueryRow(`SELECT building_id,from_resident_id,COALESCE(subscription_id,0),
		new_owner_name,COALESCE(new_phone,''),transfer_subscription,settle_history,status
		FROM ownership_transfers WHERE id=$1`, tid).
		Scan(&bid, &fromID, &subID, &newName, &newPhone, &transferSub, &settleHist, &status)
	if err != nil {
		fail(w, 404, "转让记录不存在")
		return
	}
	if _, ok := canManage(w, r, bid); !ok {
		return
	}
	if status != "PENDING" {
		fail(w, 400, "该转让已处理")
		return
	}

	tx, err := db.Begin()
	if err != nil {
		serverErr(w, err)
		return
	}
	defer tx.Rollback()

	// 1. 历史收益结清金额 = 原住户已确认结算明细累计(收益随月度已实际发放,此处仅作结清确认)
	var settled float64
	if settleHist && subID != 0 {
		_ = tx.QueryRow(`SELECT COALESCE(SUM(si.amount),0)
			FROM settlement_items si JOIN settlements s ON s.id=si.settlement_id
			WHERE si.resident_id=$1 AND s.status='CONFIRMED'`, fromID).Scan(&settled)
	}

	// 原住户房号信息
	var unit string
	var isTop bool
	var area float64
	if err := tx.QueryRow(`SELECT unit_no,is_top_floor,area_owned FROM residents WHERE id=$1`, fromID).
		Scan(&unit, &isTop, &area); err != nil {
		serverErr(w, err)
		return
	}

	var toID int64
	var refund float64

	if transferSub && subID != 0 {
		// 2. 权益随房转移:同房号更新为新业主(保持一条住户记录,密码重置)
		_, err = tx.Exec(`UPDATE residents SET owner_name=$2,phone=$3,password='123456',active=TRUE
			WHERE id=$1`, fromID, newName, newPhone)
		if err != nil {
			serverErr(w, err)
			return
		}
		toID = fromID
		_, _ = tx.Exec(`UPDATE ownership_transfers SET to_resident_id=$2 WHERE id=$1`, tid, toID)
	} else {
		// 3. 不随房转移:新建新业主任,原业主停用;认购按未退本金退款
		err = tx.QueryRow(`INSERT INTO residents
			(building_id,unit_no,owner_name,phone,role,area_owned,is_top_floor,password,active)
			VALUES ($1,$2,$3,$4,'RESIDENT',$5,$6,'123456',TRUE) RETURNING id`,
			bid, unit+"-新", newName, newPhone, area, isTop).Scan(&toID)
		if err != nil {
			serverErr(w, err)
			return
		}
		_, _ = tx.Exec(`UPDATE ownership_transfers SET to_resident_id=$2 WHERE id=$1`, tid, toID)
		_, _ = tx.Exec(`UPDATE residents SET active=FALSE WHERE id=$1`, fromID)
		if subID != 0 {
			var subStatus string
			var paid float64
			if err := tx.QueryRow(`SELECT status,amount_paid FROM subscriptions WHERE id=$1`, subID).
				Scan(&subStatus, &paid); err == nil && subStatus == "PAID" {
				refund = round2(paid)
				_, _ = tx.Exec(`UPDATE subscriptions SET status='REFUNDED' WHERE id=$1`, subID)
				_, _ = tx.Exec(`INSERT INTO payments (building_id,resident_id,subscription_id,direction,amount,reason)
					VALUES ($1,$2,$3,'OUT',$4,'房屋转让,认购不随房转移,退还认购本金')`,
					bid, fromID, subID, refund)
				insertPublicTx(bid, tx, "", "REFUND", "OUT", refund,
					"TRANSFER", tid, "房屋转让认购退款("+unit+")")
			}
		}
	}

	_, err = tx.Exec(`UPDATE ownership_transfers SET status='TRANSFERRED',settle_history=$2,
		settled_amount=$3,refund_amount=$4,processed_at=now() WHERE id=$1`,
		tid, settleHist, round2(settled), round2(refund))
	if err != nil {
		serverErr(w, err)
		return
	}
	if err := tx.Commit(); err != nil {
		serverErr(w, err)
		return
	}
	writeOK(w, map[string]interface{}{
		"id": tid, "status": "TRANSFERRED",
		"to_resident_id": toID, "settled_amount": round2(settled), "refund_amount": round2(refund),
	})
}
