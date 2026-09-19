package handler

import (
	"testing"
)

// TestAdminResetPasswordInvalidatesSessions 验证安全增强：
// 管理员重置密码后，该用户所有旧会话立即失效（旧 token 无法继续访问），
// 新密码可正常登录、旧密码被拒。
func TestAdminResetPasswordInvalidatesSessions(t *testing.T) {
	app, cleanup := newTestApp(t)
	defer cleanup()

	// 1) 管理员登录 + 批量创建 tester
	adminTok, _ := app.login(t, "admin", "admin123")
	batch := app.do("POST", "/api/admin/users/batch", adminTok, map[string]interface{}{
		"users": []map[string]string{{"username": "tester", "password": "oldpass1", "nickname": "测试员"}},
	})
	if batch.Status != 200 {
		t.Fatalf("批量创建失败 %d: %v", batch.Status, batch.Body)
	}
	created, _ := batch.Body["created"].([]interface{})
	if len(created) != 1 {
		t.Fatalf("应创建 1 个用户，实际 %d", len(created))
	}
	uid, _ := created[0].(map[string]interface{})["id"].(string)
	if uid == "" {
		t.Fatal("未拿到 tester 的 ID")
	}

	// 2) tester 用旧密码登录，拿到旧 token
	oldTok, _ := app.login(t, "tester", "oldpass1")
	if me := app.do("GET", "/api/me", oldTok, nil); me.Status != 200 {
		t.Fatalf("重置前 tester 旧 token 应可访问 /api/me，实际 %d", me.Status)
	}

	// 3) 管理员一键重置为 000000
	reset := app.do("POST", "/api/admin/users/"+uid+"/reset-password", adminTok, map[string]string{"newPassword": "000000"})
	if reset.Status != 200 {
		t.Fatalf("重置密码失败 %d: %v", reset.Status, reset.Body)
	}

	// 4) 旧 token 立即失效（会话被清除）
	if me := app.do("GET", "/api/me", oldTok, nil); me.Status != 401 {
		t.Fatalf("重置后旧 token 应被踢出(401)，实际 %d", me.Status)
	}

	// 5) 新密码 000000 可登录
	if _, id := app.login(t, "tester", "000000"); id == "" {
		t.Fatal("000000 应能登录")
	}

	// 6) 旧密码被拒
	if r := app.do("POST", "/api/login", "", map[string]string{"username": "tester", "password": "oldpass1"}); r.Status != 401 {
		t.Fatalf("旧密码应被拒(401)，实际 %d", r.Status)
	}
}

// TestAdminBanUser 验证定时封锁：
// 封锁后旧会话立即失效、登录被拒（提示封锁时间）；到期(或手动解封)后自动恢复。
func TestAdminBanUser(t *testing.T) {
	app, cleanup := newTestApp(t)
	defer cleanup()

	adminTok, _ := app.login(t, "admin", "admin123")
	batch := app.do("POST", "/api/admin/users/batch", adminTok, map[string]interface{}{
		"users": []map[string]string{{"username": "tester", "password": "pass123", "nickname": "测试员"}},
	})
	if batch.Status != 200 {
		t.Fatalf("批量创建失败 %d: %v", batch.Status, batch.Body)
	}
	uid, _ := batch.Body["created"].([]interface{})[0].(map[string]interface{})["id"].(string)

	// tester 正常登录
	oldTok, _ := app.login(t, "tester", "pass123")
	if me := app.do("GET", "/api/me", oldTok, nil); me.Status != 200 {
		t.Fatalf("封锁前应可访问，实际 %d", me.Status)
	}

	// 管理员不能封锁自己（后端防御）
	if r := app.do("POST", "/api/admin/users/"+adminID(adminTok, app)+"/ban", adminTok, map[string]float64{"hours": 1}); r.Status != 400 {
		t.Fatalf("封锁 admin 应被拒(400)，实际 %d: %v", r.Status, r.Body)
	}

	// 封锁 1 小时
	ban := app.do("POST", "/api/admin/users/"+uid+"/ban", adminTok, map[string]float64{"hours": 1})
	if ban.Status != 200 {
		t.Fatalf("封锁失败 %d: %v", ban.Status, ban.Body)
	}

	// 旧会话立即失效
	if me := app.do("GET", "/api/me", oldTok, nil); me.Status != 401 {
		t.Fatalf("封锁后旧 token 应失效(401)，实际 %d", me.Status)
	}
	// 登录被拒（提示封锁）
	if r := app.do("POST", "/api/login", "", map[string]string{"username": "tester", "password": "pass123"}); r.Status != 401 {
		t.Fatalf("封锁中登录应被拒(401)，实际 %d", r.Status)
	}

	// 手动解封
	un := app.do("POST", "/api/admin/users/"+uid+"/ban", adminTok, map[string]float64{"hours": 0})
	if un.Status != 200 {
		t.Fatalf("解封失败 %d: %v", un.Status, un.Body)
	}
	if _, id := app.login(t, "tester", "pass123"); id == "" {
		t.Fatal("解封后应能正常登录")
	}
}

