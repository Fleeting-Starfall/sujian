package handler

import (
	"encoding/json"
	"testing"
)

// TestChangePassword 用户自助修改密码全链路：
// 注册 → 管理员审核通过 → 旧密码登录 → 各种非法输入被拒 → 正确修改成功 →
// 旧会话失效（被踢）→ 新密码可登录、旧密码被拒。
func TestChangePassword(t *testing.T) {
	app, cleanup := newTestApp(t)
	defer cleanup()

	// 1) 注册一个普通用户并让管理员审核通过
	r := app.do("POST", "/api/register", "", map[string]string{"username": "pwtester", "nickname": "PW", "password": "oldpass1"})
	if r.Status != 200 {
		t.Fatalf("注册失败 %d: %v", r.Status, r.Body)
	}
	adminTok, _ := app.login(t, "admin", "admin123")
	var uid string
	r = app.do("GET", "/api/admin/users", adminTok, nil)
	if r.Status != 200 {
		t.Fatalf("admin users %d: %v", r.Status, r.Body)
	}
	var list []map[string]interface{}
	if err := json.Unmarshal(r.Raw, &list); err != nil {
		t.Fatalf("解析 admin users 失败: %v (raw=%s)", err, string(r.Raw))
	}
	found := false
	for _, um := range list {
		if um["username"] == "pwtester" {
			uid, _ = um["id"].(string)
			found = true
			break
		}
	}
	if !found || uid == "" {
		t.Fatalf("未找到 pwtester: %v", r.Body)
	}
	r = app.do("POST", "/api/admin/users/"+uid+"/approve", adminTok, nil)
	if r.Status != 200 {
		t.Fatalf("审核通过失败 %d: %v", r.Status, r.Body)
	}

	// 2) 用旧密码登录拿 token
	oldTok, _ := app.login(t, "pwtester", "oldpass1")
	if oldTok == "" {
		t.Fatal("旧密码登录失败")
	}

	// 3) 未登录 → 401
	r = app.do("POST", "/api/me/password", "", map[string]string{"oldPassword": "oldpass1", "newPassword": "newpass2"})
	if r.Status != 401 {
		t.Fatalf("未登录应 401，实际 %d", r.Status)
	}

	// 4) 旧密码错误 → 400
	r = app.do("POST", "/api/me/password", oldTok, map[string]string{"oldPassword": "wrong-old", "newPassword": "newpass2"})
	if r.Status != 400 {
		t.Fatalf("旧密码错误应 400，实际 %d: %v", r.Status, r.Body)
	}

	// 5) 新密码过短 → 400
	r = app.do("POST", "/api/me/password", oldTok, map[string]string{"oldPassword": "oldpass1", "newPassword": "123"})
	if r.Status != 400 {
		t.Fatalf("短密码应 400，实际 %d: %v", r.Status, r.Body)
	}

	// 6) 新旧相同 → 400
	r = app.do("POST", "/api/me/password", oldTok, map[string]string{"oldPassword": "oldpass1", "newPassword": "oldpass1"})
	if r.Status != 400 {
		t.Fatalf("新旧相同应 400，实际 %d: %v", r.Status, r.Body)
	}

	// 7) 正确修改 → 200
	r = app.do("POST", "/api/me/password", oldTok, map[string]string{"oldPassword": "oldpass1", "newPassword": "newpass2"})
	if r.Status != 200 {
		t.Fatalf("正确修改应 200，实际 %d: %v", r.Status, r.Body)
	}

	// 8) 修改成功后旧会话被踢 → 旧 token 401
	r = app.do("GET", "/api/me", oldTok, nil)
	if r.Status != 401 {
		t.Fatalf("改密后旧 token 应 401，实际 %d", r.Status)
	}

	// 9) 新密码可登录、旧密码被拒
	newTok, _ := app.login(t, "pwtester", "newpass2")
	if newTok == "" {
		t.Fatal("新密码应能登录")
	}
	r = app.do("POST", "/api/login", "", map[string]string{"username": "pwtester", "password": "oldpass1"})
	if r.Status != 401 {
		t.Fatalf("旧密码应被拒 401，实际 %d", r.Status)
	}
}
