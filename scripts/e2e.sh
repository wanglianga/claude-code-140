#!/usr/bin/env bash
# 端到端业务闭环验证
set -u
BASE="http://localhost:8080"
PASS=0; FAIL=0
ok(){ PASS=$((PASS+1)); echo "✅ $*"; }
no(){ FAIL=$((FAIL+1)); echo "❌ $*"; }
# req METHOD PATH TOKEN JSON  -> /tmp/resp.json, HTTP 码存 $CODE
req(){
  local method="$1" path="$2" token="${3:-}" body="${4:-}"
  if [ -n "$body" ]; then
    CODE=$(curl -s -o /tmp/resp.json -w '%{http_code}' -X "$method" "$BASE$path" \
      -H 'Content-Type: application/json' ${token:+-H "Authorization: Bearer $token"} -d "$body")
  else
    CODE=$(curl -s -o /tmp/resp.json -w '%{http_code}' -X "$method" "$BASE$path" \
      ${token:+-H "Authorization: Bearer $token"})
  fi
}
jqr(){ jq -r "$1" /tmp/resp.json; }
# assertEq 描述 期望 实际
eq(){ if [ "$2" == "$3" ]; then ok "$1 = $2"; else no "$1 期望[$2] 实际[$3]"; fi; }

echo "===== 0. 鉴权拦截 ====="
req GET /api/buildings
eq "未登录返回401" 401 "$CODE"

echo "===== 1. 登录 ====="
req POST /api/login "" '{"as":"ADMIN","identifier":"admin","password":"admin123"}'
eq "管理员登录" 200 "$CODE"; ATOK=$(jqr .token)
req POST /api/login "" '{"as":"ADMIN","identifier":"admin","password":"bad"}'
eq "错误密码被拒" 401 "$CODE"

login(){ req POST /api/login "" "{\"as\":\"RESIDENT\",\"identifier\":\"$1\",\"password\":\"123456\",\"building_id\":1}"; }
declare -A TOK RID
for n in 王物业 李建国 张敏 陈晓东 刘洋 赵芳 孙志强 周丽华 吴海涛 郑海燕 钱进 冯雪; do
  login "$n"; [ "$CODE" = 200 ] && ok "住户登录:$n" || no "住户登录失败:$n $(cat /tmp/resp.json)"
  TOK[$n]=$(jqr .token)
done
req GET /api/buildings/1/residents "$ATOK"
for row in $(jq -c '.[]' /tmp/resp.json); do
  RID[$(echo "$row" | jq -r .owner_name)]=$(echo "$row" | jq -r .id)
done
echo "住户ID:${RID[李建国]} ${RID[钱进]} ${RID[冯雪]}"

echo "===== 2. 楼栋表决 ====="
# 9户赞成(含顶楼钱进,附漏水意见), 冯雪/郑海航等不投; 王物业不投
i=0
for n in 李建国 张敏 陈晓东 刘洋 赵芳 孙志强 周丽华 吴海涛; do
  req POST /api/buildings/1/votes "${TOK[$n]}" '{"vote":"APPROVE","opinion":"同意建设"}'
  i=$((i+1))
done
req POST /api/buildings/1/votes "${TOK[钱进]}" '{"vote":"APPROVE","opinion":"顶楼住户,要求施工方出具5年防水保修,保留检修通道"}'
eq "9户投票完成" 200 "$CODE"
req GET /api/buildings/1/votes "$ATOK"
AP=$(jqr .tally.approve); HP=$(jqr .tally.household_pct); APCT=$(jqr .tally.area_pct); PASSED=$(jqr .tally.passed)
eq "赞成户数" 9 "$AP"
eq "户数赞成率" 75 "$HP"
eq "面积赞成率" 75.02 "$APCT"
eq "表决通过" true "$PASSED"
# 重复投票(改票)
req POST /api/buildings/1/votes "${TOK[张敏]}" '{"vote":"REJECT","opinion":"再想想"}'
req GET /api/buildings/1/votes "$ATOK"; AP=$(jqr .tally.approve)
eq "改票后赞成户数" 8 "$AP"
req POST /api/buildings/1/stage "$ATOK" '{"action":"NEXT"}'
eq "8户(66.7%)未达门槛拒绝推进" 400 "$CODE"
req POST /api/buildings/1/votes "${TOK[张敏]}" '{"vote":"APPROVE","opinion":"看过防水方案后同意"}'

