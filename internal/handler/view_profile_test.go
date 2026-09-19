package handler

import (
	"testing"
)

// TestViewSignalInProfile 浏览行为进入画像并影响推荐排序。
func TestViewSignalInProfile(t *testing.T) {
	app, cleanup := newTestApp(t)
	defer cleanup()

	adminTok, adminID := app.login(t, "admin", "admin123")
	if adminID == "" {
		t.Fatal("未拿到 admin ID")
	}

	// 准备两个不同标签的候选笔记（均 green / published，admin 直接发布）
	const (
		tagA = "摄影"
		tagB = "编程"
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
	noteA1 := mk("A1", tagA)
	noteA2 := mk("A2", tagA)
	noteB1 := mk("B1", tagB) // B 类对照候选：验证画像偏向 tagA 后应排在 A2 之后
	noteB2 := mk("B2", tagB) // B 类对照候选

	// 注册并审核一个普通用户 bob
	reg := app.do("POST", "/api/register", "", map[string]string{
		"username": "bob", "nickname": "Bob", "password": "pass123",
	})
	bobID := ""
	if u, ok := reg.Body["user"].(map[string]interface{}); ok {
		bobID, _ = u["id"].(string)
	}
	if bobID == "" {
		t.Fatal("未拿到 bob ID")
	}
	if app.do("POST", "/api/admin/users/"+bobID+"/approve", adminTok, nil).Status != 200 {
		t.Fatal("审核 bob 失败")
	}
	bobTok, _ := app.login(t, "bob", "pass123")

	// 冷启动基线：bob 浏览前先拉一次推荐（仅作日志，不作为严格断言基准，
	// 因冷启动顺序不确定；严格断言只看「浏览 tagA 后是否明显偏向 tagA」）
	baseFeed := app.do("GET", "/api/feed?sort=hot", bobTok, nil)
	baseItems, _ := baseFeed.Body["items"].([]interface{})
	baseRankA2 := rankOf(baseItems, noteA2)

	// bob 浏览 tagA 两篇笔记（RecordView 入画像）
	// feed 仅展示不记浏览，须打开详情才入画像
	for _, id := range []string{noteA1, noteA2} {
		v := app.do("GET", "/api/notes/"+id, bobTok, nil)
		if v.Status != 200 {
			t.Fatalf("浏览笔记 %s 失败 %d", id, v.Status)
		}
	}

	// 浏览后再次拉推荐
	afterFeed := app.do("GET", "/api/feed?sort=hot", bobTok, nil)
	afterItems, _ := afterFeed.Body["items"].([]interface{})

	// 断言：浏览 tagA 后 A2 排在 B 类对照之前
	afterRankA2 := rankOf(afterItems, noteA2)
	rankB1 := rankOf(afterItems, noteB1)
	rankB2 := rankOf(afterItems, noteB2)
	if afterRankA2 < 0 {
		t.Fatalf("浏览 tagA 后，A2 未进入首屏推荐（排名=%d），画像加权未生效", afterRankA2)
	}
	if rankB1 >= 0 && afterRankA2 > rankB1 {
		t.Fatalf("浏览 tagA 后 A2 竟排在 B1 之后：A2=%d B1=%d，画像未偏向 tagA", afterRankA2, rankB1)
	}
	if rankB2 >= 0 && afterRankA2 > rankB2 {
		t.Fatalf("浏览 tagA 后 A2 竟排在 B2 之后：A2=%d B2=%d，画像未偏向 tagA", afterRankA2, rankB2)
	}
	t.Logf("✓ 冷启动 A2 排名=%d；浏览 tagA 后 A2=%d，B 对照 B1=%d B2=%d（均不优于 A2，画像加权生效）",
		baseRankA2, afterRankA2, rankB1, rankB2)
}

// rankOf 返回笔记 id 在 feed 中的下标
func rankOf(items []interface{}, id string) int {
	for i, it := range items {
		if m, ok := it.(map[string]interface{}); ok {
			if mid, ok := m["id"].(string); ok && mid == id {
				return i
			}
		}
	}
	return -1
}
