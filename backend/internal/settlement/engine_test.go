package settlement

import (
	"math"
	"testing"
)

func approx(a, b float64) bool { return math.Abs(a-b) < 0.01 }

func TestComputeBasicSplit(t *testing.T) {
	in := Input{
		PeriodMonth: "2026-09",
		TotalKwh:    1000,
		GridPrice:   0.4,
		SubsidyRate: 0.05,
		OmFee:       100,
		Tax:         50,
		FundRatio:   20,
		Shareholders: []Share{
			{UserID: 1, Name: "张三", Ratio: 60},
			{UserID: 2, Name: "李四", Ratio: 10},
		},
	}
	res := Compute(in)
	if !approx(res.GrossRevenue, 400) {
		t.Fatalf("gross=%v want 400", res.GrossRevenue)
	}
	if !approx(res.TotalSubsidy, 50) {
		t.Fatalf("subsidy=%v want 50", res.TotalSubsidy)
	}
	if res.ShareholderRatio != 70 || res.PublicRatio != 30 {
		t.Fatalf("ratios got %v/%v", res.ShareholderRatio, res.PublicRatio)
	}
	// 张三分摊: 收入 240 + 补贴 30 - 运维 60 - 税 30 = 180
	var zhang *Item
	for i := range res.Items {
		if res.Items[i].ItemType == "shareholder" && res.Items[i].UserID != nil && *res.Items[i].UserID == 1 {
			zhang = &res.Items[i]
		}
	}
	if zhang == nil {
		t.Fatal("张三明细缺失")
	}
	if !approx(zhang.Amount, 180) {
		t.Fatalf("张三 amount=%v want 180", zhang.Amount)
	}
	if !approx(zhang.OmShare, 60) || !approx(zhang.TaxShare, 30) {
		t.Fatalf("分摊错误 om=%v tax=%v", zhang.OmShare, zhang.TaxShare)
	}
	// 公共: 收入 120 + 补贴 15 - 运维 30 - 税 15 = 90
	pub := res.Items[len(res.Items)-2]
	if !approx(pub.Amount, 90) {
		t.Fatalf("public amount=%v want 90", pub.Amount)
	}
	// 维修基金计提 90*20%=18，公共可支配 72
	if !approx(res.FundAddition, 18) || !approx(res.PublicDistributab, 72) {
		t.Fatalf("fund=%v pubDist=%v", res.FundAddition, res.PublicDistributab)
	}
	// 碳减排
	if !approx(res.CarbonKg, 581) {
		t.Fatalf("carbon=%v want 581", res.CarbonKg)
	}
}

func TestWorkOrderFundAndResponsibility(t *testing.T) {
	// 基金余额 1000；物业责任维修 600，安装商责任 800（赔偿，不冲减住户收益）
	in := Input{
		TotalKwh: 1000, GridPrice: 0.4, SubsidyRate: 0,
		FundRatio:    20,
		FundBalance:  1000,
		Shareholders: []Share{{UserID: 1, Ratio: 100}},
		WorkOrders: []WorkOrder{
			{ID: 1, Title: "漏水", Cost: 600, Responsibility: "property"},
			{ID: 2, Title: "逆变器", Cost: 800, Responsibility: "installer"},
		},
	}
	res := Compute(in)
	if !approx(res.WorkOrderCost, 600) {
		t.Fatalf("woCost=%v want 600（安装商责任不计入）", res.WorkOrderCost)
	}
	// 600 全部由维修基金承担，住户分摊 0
	if !approx(res.FundUsed, 600) || !approx(res.RepairDeduct, 0) {
		t.Fatalf("fundUsed=%v deduct=%v", res.FundUsed, res.RepairDeduct)
	}
	holder := res.Items[0]
	if !approx(holder.Amount, 400) {
		t.Fatalf("holder amount=%v want 400（基金足额支付维修费）", holder.Amount)
	}

	// 基金不足场景
	in.FundBalance = 200
	res = Compute(in)
	if !approx(res.FundUsed, 200) || !approx(res.RepairDeduct, 400) {
		t.Fatalf("fundUsed=%v deduct=%v want 200/400", res.FundUsed, res.RepairDeduct)
	}
	holder = res.Items[0]
	if !approx(holder.Amount, 0) { // 400 收入 - 400 维修分摊
		t.Fatalf("holder amount=%v want 0", holder.Amount)
	}

	// 有赔偿时：property 责任 600，赔偿 300，净支出 300 走基金
	in.Compensation = 300
	in.FundBalance = 1000
	res = Compute(in)
	if !approx(res.RepairDeduct, 0) || !approx(res.FundUsed, 300) {
		t.Fatalf("赔偿后 fundUsed=%v deduct=%v want 300/0", res.FundUsed, res.RepairDeduct)
	}
}

func TestPublicOnlyRatio(t *testing.T) {
	res := Compute(Input{TotalKwh: 500, GridPrice: 0.4, FundRatio: 0})
	if res.PublicRatio != 100 {
		t.Fatalf("public ratio=%v want 100", res.PublicRatio)
	}
	if !approx(res.GrossRevenue, 200) || !approx(res.Distributable, 200) {
		t.Fatalf("金额错误 gross=%v dist=%v", res.GrossRevenue, res.Distributable)
	}
}
