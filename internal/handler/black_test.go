package handler

import (
	"encoding/json"
	"testing"
)

// idsOf 从 feed / 列表响应里取出所有笔记 id（feed 类接口包裹在 items 下）
func idsOf(items interface{}) []string {
	out := []string{}
	if arr, ok := items.([]interface{}); ok {
		for _, it := range arr {
			if m, ok := it.(map[string]interface{}); ok {
				if id, ok := m["id"].(string); ok {
					out = append(out, id)
				}
			}
		}
	}
	return out
}

func contains(items interface{}, id string) bool {
	for _, x := range idsOf(items) {
		if x == id {
			return true
		}
	}
	return false
}

// idsOfRaw 从裸 JSON 数组响应里取笔记 id（如 /api/user/{id}/notes 直接返回数组）
func idsOfRaw(raw []byte) []string {
	var arr []map[string]interface{}
	if err := json.Unmarshal(raw, &arr); err != nil {
		return nil
	}
	out := []string{}
	for _, m := range arr {
		if id, ok := m["id"].(string); ok {
			out = append(out, id)
		}
	}
	return out
}

func containsRaw(raw []byte, id string) bool {
	for _, x := range idsOfRaw(raw) {
		if x == id {
			return true
		}
	}
	return false
}

// TestBlackLevelVisibility 黑标（最高敏感）行为：
//   - 不在首页 feed / 推荐 / 他人主页展示
//   - 仅用户主动搜索可见
//   - 详情需登录，且返回 sensitive + sensitiveLevel=black
//   - 作者本人与管理员可见自己的黑标内容
func TestBlackLevelVisibility(t *testing.T) {
	app, cleanup := newTestApp(t)
	defer cleanup()

	adminTok, adminID := app.login(t, "admin", "admin123")
	if adminTok == "" {
		t.Fatal("管理员登录失败")
	}
	// 普通用户（用于验证「他人视角」不可见）
	reg := app.do("POST", "/api/register", "", map[string]string{
		"username": "bob", "password": "pass123", "nickname": "Bob",
	})
	if reg.Status != 200 {
		t.Fatalf("注册 bob 失败 %d: %v", reg.Status, reg.Body)
	}
	// 新注册用户默认待审核，测试中由管理员审核通过 bob
	bobID := ""
	if u, ok := reg.Body["user"].(map[string]interface{}); ok {
		bobID, _ = u["id"].(string)
	}
	if bobID == "" {
		t.Fatal("未拿到 bob ID")
	}
	if app.do("POST", "/api/admin/users/"+bobID+"/approve", adminTok, nil).Status != 200 {
		t.Fatal("审核通过 bob 失败")
	}
	bobTok, _ := app.login(t, "bob", "pass123")

	// 管理员发一篇笔记（admin 自动 published+green），再改级为黑标
	const title = "黑标专属搜索词xyz"
	r := app.do("POST", "/api/notes", adminTok, map[string]interface{}{
		"title": title, "content": "内容", "mediaType": "image",
		"media": []string{"/uploads/" + adminID + "/a.jpg"}, "tags": []string{"黑标测试"},
	})
	if r.Status != 200 {
		t.Fatalf("发笔记失败 %d: %v", r.Status, r.Body)
	}
	blackID := ""
	if n, ok := r.Body["note"].(map[string]interface{}); ok {
		blackID, _ = n["id"].(string)
	}
	if blackID == "" {
		t.Fatal("未拿到笔记 ID")
	}
	rev := app.do("POST", "/api/admin/notes/"+blackID+"/review", adminTok, map[string]string{"level": "black"})
	if rev.Status != 200 {
		t.Fatalf("定级 black 失败 %d: %v", rev.Status, rev.Body)
	}
	if app.srv.Store.NoteRaw(blackID).Level != "black" {
		t.Fatal("笔记等级未落库为 black")
	}

	// 1) 首页 feed（无关键词，浏览态）不包含黑标
	feed := app.do("GET", "/api/feed", adminTok, nil)
	if feed.Status != 200 {
		t.Fatalf("feed 失败 %d: %v", feed.Status, feed.Body)
	}
	if contains(feed.Body["items"], blackID) {
		t.Fatal("黑标不应出现在首页 feed（浏览态）")
	}

	// 2) 推荐频道（sort=hot&tab=空&已登录）不包含黑标
	reco := app.do("GET", "/api/feed?sort=hot", adminTok, nil)
	if contains(reco.Body["items"], blackID) {
		t.Fatal("黑标不应进入「为你推荐」推送")
	}

	// 3) 主动搜索可找到黑标
	search := app.do("GET", "/api/feed?q="+title, adminTok, nil)
	if !contains(search.Body["items"], blackID) {
		t.Fatal("黑标应通过用户搜索可见")
	}
	// 3b) 未登录用户搜索黑标不可见（黑标最敏感，与详情一致必须登录；否则搜到却点不进去）
	anonSearch := app.do("GET", "/api/feed?q="+title, "", nil)
	if contains(anonSearch.Body["items"], blackID) {
		t.Fatal("黑标即使搜索，未登录用户也不应可见（需登录）")
	}

	// 4) 他人主页（bob 看 admin 的资料）看不到黑标
	othersProfile := app.do("GET", "/api/user/"+adminID+"/notes", bobTok, nil)
	if containsRaw(othersProfile.Raw, blackID) {
		t.Fatal("黑标不应出现在他人公开主页")
	}
	// 4b) 作者本人主页可见自己的黑标
	selfProfile := app.do("GET", "/api/user/"+adminID+"/notes", adminTok, nil)
	if !containsRaw(selfProfile.Raw, blackID) {
		t.Fatal("作者本人应能在自己主页看到黑标内容")
	}

	// 5) 详情：未登录 → 401
	anonDetail := app.do("GET", "/api/notes/"+blackID, "", nil)
	if anonDetail.Status != 401 {
		t.Fatalf("黑标详情未登录应 401，实际 %d", anonDetail.Status)
	}
	// 6) 详情：登录用户 → 200，且 sensitive=true, sensitiveLevel=black
	detail := app.do("GET", "/api/notes/"+blackID, bobTok, nil)
	if detail.Status != 200 {
		t.Fatalf("登录用户查看黑标详情应 200，实际 %d: %v", detail.Status, detail.Body)
	}
	if sens, _ := detail.Body["sensitive"].(bool); !sens {
		t.Fatal("黑标详情应返回 sensitive=true")
	}
	if lv, _ := detail.Body["sensitiveLevel"].(string); lv != "black" {
		t.Fatalf("黑标详情 sensitiveLevel 应为 black，实际 %v", lv)
	}
}
