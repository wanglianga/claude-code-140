package main

import "net/http"

// 演示项目密码明文存储(种子数据中给定),登录校验密码相等。

func handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		As         string `json:"as"` // ADMIN / RESIDENT
		Identifier string `json:"identifier"`
		Password   string `json:"password"`
		BuildingID int64  `json:"building_id"`
	}
	if err := decode(r, &req); err != nil {
		badRequest(w, err)
		return
	}
	if req.As == "ADMIN" {
		var id int64
		var pwd, name string
		err := db.QueryRow(`SELECT id, password, name FROM admins WHERE username=$1`, req.Identifier).
			Scan(&id, &pwd, &name)
		if err != nil || pwd != req.Password {
			fail(w, 401, "管理员账号或密码错误")
			return
		}
		tok := saveSession(&session{Kind: "ADMIN", AdminID: id, AdminName: name, Name: name, Role: "ADMIN"})
		writeOK(w, map[string]interface{}{
			"token": tok,
			"user":  map[string]string{"kind": "ADMIN", "name": name},
		})
		return
	}

	var id, bid int64
	var pwd, name, role, unit string
	var active bool
	err := db.QueryRow(`SELECT id, building_id, owner_name, role, unit_no, active, COALESCE(password,'')
		FROM residents WHERE owner_name=$1 AND COALESCE(building_id,0)=COALESCE(NULLIF($2,0), building_id)`,
		req.Identifier, req.BuildingID).Scan(&id, &bid, &name, &role, &unit, &active, &pwd)
	// 同名时可能不确定，改用更精确查询
	if err != nil {
		err = db.QueryRow(`SELECT r.id, r.building_id, r.owner_name, r.role, r.unit_no, r.active, COALESCE(r.password,'')
			FROM residents r
			WHERE (r.owner_name=$1 OR r.unit_no=$1) AND ($2=0 OR r.building_id=$2)
			ORDER BY r.id LIMIT 1`, req.Identifier, req.BuildingID).
			Scan(&id, &bid, &name, &role, &unit, &active, &pwd)
	}
	if err != nil || pwd != req.Password {
		fail(w, 401, "住户姓名/房号或密码错误")
		return
	}
	if !active {
		fail(w, 403, "该住户已因房屋转让停用,请使用新业主账号")
		return
	}
	tok := saveSession(&session{Kind: "RESIDENT", ResidentID: id, BuildingID: bid, Name: name, Role: role})
	writeOK(w, map[string]interface{}{
		"token": tok,
		"user": map[string]interface{}{
			"kind": "RESIDENT", "resident_id": id, "name": name, "role": role,
			"building_id": bid, "unit_no": unit,
		},
	})
}

// requireAdmin 仅平台管理员
func requireAdmin(w http.ResponseWriter, r *http.Request) (*session, bool) {
	s := curUser(r)
	if s.Kind != "ADMIN" {
		fail(w, 403, "需要平台管理员权限")
		return nil, false
	}
	return s, true
}

// canManage: 管理员 或 该楼物业
func canManage(w http.ResponseWriter, r *http.Request, buildingID int64) (*session, bool) {
	s := curUser(r)
	if s.Kind == "ADMIN" {
		return s, true
	}
	if s.Kind == "RESIDENT" && s.BuildingID == buildingID && s.Role == "PROPERTY" {
		return s, true
	}
	fail(w, 403, "无权操作该楼栋(需管理员或本楼物业)")
	return nil, false
}

// residentOf: 该楼登录住户(含物业账号)
func residentOf(w http.ResponseWriter, r *http.Request, buildingID int64) (*session, bool) {
	s := curUser(r)
	if s.Kind == "ADMIN" || (s.Kind == "RESIDENT" && s.BuildingID == buildingID) {
		return s, true
	}
	fail(w, 403, "只能操作本楼栋数据")
	return nil, false
}