echo "===== 3. 推进到认购缴费 ====="
req POST /api/buildings/1/stage "$ATOK" '{"action":"NEXT"}'
eq "VOTING->SUBSCRIBED" 200 "$CODE"; eq "当前阶段" SUBSCRIBED "$(jqr .stage)"

echo "===== 4. 认购与缴费 ====="
sub(){ req POST /api/buildings/1/subscriptions "$ATOK" "{\"resident_id\":${RID[$1]},\"mode\":\"SUBSCRIBE\",\"component_qty\":$2}"; }
sub 李建国 40; eq "李建国认购40" 200 "$CODE"
sub 张敏 30; sub 陈晓东 20; sub 刘洋 20; sub 赵芳 20; sub 钱进 30
eq "6户累计认购160" 200 "$CODE"
sub 李建国 5; eq "重复认购被拒" 400 "$CODE"
# 冯雪选择只参与公共收益分配(不出资)
req POST /api/buildings/1/subscriptions "$ATOK" "{\"resident_id\":${RID[冯雪]},\"mode\":\"PUBLIC_ONLY\",\"component_qty\":0}"
eq "冯雪公共分配模式" 200 "$CODE"; eq "公共模式直接PAID" PAID "$(jqr .status)"
# 超额
req POST /api/buildings/1/subscriptions "$ATOK" "{\"resident_id\":${RID[郑海燕]},\"mode\":\"SUBSCRIBE\",\"component_qty\":999}"
eq "超额认购被拒" 400 "$CODE"
req GET /api/buildings/1/subscriptions "$ATOK"
eq "认购汇总已认购" 160 "$(jqr .subscribed_qty)"
eq "公共剩余组件" 40 "$(jqr .public_qty)"
# 缴费:查出 pending 认购 id, 物业代缴
PENDING=$(jq -r '.subscriptions[]|select(.status=="PENDING")|.id' /tmp/resp.json)
for sid in $PENDING; do
  req POST /api/subscriptions/$sid/pay "${TOK[王物业]}" '{}'
  [ "$CODE" = 200 ] || no "缴费失败 sid=$sid"
done
ok "全部待缴费认购完成缴费"
req GET /api/buildings/1/subscriptions "$ATOK"
eq "无待缴费" 0 "$(jq '[.subscriptions[]|select(.status=="PENDING")]|length' /tmp/resp.json)"

echo "===== 5. 施工排期与设备 ====="
req POST /api/buildings/1/stage "$ATOK" '{"action":"NEXT"}'
eq "->SCHEDULED" 200 "$CODE"
req POST /api/buildings/1/schedule "$ATOK" '{"kind":"DELAY","title":"组件到货延迟2周","detail":"厂家排产紧张,安装窗口顺延,不影响并网承诺","planned_on":"2026-02-10","actual_on":""}'
req POST /api/buildings/1/schedule "$ATOK" '{"kind":"GRID_CHANGE","title":"电网并网时间提前至3月20日","detail":"配电室改造提前完工","planned_on":"2026-03-20","actual_on":""}'
eq "排期事件登记" 200 "$CODE"
req POST /api/buildings/1/devices "$ATOK" '{"code":"INV-01","name":"1号逆变器","device_type":"INVERTER","component_from":1,"component_to":100,"capacity_kw":50}'
req POST /api/buildings/1/devices "$ATOK" '{"code":"INV-02","name":"2号逆变器","device_type":"INVERTER","component_from":101,"component_to":200,"capacity_kw":50}'
eq "登记2台逆变器" 200 "$CODE"
req POST /api/buildings/1/stage "$ATOK" '{"action":"NEXT"}'
eq "->GRID_CONNECTED" 200 "$CODE"