// adminID 从 /api/me 取当前用户 ID（避免在测试里硬编码）
func adminID(tok string, app *testApp) string {
	me := app.do("GET", "/api/me", tok, nil)
	id, _ := me.Body["id"].(string)
	return id
}

// TestBanHidesContent 验证封锁语义一致性（与「封号即下架」一致）：
// 封锁只限制账号使用，已发布内容保持正常展示（按用户澄清：封号≠下架）
func TestBanKeepsContent(t *testing.T) {
	app, cleanup := newTestApp(t)
	defer cleanup()

	adminTok, _ := app.login(t, "admin", "admin123")
	batch := app.do("POST", "/api/admin/users/batch", adminTok, map[string]interface{}{
		"users": []map[string]string{{"username": "tester", "password": "pass123", "nickname": "测试员"}},
	})
	uid, _ := batch.Body["created"].([]interface{})[0].(map[string]interface{})["id"].(string)

	// tester 发笔记（普通用户默认待审核）
	tok, _ := app.login(t, "tester", "pass123")
	note := app.do("POST", "/api/notes", tok, map[string]interface{}{
		"title": "被封用户的笔记", "content": "内容", "mediaType": "image",
		"media": []string{"/uploads/" + uid + "/a.jpg"}, "tags": []string{"测试"},
	})
	nid, _ := note.Body["note"].(map[string]interface{})["id"].(string)
	if nid == "" {
		t.Fatalf("发笔记失败: %v", note.Body)
	}

	// 管理员审核定级 green → 可见
	if r := app.do("POST", "/api/admin/notes/"+nid+"/review", adminTok, map[string]string{"level": "green"}); r.Status != 200 {
		t.Fatalf("审核失败 %d: %v", r.Status, r.Body)
	}
	feedHas := func() bool {
		feed := app.do("GET", "/api/feed?sort=hot", adminTok, nil)
		items, _ := feed.Body["items"].([]interface{})
		for _, it := range items {
			if m, ok := it.(map[string]interface{}); ok && m["id"] == nid {
				return true
			}
		}
		return false
	}
	if !feedHas() {
		t.Fatal("审核通过后 feed 应包含该笔记")
	}

	// 封锁 1 小时：账号不可用，但内容保留展示
	if r := app.do("POST", "/api/admin/users/"+uid+"/ban", adminTok, map[string]float64{"hours": 1}); r.Status != 200 {
		t.Fatalf("封锁失败 %d", r.Status)
	}
	if !feedHas() {
		t.Fatal("封锁只限制账号使用，feed 应仍包含该笔记")
	}

	// 账号层面确实被锁：旧 token 失效、登录被拒
	if me := app.do("GET", "/api/me", tok, nil); me.Status != 401 {
		t.Fatalf("封锁后旧 token 应失效(401)，实际 %d", me.Status)
	}
	if r := app.do("POST", "/api/login", "", map[string]string{"username": "tester", "password": "pass123"}); r.Status != 401 {
		t.Fatalf("封锁中登录应被拒(401)，实际 %d", r.Status)
	}
}
