package main

import "database/sql"

type querier interface {
	QueryRow(query string, args ...interface{}) *sql.Row
}

// 公共账户当前余额 = 最近一笔流水的 balance_after
func publicBalanceQ(q querier, buildingID int64) float64 {
	var bal float64
	err := q.QueryRow(`SELECT COALESCE((SELECT balance_after FROM public_transactions
		WHERE building_id=$1 ORDER BY id DESC LIMIT 1),0)`, buildingID).Scan(&bal)
	if err != nil {
		return 0
	}
	return bal
}

// 公共账户记账。tx 为 nil 时使用 db。
func insertPublicTx(buildingID int64, tx *sql.Tx, period, typ, direction string,
	amount float64, refType string, refID int64, remark string) float64 {
	amount = round2(amount)
	exec := db.Exec
	bal := publicBalanceQ(db, buildingID)
	if tx != nil {
		exec = tx.Exec
		bal = publicBalanceQ(tx, buildingID)
	}
	if direction == "IN" {
		bal += amount
	} else {
		bal -= amount
	}
	bal = round2(bal)
	var per, ref interface{}
	if period != "" {
		per = period
	}
	if refID != 0 {
		ref = refID
	}
	_, _ = exec(`INSERT INTO public_transactions
		(building_id,period,type,direction,amount,balance_after,ref_type,ref_id,remark)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		buildingID, per, typ, direction, amount, bal, refType, ref, remark)
	return bal
}