echo "===== 6. 发电计量与异常解释 ====="
req GET /api/buildings/1/devices "$ATOK"
D1=$(jq -r '.[]|select(.code=="INV-01").id' /tmp/resp.json)
D2=$(jq -r '.[]|select(.code=="INV-02").id' /tmp/resp.json)
req POST /api/buildings/1/meters "$ATOK" "{\"device_id\":$D1,\"period\":\"2026-01\",\"reading_kwh\":5200,\"note\":\"\"}"
req POST /api/buildings/1/meters "$ATOK" "{\"device_id\":$D2,\"period\":\"2026-01\",\"reading_kwh\":4800,\"note\":\"下旬故障停机3天\"}"
eq "录入1月电量10000" 200 "$CODE"
req GET /api/buildings/1 "$ATOK"; eq "首条读数自动进入运营" OPERATING "$(jqr .stage)"
req POST /api/buildings/1/meters "$ATOK" "{\"device_id\":$D1,\"period\":\"2026-01\",\"reading_kwh\":5200,\"note\":\"覆盖测试\"}"
eq "同周期读数覆盖" 200 "$CODE"
req POST /api/buildings/1/generation-notes "$ATOK" '{"period":"2026-01","title":"1月下旬停机说明","explanation":"INV-02因IGBT模块故障停机3天,厂家免费更换,少发电量已纳入维修责任判定。"}'
eq "异常解释发布" 200 "$CODE"
# 2月正常数据(年报用)
req POST /api/buildings/1/meters "$ATOK" "{\"device_id\":$D1,\"period\":\"2026-02\",\"reading_kwh\":6100,\"note\":\"\"}"
req POST /api/buildings/1/meters "$ATOK" "{\"device_id\":$D2,\"period\":\"2026-02\",\"reading_kwh\":5900,\"note\":\"\"}"

echo "===== 7. 维修工单与责任判定 ====="
req POST /api/buildings/1/repairs "${TOK[钱进]}" '{"device_id":0,"category":"LEAK","title":"601阳台顶角渗水","description":"大雨后顶角出现水渍,怀疑施工破坏防水"}'
eq "顶楼住户报修漏水" 200 "$CODE"
WX1=$(jqr .id)
req POST /api/buildings/1/repairs "$ATOK" "{\"device_id\":$D2,\"category\":\"INVERTER_FAULT\",\"title\":\"2号逆变器停机\",\"description\":\"故障停机3天\"}"
WX2=$(jqr .id)
# 漏水:物业承担1500,影响1月
req POST /api/repairs/$WX1/resolve "$ATOK" '{"responsibility":"PROPERTY","cost":1500,"affect_period":"2026-01","affected_kwh":0,"deductible_share":0,"resolution":"重做局部防水层,物业公共账户列支"}'
eq "漏水工单处理" 200 "$CODE"
# 逆变器:供应商全责,少发500度,100%赔偿=500*0.44=220
req POST /api/repairs/$WX2/resolve "$ATOK" '{"responsibility":"SUPPLIER","cost":8000,"affect_period":"2026-01","affected_kwh":500,"deductible_share":100,"resolution":"厂家免费更换IGBT并赔偿少发电量损失"}'
eq "逆变器工单处理" 200 "$CODE"
req GET /api/buildings/1/repairs "$ATOK"
eq "赔偿金额计算" 220 "$(jq -r ".[]|select(.id==$WX2)|.deduction_amount" /tmp/resp.json)"
req GET /api/buildings/1/devices "$ATOK"
eq "关联逆变器标记已修复" REPAIRED "$(jq -r ".[]|select(.id==$D2)|.status" /tmp/resp.json)"
req POST /api/repairs/$WX2/resolve "$ATOK" '{"responsibility":"SUPPLIER","cost":1}'
eq "已处理工单不可重复处理" 400 "$CODE"

