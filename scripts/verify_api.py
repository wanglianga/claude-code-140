#!/usr/bin/env python3
"""端到端 API 验证脚本（依赖 python3，无第三方包）。

用法：
    python3 scripts/verify_api.py [BASE_URL]

前置：服务已启动且数据库为初始种子状态（SEED_ON_START=true 首次启动后的干净库）。
脚本会校验：驾驶舱 → 9 月结算（维修基金动用+分摊）→ 定稿入账 → 房屋转让 →
次月分成归属切换 → 公告/异议/异常解释/年报 → 阶段回退原因强制。
任一断言失败立即退出（exit 1）。
"""
import json
import sys
import urllib.request
import urllib.error

BASE = sys.argv[1] if len(sys.argv) > 1 else "http://127.0.0.1:8080"


def call(method, path, token=None, body=None):
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(BASE + path, data=data, method=method)
    req.add_header("Content-Type", "application/json")
    if token:
        req.add_header("Authorization", "Bearer " + token)
    try:
        with urllib.request.urlopen(req) as r:
            raw = r.read().decode()
            return r.status, json.loads(raw) if raw else {}
    except urllib.error.HTTPError as e:
        raw = e.read().decode()
        try:
            return e.code, json.loads(raw)
        except Exception:
            return e.code, {"raw": raw}


def check(name, cond, detail=""):
    if not cond:
        print(f"FAIL: {name} {detail}")
        sys.exit(1)
    print(f"PASS: {name} {detail}")


def approx(a, b, eps=0.01):
    return abs(float(a) - float(b)) < eps


def login(phone, pwd):
    st, r = call("POST", "/api/auth/login", body={"phone": phone, "password": pwd})
    check(f"login {phone}", st == 200)
    return r["token"]


