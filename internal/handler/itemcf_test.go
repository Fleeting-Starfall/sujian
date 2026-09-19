package handler

import (
	"testing"
)

// TestItemCF 验证 Item-CF 协同过滤：用户 bob 互动(收藏)了笔记 A(tagA)；
// 另一用户 carol 同时收藏了 A 和 B(tagB)。即便 bob 从未接触过 tagB，
// 因「和 bob 相似(同样收藏 A)的 carol 也收藏了 B」，B 应被协同加权、排在无人共现的对照 C 之前。
func TestItemCF(t *testing.T) {
	app, cleanup := newTestApp(t)
	defer cleanup()

	adminTok, adminID := app.login(t, "admin", "admin123")
	if adminID == "" {
		t.Fatal("未拿到 admin ID")
	}
	const (
		tagA = "摄影"
		tagB = "编程"
		tagC = "美食"
	)
	mk := func(title, tag string) string {
		r := app.do("POST", "/api/notes", adminTok, map[string]interface{}{
			"title":     title,
			"content":   "内容",
			"mediaType": "image",
			"media":     []string{"/uploads/" + adminID + "/" + title + ".jpg"},
			"tags":      []string{tag},
		})
		if r.Status != 200 {
			t.Fatalf("发笔记 %s 失败 %d: %v", title, r.Status, r.Body)
		}
		if n, ok := r.Body["note"].(map[string]interface{}); ok {
			if id, ok := n["id"].(string); ok {
				return id
			}
		}
		t.Fatalf("未拿到笔记 %s ID", title)
		return ""
	}
	noteA := mk("CFA", tagA) // bob 收藏；carol 也收藏
	noteB := mk("CFB", tagB) // 仅 carol 收藏 -> 应被协同拉给 bob
	noteC := mk("CFC", tagC) // 无人与 A 共现 -> 对照，不应因协同获益

	// 注册 bob 与 carol 并审核
	regBob := app.do("POST", "/api/register", "", map[string]string{"username": "bob", "nickname": "Bob", "password": "pass123"})
	regCarol := app.do("POST", "/api/register", "", map[string]string{"username": "carol", "nickname": "Carol", "password": "pass123"})
	bobID := regBob.Body["user"].(map[string]interface{})["id"].(string)
	carolID := regCarol.Body["user"].(map[string]interface{})["id"].(string)
	for _, id := range []string{bobID, carolID} {
		if app.do("POST", "/api/admin/users/"+id+"/approve", adminTok, nil).Status != 200 {
			t.Fatalf("审核用户 %s 失败", id)
		}
	}
	bobTok, _ := app.login(t, "bob", "pass123")
	carolTok, _ := app.login(t, "carol", "pass123")

	// bob 收藏 A（建立 bob 的协同起点）
	if app.do("POST", "/api/notes/"+noteA+"/favorite", bobTok, nil).Status != 200 {
		t.Fatal("bob 收藏 A 失败")
	}
	// carol 同时收藏 A 和 B（建立共现：A<->B 通过 carol 关联）
	if app.do("POST", "/api/notes/"+noteA+"/favorite", carolTok, nil).Status != 200 {
		t.Fatal("carol 收藏 A 失败")
	}
	if app.do("POST", "/api/notes/"+noteB+"/favorite", carolTok, nil).Status != 200 {
		t.Fatal("carol 收藏 B 失败")
	}
	// 注：noteC 无人收藏，作为无协同的对照

	// bob 的推荐：B 应因协同(经 A 共现)排在 C 之前
	feed := app.do("GET", "/api/feed?sort=hot", bobTok, nil)
	items, _ := feed.Body["items"].([]interface{})
	rankB := rankOf(items, noteB)
	rankC := rankOf(items, noteC)
	if rankB < 0 {
		t.Fatalf("协同过滤未将 B 推入 bob 推荐（B 不在列表中）")
	}
	if rankC < 0 {
		t.Fatalf("对照 C 未在推荐中（测试环境异常）")
	}
	if rankB > rankC {
		t.Fatalf("协同过滤失效：B(rank=%d) 排在 C(rank=%d) 之后，应更靠前", rankB, rankC)
	}
	t.Logf("✓ Item-CF 生效：B(rank=%d) 经 A 共现排在 C(rank=%d) 之前", rankB, rankC)
}
