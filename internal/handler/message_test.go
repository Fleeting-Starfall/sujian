package handler

import (
	"encoding/json"
	"testing"
)

// TestMessageFile 私信图片/文件：成功发送与非法输入被拒。
func TestMessageFile(t *testing.T) {
	app, cleanup := newTestApp(t)
	defer cleanup()

	adminTok, adminID := app.login(t, "admin", "admin123")
	if r := app.do("POST", "/api/register", "", map[string]string{"username": "msgfile", "nickname": "MF", "password": "oldpass1"}); r.Status != 200 {
		t.Fatalf("注册失败 %d: %v", r.Status, r.Body)
	}
	var list []map[string]interface{}
	if r := app.do("GET", "/api/admin/users", adminTok, nil); r.Status != 200 {
		t.Fatalf("admin users %d", r.Status)
	} else if err := json.Unmarshal(r.Raw, &list); err != nil {
		t.Fatalf("解析 users 失败: %v", err)
	}
	aliceID := ""
	for _, um := range list {
		if um["username"] == "msgfile" {
			aliceID, _ = um["id"].(string)
			break
		}
	}
	if aliceID == "" {
		t.Fatal("未找到 msgfile")
	}
	if r := app.do("POST", "/api/admin/users/"+aliceID+"/approve", adminTok, nil); r.Status != 200 {
		t.Fatalf("审核失败 %d", r.Status)
	}
	aliceTok, _ := app.login(t, "msgfile", "oldpass1")

	// 非法输入
	if r := app.do("POST", "/api/messages/send", adminTok, map[string]string{"to": aliceID, "mediaType": "file"}); r.Status != 400 {
		t.Fatalf("文件无 media 应 400，实际 %d", r.Status)
	}
	if r := app.do("POST", "/api/messages/send", adminTok, map[string]interface{}{"to": aliceID, "mediaType": "image", "media": "/uploads/" + aliceID + "/x.png"}); r.Status != 400 {
		t.Fatalf("他人文件应 400，实际 %d", r.Status)
	}

	// 图片消息
	img := "/uploads/" + adminID + "/f_pic.png"
	r := app.do("POST", "/api/messages/send", adminTok, map[string]interface{}{
		"to": aliceID, "mediaType": "image", "media": img, "mediaName": "照片.png", "mediaSize": 2048,
	})
	if r.Status != 200 {
		t.Fatalf("图片消息失败 %d: %v", r.Status, r.Body)
	}
	if r.Body["mediaType"] != "image" || r.Body["media"] != img || r.Body["mediaName"] != "照片.png" || r.Body["mediaSize"] != float64(2048) {
		t.Fatalf("图片消息字段不对: %v", r.Body)
	}

	// 普通文件消息（.zip）
	file := "/uploads/" + adminID + "/f_doc.zip"
	r = app.do("POST", "/api/messages/send", adminTok, map[string]interface{}{
		"to": aliceID, "mediaType": "file", "media": file, "mediaName": "资料包.zip", "mediaSize": 1048576,
	})
	if r.Status != 200 {
		t.Fatalf("文件消息失败 %d: %v", r.Status, r.Body)
	}
	if r.Body["mediaType"] != "file" || r.Body["mediaName"] != "资料包.zip" {
		t.Fatalf("文件消息字段不对: %v", r.Body)
	}

	// 对方能看到两条文件消息
	r = app.do("GET", "/api/messages/with/"+adminID, aliceTok, nil)
	if r.Status != 200 {
		t.Fatalf("alice 拉会话失败 %d", r.Status)
	}
	msgs, _ := r.Body["messages"].([]interface{})
	types := map[string]int{}
	for _, im := range msgs {
		m, _ := im.(map[string]interface{})
		types[m["mediaType"].(string)]++
	}
	if types["image"] != 1 || types["file"] != 1 {
		t.Fatalf("对方应收到 1 图 1 文件，实际 %v", types)
	}
}

