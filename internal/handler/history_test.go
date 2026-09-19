package handler

import (
	"encoding/json"
	"fmt"
	"testing"

	"sujian/internal/store"
)

// TestViewHistory 浏览历史全链路：
// 注册→审核→admin发2篇笔记→用户浏览详情→历史记录→重复浏览去重(更新时间戳)→
// 列表时间倒序→删除单条→清空→未登录401。
func TestViewHistory(t *testing.T) {
	app, cleanup := newTestApp(t)
	defer cleanup()

	// 1) admin 登录并发两篇笔记（admin 发布即 published/green）
	adminTok, adminID := app.login(t, "admin", "admin123")
	noteIDs := make([]string, 0, 2)
	for _, title := range []string{"历史测试一", "历史测试二"} {
		r := app.do("POST", "/api/notes", adminTok, map[string]interface{}{
			"title": title, "content": "内容", "mediaType": "image",
			"media": []string{"/uploads/" + adminID + "/a.jpg"},
		})
		if r.Status != 200 {
			t.Fatalf("发笔记失败 %d: %v", r.Status, r.Body)
		}
		n, _ := r.Body["note"].(map[string]interface{})
		noteIDs = append(noteIDs, n["id"].(string))
	}

	// 2) 注册 histuser 并让管理员审核通过
	if r := app.do("POST", "/api/register", "", map[string]string{"username": "histuser", "nickname": "H", "password": "oldpass1"}); r.Status != 200 {
		t.Fatalf("注册失败 %d: %v", r.Status, r.Body)
	}
	var list []map[string]interface{}
	if r := app.do("GET", "/api/admin/users", adminTok, nil); r.Status != 200 {
		t.Fatalf("admin users %d", r.Status)
	} else if err := json.Unmarshal(r.Raw, &list); err != nil {
		t.Fatalf("解析 users 失败: %v", err)
	}
	uid := ""
	for _, um := range list {
		if um["username"] == "histuser" {
			uid, _ = um["id"].(string)
			break
		}
	}
	if uid == "" {
		t.Fatal("未找到 histuser")
	}
	if r := app.do("POST", "/api/admin/users/"+uid+"/approve", adminTok, nil); r.Status != 200 {
		t.Fatalf("审核失败 %d: %v", r.Status, r.Body)
	}
	userTok, _ := app.login(t, "histuser", "oldpass1")

	// 3) 未登录访问历史 → 401
	if r := app.do("GET", "/api/me/history", "", nil); r.Status != 401 {
		t.Fatalf("未登录应 401，实际 %d", r.Status)
	}

	// 4) 浏览 n1 → 历史 1 条
	if r := app.do("GET", "/api/notes/"+noteIDs[0], userTok, nil); r.Status != 200 {
		t.Fatalf("浏览 n1 失败 %d", r.Status)
	}
	hist := app.historyIDs(t, userTok)
	if len(hist) != 1 || hist[0] != noteIDs[0] {
		t.Fatalf("浏览 n1 后历史应为 [n1]，实际 %v", hist)
	}

	// 5) 浏览 n2 → 2 条，倒序第一是 n2
	if r := app.do("GET", "/api/notes/"+noteIDs[1], userTok, nil); r.Status != 200 {
		t.Fatalf("浏览 n2 失败 %d", r.Status)
	}
	hist = app.historyIDs(t, userTok)
	if len(hist) != 2 || hist[0] != noteIDs[1] || hist[1] != noteIDs[0] {
		t.Fatalf("浏览 n2 后历史应为 [n2 n1]，实际 %v", hist)
	}

	// 6) 再浏览 n1 → 仍 2 条（去重），且 n1 变到最前（更新时间戳）
	if r := app.do("GET", "/api/notes/"+noteIDs[0], userTok, nil); r.Status != 200 {
		t.Fatalf("再浏览 n1 失败 %d", r.Status)
	}
	hist = app.historyIDs(t, userTok)
	if len(hist) != 2 || hist[0] != noteIDs[0] || hist[1] != noteIDs[1] {
		t.Fatalf("重复浏览后应为 [n1 n2]，实际 %v", hist)
	}

	// 7) 删除单条 n2 → 剩 1 条
	if r := app.do("DELETE", "/api/me/history/"+noteIDs[1], userTok, nil); r.Status != 200 {
		t.Fatalf("删除单条失败 %d", r.Status)
	}
	hist = app.historyIDs(t, userTok)
	if len(hist) != 1 || hist[0] != noteIDs[0] {
		t.Fatalf("删除 n2 后应为 [n1]，实际 %v", hist)
	}

	// 8) 清空 → 0 条
	if r := app.do("DELETE", "/api/me/history", userTok, nil); r.Status != 200 {
		t.Fatalf("清空失败 %d", r.Status)
	}
	if hist = app.historyIDs(t, userTok); len(hist) != 0 {
		t.Fatalf("清空后应为空，实际 %v", hist)
	}
}

// historyIDs 调 /api/me/history 并返回 note id 列表（按返回顺序）
func (a *testApp) historyIDs(t *testing.T, token string) []string {
	t.Helper()
	r := a.do("GET", "/api/me/history", token, nil)
	if r.Status != 200 {
		t.Fatalf("history %d: %v", r.Status, r.Body)
	}
	var arr []map[string]interface{}
	if err := json.Unmarshal(r.Raw, &arr); err != nil {
		t.Fatalf("解析 history 失败: %v (raw=%s)", err, string(r.Raw))
	}
	out := make([]string, 0, len(arr))
	for _, it := range arr {
		n, _ := it["note"].(map[string]interface{})
		if id, ok := n["id"].(string); ok {
			out = append(out, id)
		}
	}
	return out
}

// TestViewHistoryLimit store 层上限淘汰：超过上限（200）条时自动淘汰最旧的。
func TestViewHistoryLimit(t *testing.T) {
	dir := t.TempDir()
	st := store.New(dir)
	st.SeedAdmin("admin", "admin123")
	u, err := st.Register("limituser", "L", "pass1234", nil)
	if err != nil {
		t.Fatal(err)
	}
	const limit = 200
	// 先记录超过上限数量的笔记浏览历史（笔记 ID 无所谓，store 不校验存在性）
	for i := 0; i < limit+50; i++ {
		st.RecordView(u.ID, fmt.Sprintf("n_%05d", i))
	}
	hist := st.ViewHistory(u.ID)
	if len(hist) != limit {
		t.Fatalf("上限应为 %d 条，实际 %d", limit, len(hist))
	}
	// 最旧的 50 条应被淘汰：剩余应是最新的 limit 条（n_0050 ~ n_0249）
	first := hist[0].NoteID
	if first != fmt.Sprintf("n_%05d", limit+49) {
		t.Fatalf("最新一条应为 n_%05d，实际 %s", limit+49, first)
	}
	// 删除单条 + 清空
	st.DeleteHistoryItem(u.ID, hist[0].NoteID)
	if len(st.ViewHistory(u.ID)) != limit-1 {
		t.Fatal("删除单条后应为上限-1")
	}
	st.ClearHistory(u.ID)
	if len(st.ViewHistory(u.ID)) != 0 {
		t.Fatal("清空后应为 0")
	}
}