echo "===== 8. 月度结算 ====="
req POST /api/buildings/1/settlements "$ATOK" '{"period":"2026-01","note":"首月结算"}'
eq "生成1月草稿" 200 "$CODE"
eq "总电量" 10000 "$(jqr .calc.total_kwh)"
eq "发电收入" 3900 "$(jqr .calc.gross)"
eq "补贴" 500 "$(jqr .calc.subsidy)"
eq "运维费" 220 "$(jqr .calc.maintenance_fee)"
eq "税费" 125.4 "$(jqr .calc.tax)"
eq "维修基金" 132 "$(jqr .calc.repair_fund)"
eq "公共维修支出" 1500 "$(jqr .calc.repair_cost)"
eq "责任赔偿" 220 "$(jqr .calc.deduction)"
eq "可分净额" 4142.6 "$(jqr .calc.net)"
SID=$(jqr .id)
# 2月草稿+确认(年报多月)
req POST /api/buildings/1/settlements "$ATOK" '{"period":"2026-02","note":""}'
# 确认1月
req POST /api/settlements/$SID/confirm "$ATOK" '{}'
eq "1月结算确认入账" 200 "$CODE"
eq "住户收益发放总额" 2651.27 "$(jqr .resident_payout)"
req POST /api/buildings/1/settlements "$ATOK" '{"period":"2026-01"}'
eq "已确认结算不可重算" 400 "$CODE"
# 确认2月
req GET /api/buildings/1/settlements "$ATOK"
SID2=$(jq -r '.[]|select(.period=="2026-02").id' /tmp/resp.json)
req POST /api/settlements/$SID2/confirm "$ATOK" '{}'
eq "2月结算确认" 200 "$CODE"
# 明细核对:李建国20%份额 1月住户金额=4142.6*0.2*0.8=662.82
LI=$(jq -r '.[]|select(.period=="2026-01").items[]|select(.resident_id=='${RID[李建国]}').amount' /tmp/resp.json 2>/dev/null)
# 上一行响应已被确认2月覆盖,重新取
req GET /api/buildings/1/settlements "$ATOK"
LI=$(jq -r '.[]|select(.period=="2026-01").items[]|select(.resident_id=='${RID[李建国]}').amount' /tmp/resp.json)
eq "李建国1月收益(20%份额)" 662.82 "$LI"
PUB=$(jq -r '.[]|select(.period=="2026-01").items[]|select(.kind=="PUBLIC").amount' /tmp/resp.json)
eq "公共部分1月收益" 1491.33 "$PUB"

echo "===== 9. 公共账户 ====="
req GET /api/buildings/1/public-transactions "$ATOK"
eq "账户流水返回200" 200 "$CODE"
BAL=$(jqr .balance)
echo "  公共账户余额:$BAL"
eq "含维修出账" REPAIR "$(jq -r '[.transactions[]|select(.type=="REPAIR")]|length|if .>0 then "REPAIR" else "NONE" end' /tmp/resp.json)"
eq "含赔偿入账" COMPENSATION "$(jq -r '[.transactions[]|select(.type=="COMPENSATION")]|length|if .>0 then "COMPENSATION" else "NONE" end' /tmp/resp.json)"
eq "维修基金累计" 290.4 "$(jqr .repair_fund_total)"

echo "===== 10. 异议与投票 ====="
req POST /api/buildings/1/objections "${TOK[冯雪]}" '{"title":"对收益测算口径的疑问","content":"要求公开运维费明细与维修基金使用规则"}'
eq "住户提出异议" 200 "$CODE"
OID=$(jqr .id)
req POST /api/objections/$OID/vote "${TOK[钱进]}" '{"vote":"APPROVE"}'
eq "未开始投票拒绝" 400 "$CODE"
req POST /api/objections/$OID/vote-start "$ATOK" '{}'
eq "发起异议投票" 200 "$CODE"
for n in 钱进 李建国 张敏 陈晓东 刘洋 赵芳 周丽华; do
  req POST /api/objections/$OID/vote "${TOK[$n]}" '{"vote":"APPROVE"}'
