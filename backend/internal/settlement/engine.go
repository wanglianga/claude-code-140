// Package settlement 实现光伏发电月度收益结算引擎（纯计算，无数据库依赖，便于单元测试）。
package settlement

import "math"

// 华东电网平均 CO2 排放因子 (kg/kWh)，用于碳减排估算
const CarbonFactorKgPerKwh = 0.5810

// Share 已认购住户的组件份额
type Share struct {
	UserID int64   `json:"user_id"`
	Name   string  `json:"name"`
	Ratio  float64 `json:"ratio"` // 占整楼组件比例 %
}

// WorkOrder 当期维修工单（仅 property/owner 责任的费用由项目承担；installer/insurance 走赔偿）
type WorkOrder struct {
	ID             int64   `json:"id"`
	Title          string  `json:"title"`
	Cost           float64 `json:"cost"`
	Responsibility string  `json:"responsibility"`
}

// Input 月度结算输入
type Input struct {
	BuildingID   int64
	PeriodMonth  string      `json:"period_month"`
	TotalKwh     float64     `json:"total_kwh"`
	GridPrice    float64     `json:"grid_price"`   // 上网电价 元/kWh
	SubsidyRate  float64     `json:"subsidy_rate"` // 度电补贴 元/kWh
	OmFee        float64     `json:"om_fee"`       // 当期运维费(元)
	Tax          float64     `json:"tax"`          // 当期税费(元)
	FundRatio    float64     `json:"fund_ratio"`   // 公共净收益划入维修基金比例 %
	FundBalance  float64     `json:"fund_balance"` // 维修基金当前余额
	Compensation float64     `json:"compensation"` // 安装商/保险当期赔偿
	Shareholders []Share     `json:"shareholders"`
	WorkOrders   []WorkOrder `json:"work_orders"`
}

// Item 结算明细行
type Item struct {
	UserID   *int64  `json:"user_id"`
	ItemType string  `json:"item_type"` // shareholder / public / maintenance_fund
	Name     string  `json:"name"`
	Ratio    float64 `json:"ratio"`
	Kwh      float64 `json:"kwh"`
	Revenue  float64 `json:"revenue"`
	Subsidy  float64 `json:"subsidy"`
	OmShare  float64 `json:"om_share"`
	TaxShare float64 `json:"tax_share"`
	WoShare  float64 `json:"wo_share"`
	Amount   float64 `json:"amount"`
	Memo     string  `json:"memo"`
}

// Result 结算结果
type Result struct {
	PeriodMonth       string  `json:"period_month"`
	TotalKwh          float64 `json:"total_kwh"`
	GrossRevenue      float64 `json:"gross_revenue"`
	TotalSubsidy      float64 `json:"total_subsidy"`
	OmFee             float64 `json:"om_fee"`
	Tax               float64 `json:"tax"`
	WorkOrderCost     float64 `json:"work_order_cost"` // property/owner 责任的维修费用合计
	Compensation      float64 `json:"compensation"`
	FundUsed          float64 `json:"fund_used"`     // 维修基金动用金额
	RepairDeduct      float64 `json:"repair_deduct"` // 维修净支出（按组件比例分摊）
	ShareholderRatio  float64 `json:"shareholder_ratio"`
	PublicRatio       float64 `json:"public_ratio"`
	FundRatio         float64 `json:"fund_ratio"`
	FundAddition      float64 `json:"fund_addition"` // 本期公共收益计提维修基金
	PublicDistributab float64 `json:"public_distributable"`
	Distributable     float64 `json:"distributable"` // 当期可分配净额合计
	CarbonKg          float64 `json:"carbon_kg"`
	Items             []Item  `json:"items"`
}

func round2(x float64) float64 { return math.Round(x*100) / 100 }

// Compute 根据输入生成完整月度结算方案。
//
// 分摊规则：
//  1. 发电总收入 = 发电量×上网电价；补贴 = 发电量×度电补贴；
//  2. 运维费、税费按组件比例在认购住户与公共部分之间分摊；
//  3. 维修费用中 property/owner 责任部分先由维修基金支付，基金不足部分按组件比例分摊；
//     installer/insurance 责任由对方赔偿，不冲减住户收益（赔偿计入公共账）；
//  4. 公共部分净收益按 FundRatio 计提维修基金，其余为公共可支配收益。
func Compute(in Input) Result {
	res := Result{
		PeriodMonth:  in.PeriodMonth,
		TotalKwh:     round2(in.TotalKwh),
		OmFee:        round2(in.OmFee),
		Tax:          round2(in.Tax),
		FundRatio:    in.FundRatio,
		Compensation: round2(in.Compensation),
	}
	res.GrossRevenue = round2(in.TotalKwh * in.GridPrice)
	res.TotalSubsidy = round2(in.TotalKwh * in.SubsidyRate)
	res.CarbonKg = round2(in.TotalKwh * CarbonFactorKgPerKwh)

	var sRatio float64
	for _, s := range in.Shareholders {
		sRatio += s.Ratio
	}
	res.ShareholderRatio = round2(sRatio)
	res.PublicRatio = round2(100 - sRatio)

	var woCost float64
	for _, w := range in.WorkOrders {
		if w.Responsibility == "property" || w.Responsibility == "owner" {
			woCost += w.Cost
		}
	}
	res.WorkOrderCost = round2(woCost)
	repairGross := math.Max(0, woCost-in.Compensation)
	res.FundUsed = round2(math.Min(in.FundBalance, repairGross))
	res.RepairDeduct = round2(repairGross - res.FundUsed)

	// 按比例为任意一方（住户或公共）生成分摊
	party := func(ratio float64) Item {
		r := ratio / 100
		return Item{
			Ratio:    round2(ratio),
			Kwh:      round2(in.TotalKwh * r),
			Revenue:  round2(res.GrossRevenue * r),
			Subsidy:  round2(res.TotalSubsidy * r),
			OmShare:  round2(res.OmFee * r),
			TaxShare: round2(res.Tax * r),
			WoShare:  round2(res.RepairDeduct * r),
		}
	}

	for _, s := range in.Shareholders {
		it := party(s.Ratio)
		uid := s.UserID
		it.UserID = &uid
		it.ItemType = "shareholder"
		it.Name = s.Name
		it.Amount = round2(it.Revenue + it.Subsidy - it.OmShare - it.TaxShare - it.WoShare)
		res.Items = append(res.Items, it)
	}

	pub := party(res.PublicRatio)
	pub.ItemType = "public"
	pub.Name = "公共账户"
	pub.Amount = round2(pub.Revenue + pub.Subsidy - pub.OmShare - pub.TaxShare - pub.WoShare)
	res.Items = append(res.Items, pub)

	if pub.Amount > 0 && in.FundRatio > 0 {
		res.FundAddition = round2(pub.Amount * in.FundRatio / 100)
	}
	fund := Item{
		ItemType: "maintenance_fund",
		Name:     "维修基金（公共收益计提/动用）",
		Amount:   round2(res.FundAddition - res.FundUsed),
		Memo:     "正数为计提，负数为动用",
	}
	res.Items = append(res.Items, fund)

	res.PublicDistributab = round2(pub.Amount - res.FundAddition)
	var dist float64
	for _, it := range res.Items {
		if it.ItemType == "shareholder" {
			dist += it.Amount
		}
	}
	dist += res.PublicDistributab
	res.Distributable = round2(dist)
	return res
}