// TestMessageVideo 私信视频：成功发送、摘要[视频]、非法输入被拒。
func TestMessageVideo(t *testing.T) {
	app, cleanup := newTestApp(t)
	defer cleanup()

	// 1) admin 登录；注册 alice 并审核通过
	adminTok, adminID := app.login(t, "admin", "admin123")
	if r := app.do("POST", "/api/register", "", map[string]string{"username": "msgvideo", "nickname": "MV", "password": "oldpass1"}); r.Status != 200 {
		t.Fatalf("注册失败 %d: %v", r.Status, r.Body)
	}
	var list []map[string]interface{}
	if r := app.do("GET", "/api/admin/users", adminTok, nil); r.Status != 200 {
		t.Fatalf("admin users %d", r.Status)
	} else if err := json.Unmarshal(r.Raw, &list); err != nil {
		t.Fatalf("解析 users 失败: %v", err)
	}
	aliceID := ""
	for _, um := range list {
		if um["username"] == "msgvideo" {
			aliceID, _ = um["id"].(string)
			break
		}
	}
	if aliceID == "" {
		t.Fatal("未找到 msgvideo")
	}
	if r := app.do("POST", "/api/admin/users/"+aliceID+"/approve", adminTok, nil); r.Status != 200 {
		t.Fatalf("审核失败 %d", r.Status)
	}
	aliceTok, _ := app.login(t, "msgvideo", "oldpass1")

	// 2) 非法输入
	// 2a. 文本空 → 400
	if r := app.do("POST", "/api/messages/send", adminTok, map[string]string{"to": aliceID, "content": ""}); r.Status != 400 {
		t.Fatalf("空文本应 400，实际 %d", r.Status)
	}
	// 2b. 视频无媒体 → 400
	if r := app.do("POST", "/api/messages/send", adminTok, map[string]string{"to": aliceID, "mediaType": "video"}); r.Status != 400 {
		t.Fatalf("视频无媒体应 400，实际 %d", r.Status)
	}
	// 2c. 媒体不属于当前账号 → 400
	if r := app.do("POST", "/api/messages/send", adminTok, map[string]interface{}{"to": aliceID, "mediaType": "video", "media": "/uploads/" + aliceID + "/evil.mp4"}); r.Status != 400 {
		t.Fatalf("他人媒体应 400，实际 %d: %v", r.Status, r.Body)
	}

	// 3) admin 发视频消息给 alice
	vid := "/uploads/" + adminID + "/f_demo.mp4"
	r := app.do("POST", "/api/messages/send", adminTok, map[string]interface{}{"to": aliceID, "mediaType": "video", "media": vid})
	if r.Status != 200 {
		t.Fatalf("视频消息发送失败 %d: %v", r.Status, r.Body)
	}
	msg, _ := r.Body["mediaType"].(string)
	media, _ := r.Body["media"].(string)
	if msg != "video" || media != vid {
		t.Fatalf("返回的媒体字段不对: mediaType=%q media=%q", msg, media)
	}

	// 4) alice 会话中能看到该视频消息
	r = app.do("GET", "/api/messages/with/"+adminID, aliceTok, nil)
	if r.Status != 200 {
		t.Fatalf("alice 拉会话失败 %d", r.Status)
	}
	msgs, _ := r.Body["messages"].([]interface{})
	found := false
	for _, im := range msgs {
		m, _ := im.(map[string]interface{})
		if m["mediaType"] == "video" && m["media"] == vid && m["fromId"] == adminID {
			found = true
		}
	}
	if !found {
		t.Fatalf("alice 未收到视频消息: %v", r.Body["messages"])
	}

	// 5) alice 会话列表最后消息摘要为 [视频]（由前端 msgPreview 处理，这里验证消息数据可渲染）
	r = app.do("GET", "/api/messages", aliceTok, nil)
	if r.Status != 200 {
		t.Fatalf("alice 会话列表失败 %d", r.Status)
	}
	// /api/messages 返回数组，用 Raw 解析
	var arr []map[string]interface{}
	if err := json.Unmarshal(r.Raw, &arr); err != nil {
		t.Fatalf("解析会话列表失败: %v", err)
	}
	lastIsVideo := false
	for _, c := range arr {
		if lm, ok := c["lastMsg"].(map[string]interface{}); ok && lm["mediaType"] == "video" {
			lastIsVideo = true
		}
	}
	if !lastIsVideo {
		t.Fatalf("会话列表应有视频消息 lastMsg: %v", arr)
	}
}