done
req POST /api/objections/$OID/vote "${TOK[吴海涛]}" '{"vote":"REJECT"}'
req POST /api/objections/$OID/close "$ATOK" '{}'
eq "7/12赞成 异议成立" APPROVED "$(jqr .result)"
req POST /api/buildings/1/announcements "$ATOK" '{"title":"关于运维费与维修基金细则的公示","content":"接受业主异议,现公示明细……"}'
eq "公告发布" 200 "$CODE"

echo "===== 11. 房屋转让 ====="
# 李建国(40组件)房子卖给马磊,权益随房转移+结清历史
req POST /api/buildings/1/transfers "$ATOK" "{\"resident_id\":${RID[李建国]},\"new_owner_name\":\"马磊\",\"new_phone\":\"13900000099\",\"transfer_subscription\":true,\"settle_history\":true,\"remark\":\"二手房买卖\"}"
T1=$(jqr .id); eq "登记转让(随房转移)" 200 "$CODE"
req POST /api/transfers/$T1/process "$ATOK" '{}'
eq "执行随房转移" 200 "$CODE"
# 李建国1月+2月收益:1月662.82;2月净额:12000kWh→gross4680+sub600-maint264-tax150.48-fund158.4=4707.12;*0.2*0.8=753.14(2月无维修) → 合计1415.96
SETTLED=$(jqr .settled_amount); echo "  历史结清金额:$SETTLED"
req POST /api/login "" '{"as":"RESIDENT","identifier":"李建国","password":"123456","building_id":1}'
eq "原业主无法登录(记录已改名)" 401 "$CODE"
req POST /api/login "" '{"as":"RESIDENT","identifier":"马磊","password":"123456","building_id":1}'
eq "新业主可登录(权益承继)" 200 "$CODE"
# 赵芳20块 不随房转移→退本金24000
req POST /api/buildings/1/transfers "$ATOK" "{\"resident_id\":${RID[赵芳]},\"new_owner_name\":\"许晴\",\"new_phone\":\"13900000100\",\"transfer_subscription\":false,\"settle_history\":false,\"remark\":\"\"}"
T2=$(jqr .id)
req POST /api/transfers/$T2/process "$ATOK" '{}'
eq "不随房转移执行" 200 "$CODE"
eq "退还认购本金" 24000 "$(jqr .refund_amount)"

echo "===== 12. 年度报告 ====="
req GET "/api/buildings/1/annual-report?year=2026" "$ATOK"
eq "年报请求" 200 "$CODE"
eq "年报总电量" 22000 "$(jqr .summary.total_kwh)"
eq "年报碳减排kg" 12782 "$(jqr .summary.carbon_kg)"
eq "年报维修单数" 2 "$(jqr .summary.repair_orders)"
eq "年报已解决" 2 "$(jqr .summary.repair_resolved)"
eq "年报住户榜含新业主马磊" 1 "$(jq '[.resident_rank[]|select(.owner_name=="马磊")]|length' /tmp/resp.json)"
eq "年报含赵芳退款前收益记录" 1 "$(jq '[.resident_rank[]|select(.owner_name=="赵芳")]|length' /tmp/resp.json)"

echo "===== 13. 总览仪表盘 ====="
req GET /api/dashboard "$ATOK"
eq "仪表盘200" 200 "$CODE"
eq "楼栋数" 1 "$(jqr .building_count)"
echo "  累计发电:$(jqr .total_kwh) kWh / 累计净额:¥$(jqr .total_income) / 公共余额:¥$(jqr .public_balance) / 碳减排:$(jqr .carbon_t) t"

echo
echo "=================================="
echo "通过:$PASS  失败:$FAIL"
echo "=================================="
[ "$FAIL" = 0 ]