def main():
    prop = login("13800000001", "property123")
    zhang = login("13900000001", "resident123")
    liu = login("13900000006", "resident123")

    st, r = call("GET", "/api/buildings", prop)
    b1 = next(b for b in r["items"] if "1 号楼" in b["name"])
    bid = b1["id"]
    print(f"-- 目标楼栋：{b1['name']} (id={bid})，阶段 {b1['stage']}")

    # A. 8 月已定稿的驾驶舱
    st, d = call("GET", f"/api/buildings/{bid}/dashboard", prop)
    check("8月后公共账户余额=85631.00", approx(d["public_balance"], 85631.00), str(d["public_balance"]))
    check("维修基金余额=97.86", approx(d["maintenance_fund_balance"], 97.86), str(d["maintenance_fund_balance"]))
    check("8月累计发电4150", approx(d["total_kwh"], 4150))
    check("碳减排2411.15kg", approx(d["carbon_kg"], 2411.15))

    # B. 9 月试算：2600kWh，漏水 600 物业责任，基金期初 97.86
    st, r = call("POST", f"/api/buildings/{bid}/settlements", prop, {
        "period_month": "2026-09", "grid_price": 0.4153, "subsidy_per_kwh": 0.05,
        "om_fee": 200, "tax": 120,
    })
    s = r["settlement"]
    check("9月动用基金97.86", approx(s["fund_used"], 97.86), str(s["fund_used"]))
    check("9月维修分摊502.14", approx(s["repair_deduct"], 502.14), str(s["repair_deduct"]))
    check("9月计提基金23.26", approx(s["fund_addition"], 23.26), str(s["fund_addition"]))
    zhang_item = next(i for i in s["items"] if i["item_type"] == "shareholder" and i["name"].startswith("张伟"))
    check("张伟9月分成116.29", approx(zhang_item["amount"], 116.29), str(zhang_item["amount"]))
    spid = r["id"]

    # C. 定稿
    st, _ = call("POST", f"/api/settlements/{spid}/finalize", prop)
    check("9月结算定稿", st == 200)
    st, _ = call("POST", f"/api/buildings/{bid}/settlements", prop,
                 {"period_month": "2026-09", "grid_price": 0.5})
    check("定稿后禁止重算(409)", st == 409, str(st))
    st, me = call("GET", "/api/me", zhang)
    check("张伟余额=8月489.30+9月116.29=605.59", approx(me["balance"], 605.59), str(me["balance"]))
    st, lg = call("GET", f"/api/buildings/{bid}/ledger", prop)
    check("9月后公共账户=85920.78", approx(lg["public_balance"], 85920.78), str(lg["public_balance"]))
    check("9月后维修基金=23.26", approx(lg["maintenance_fund_balance"], 23.26), str(lg["maintenance_fund_balance"]))

    # D. 房屋转让：张伟 -> 刘洋
    st, subs = call("GET", f"/api/buildings/{bid}/subscriptions", prop)
    sid = next(x["id"] for x in subs["items"] if x["name"].startswith("张伟") and x["type"] == "shareholder")
    st, users = call("GET", "/api/users?q=13900000006", prop)
    liu_id = next(u["id"] for u in users["items"] if u["phone"] == "13900000006")
    st, tr = call("POST", f"/api/buildings/{bid}/transfers", zhang, {
        "subscription_id": sid, "to_user_id": liu_id, "rights_transfer": True,
    })
    check("发起转让", st == 201 and tr["history_settled"] is True, str(tr))
    tid = tr["id"]
    st, _ = call("POST", f"/api/transfers/{tid}/confirm", prop)
    check("物业确认转让", st == 200)

    # E. 10 月发电与结算：30% 份额应归刘洋
    st, invs = call("GET", f"/api/buildings/{bid}/inverters", prop)
    for iv in invs["items"]:
        st, _ = call("POST", f"/api/buildings/{bid}/generation", prop, {
            "period_month": "2026-10", "inverter_id": iv["id"], "kwh": 1200,
        })
        check(f"录入{iv['name']}10月电量", st == 200)
    st, r = call("POST", f"/api/buildings/{bid}/settlements", prop, {
        "period_month": "2026-10", "grid_price": 0.4153, "subsidy_per_kwh": 0.05,
    })
    check("10月结算生成", st == 200, str(r)[:200])
    octid = r["id"]
    st, detail = call("GET", f"/api/settlements/{octid}", prop)
    holders = {i["name"]: i["amount"] for i in detail["items"] if i["item_type"] == "shareholder"}
    check("张伟已无10月分成", not any(k.startswith("张伟") for k in holders), str(holders))
    check("刘洋(新业主)获30%分成335.02", approx(holders.get("刘洋(新业主)", 0), 335.02), str(holders))
    call("POST", f"/api/settlements/{octid}/finalize", prop)
    st, me = call("GET", "/api/me", liu)
    check("刘洋余额=335.02", approx(me["balance"], 335.02), str(me["balance"]))
    st, me = call("GET", "/api/me", zhang)
    check("张伟余额保持605.59（历史收益不转移）", approx(me["balance"], 605.59), str(me["balance"]))

    # F. 公告 / 异议 / 异常解释 / 年报
    st, _ = call("POST", f"/api/buildings/{bid}/announcements", prop,
                 {"title": "10月公示", "content": "见结算", "pinned": True})
    check("发布公告", st == 201)
    st, poll = call("POST", f"/api/buildings/{bid}/polls", zhang,
                    {"title": "验证用异议", "content": "测试"})
    pid = poll["id"]
    call("POST", f"/api/polls/{pid}/vote", zhang, {"support": True})
    call("POST", f"/api/polls/{pid}/vote", liu, {"support": True})
    st, r = call("POST", f"/api/polls/{pid}/close", prop)
    check("异议投票成立", r["result"] == "approved", str(r))
    st, al = call("POST", f"/api/buildings/{bid}/alerts", zhang,
                  {"period_month": "2026-10", "reason": "脚本验证异常"})
    aid = al["id"]
    st, _ = call("POST", f"/api/alerts/{aid}/explain", prop, {"explanation": "脚本验证：计划检修 1 天"})
    check("异常解释", st == 200)
    st, r = call("POST", f"/api/buildings/{bid}/annual-reports", prop, {"year": 2026})
    check("年度报告生成（3个定稿月份）", st == 200 and r["summary"]["finalized_months"] == 3, str(st))
    rid = r["id"]
    st, _ = call("POST", f"/api/annual-reports/{rid}/publish", prop)
    check("年报发布", st == 200)

    # G. 阶段回退必须有原因
    st, _ = call("POST", f"/api/buildings/{bid}/stage", prop, {"to_stage": "construction"})
    check("无原因回退被拒(409)", st == 409, str(st))
    st, r = call("POST", f"/api/buildings/{bid}/stage", prop,
                 {"to_stage": "construction", "reason": "设备供应延迟，组件到货推迟"})
    check("有原因回退成功", st == 200, str(r))

    print("\n全部端到端断言通过 ✅")


if __name__ == "__main__":
    main()
