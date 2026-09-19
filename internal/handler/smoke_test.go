package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"sujian/internal/store"
)

// testApp 进程内 Server（真实 handler + store，内存临时目录）
type testApp struct {
	srv *Server
	mux *http.ServeMux
	dir string
}

type resp struct {
	Status int
	Body   map[string]interface{}
	Raw    []byte
}

func newTestApp(t *testing.T) (*testApp, func()) {
	t.Helper()
	dir, err := os.MkdirTemp("", "sujian-test-")
	if err != nil {
		t.Fatal(err)
	}
	st := store.New(dir)
	st.SeedAdmin("admin", "admin123")
	cfg := Config{
		Port:       0,
		AdminUser:  "admin",
		AdminPass:  "admin123",
		MaxImageMB: 10,
		MaxVideoMB: 200,
		UploadDir:  filepath.Join(dir, "uploads"),
		PublicDir:  "public",
	}
	srv := &Server{Store: st, Cfg: cfg}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	return &testApp{srv: srv, mux: mux, dir: dir}, func() { os.RemoveAll(dir) }
}

func (a *testApp) do(method, path, token string, body interface{}) resp {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Cookie", "session="+token)
	}
	rec := httptest.NewRecorder()
	a.mux.ServeHTTP(rec, req)
	r := resp{Status: rec.Code, Body: map[string]interface{}{}, Raw: rec.Body.Bytes()}
	if rec.Body.Len() > 0 {
		_ = json.Unmarshal(rec.Body.Bytes(), &r.Body)
	}
	return r
}

// login 返回 (token, userID)
func (a *testApp) login(t *testing.T, user, pass string) (string, string) {
	t.Helper()
	r := a.do("POST", "/api/login", "", map[string]string{"username": user, "password": pass})
	if r.Status != 200 {
		t.Fatalf("login %s 期望 200，实际 %d: %v", user, r.Status, r.Body)
	}
	tok, _ := r.Body["token"].(string)
	u, _ := r.Body["user"].(map[string]interface{})
	id, _ := u["id"].(string)
	return tok, id
}

func TestFunctionalFlows(t *testing.T) {
	app, cleanup := newTestApp(t)
	defer cleanup()

	// 1) 管理员登录
	adminTok, adminID := app.login(t, "admin", "admin123")
	if adminTok == "" || adminID == "" {
		t.Fatal("管理员登录失败")
	}

	// 2) 管理员发一篇 green 笔记（admin 角色直接 published+green）
	r := app.do("POST", "/api/notes", adminTok, map[string]interface{}{
		"title":     "管理员笔记",
		"content":   "内容",
		"mediaType": "image",
		"media":     []string{"/uploads/" + adminID + "/a.jpg"},
		"tags":      []string{"测试"},
	})
	if r.Status != 200 {
		t.Fatalf("管理员发笔记失败 %d: %v", r.Status, r.Body)
	}
	adminNoteID := ""
	if n, ok := r.Body["note"].(map[string]interface{}); ok {
		adminNoteID, _ = n["id"].(string)
	}
	if adminNoteID == "" {
		t.Fatal("未拿到管理员笔记 ID")
	}

	// 3) feed 包含该笔记
	feed := app.do("GET", "/api/feed", adminTok, nil)
	if feed.Status != 200 {
		t.Fatalf("feed 失败 %d", feed.Status)
	}
	notes, _ := feed.Body["items"].([]interface{})
	if !containsNoteID(notes, adminNoteID) {
		t.Fatalf("feed 未包含管理员笔记 %s, 实际: %v", adminNoteID, notes)
	}

	// 4) 注册普通用户（pending）
	reg := app.do("POST", "/api/register", "", map[string]string{
		"username": "alice", "nickname": "Alice", "password": "pass123",
	})
	if reg.Status != 200 {
		t.Fatalf("注册失败 %d: %v", reg.Status, reg.Body)
	}
	aliceID := ""
	if u, ok := reg.Body["user"].(map[string]interface{}); ok {
		aliceID, _ = u["id"].(string)
	}
	if aliceID == "" {
		t.Fatal("未拿到 alice ID")
	}

	// 5) pending 用户无法登录
	badLogin := app.do("POST", "/api/login", "", map[string]string{"username": "alice", "password": "pass123"})
	if badLogin.Status == 200 {
		t.Fatal("pending 用户不应能登录")
	}

	// 6) 管理员通过审核
	appr := app.do("POST", "/api/admin/users/"+aliceID+"/approve", adminTok, nil)
	if appr.Status != 200 {
		t.Fatalf("审核通过失败 %d: %v", appr.Status, appr.Body)
	}

	// 7) alice 现在可以登录
	aliceTok, _ := app.login(t, "alice", "pass123")
	if aliceTok == "" {
		t.Fatal("alice 审核后登录失败")
	}

	// 8) alice 发笔记（pending，需审核）
	rn := app.do("POST", "/api/notes", aliceTok, map[string]interface{}{
		"title":     "alice笔记",
		"content":   "hi",
		"mediaType": "image",
		"media":     []string{"/uploads/" + aliceID + "/x.jpg"},
	})
	if rn.Status != 200 {
		t.Fatalf("alice 发笔记失败 %d: %v", rn.Status, rn.Body)
	}
	aliceNoteID := ""
	if n, ok := rn.Body["note"].(map[string]interface{}); ok {
		aliceNoteID, _ = n["id"].(string)
	}
	if aliceNoteID == "" {
		t.Fatal("未拿到 alice 笔记 ID")
	}

	// alice 的笔记此时仍是 pending，feed 不应包含
	feed2 := app.do("GET", "/api/feed", aliceTok, nil)
	notes2, _ := feed2.Body["items"].([]interface{})
	if containsNoteID(notes2, aliceNoteID) {
		t.Fatal("pending 笔记不应出现在 feed")
	}

	// 9) 管理员审核通过 alice 的笔记
	rev := app.do("POST", "/api/admin/notes/"+aliceNoteID+"/review", adminTok, map[string]string{"level": "green"})
	if rev.Status != 200 {
		t.Fatalf("审核笔记失败 %d: %v", rev.Status, rev.Body)
	}
	feed3 := app.do("GET", "/api/feed", aliceTok, nil)
	notes3, _ := feed3.Body["items"].([]interface{})
	if !containsNoteID(notes3, aliceNoteID) {
		t.Fatal("审核通过后笔记应出现在 feed")
	}

	// 10) alice 评论 + 点赞
	cmt := app.do("POST", "/api/notes/"+adminNoteID+"/comment", aliceTok, map[string]string{"content": "好棒"})
	if cmt.Status != 200 {
		t.Fatalf("评论失败 %d: %v", cmt.Status, cmt.Body)
	}
	commentID := ""
	if cid, ok := cmt.Body["id"].(string); ok {
		commentID = cid
	}
	if commentID == "" {
		t.Fatal("未拿到评论 ID")
	}
	clk := app.do("POST", "/api/comments/"+commentID+"/like", aliceTok, nil)
	if clk.Status != 200 {
		t.Fatalf("评论点赞失败 %d: %v", clk.Status, clk.Body)
	}

	// 11) 私信（验证安全修复：响应里绝不能出现 passwordHash）
	app.do("POST", "/api/messages/send", aliceTok, map[string]string{"to": adminID, "content": "你好管理员"})
	thread := app.do("GET", "/api/messages/with/"+adminID, aliceTok, nil)
	if thread.Status != 200 {
		t.Fatalf("私信会话失败 %d", thread.Status)
	}
	if hasPasswordKey(thread.Body) {
		t.Fatal("私信会话响应泄露了密码字段（安全漏洞未修复）")
	}
	peer, _ := thread.Body["peer"].(map[string]interface{})
	if hasPasswordKey(peer) {
		t.Fatal("peer 对象泄露了密码字段")
	}

	// 12) 举报笔记
	rep := app.do("POST", "/api/report/note/"+adminNoteID, aliceTok, map[string]string{"reason": "违规"})
	if rep.Status != 200 {
		t.Fatalf("举报失败 %d: %v", rep.Status, rep.Body)
	}
	reports := app.do("GET", "/api/admin/reports", adminTok, nil)
	if reports.Status != 200 {
		t.Fatalf("获取举报列表失败 %d", reports.Status)
	}
	var rlist []interface{}
	if err := json.Unmarshal(reports.Raw, &rlist); err != nil || len(rlist) == 0 {
		t.Fatalf("举报未进入后台列表 (raw=%s, err=%v)", string(reports.Raw), err)
	}

	// 13) 被禁用户内容自动下架（banned 状态 = 非 active）
	app.srv.Store.SetUserStatus(aliceID, "banned")
	feedAfterBan := app.do("GET", "/api/feed", adminTok, nil)
	notesBan, _ := feedAfterBan.Body["items"].([]interface{})
	if containsNoteID(notesBan, aliceNoteID) {
		t.Fatal("被禁用户的笔记仍出现在 feed（下架失败）")
	}
}

// TestNoteEditReReview 编辑后强制重新审核
func TestNoteEditReReview(t *testing.T) {
	app, cleanup := newTestApp(t)
	defer cleanup()

	adminTok, _ := app.login(t, "admin", "admin123")

	// 1) 注册用户 dave 并通过审核
	reg := app.do("POST", "/api/register", "", map[string]string{
		"username": "dave", "nickname": "Dave", "password": "pass123",
	})
	daveID := ""
	if u, ok := reg.Body["user"].(map[string]interface{}); ok {
		daveID, _ = u["id"].(string)
	}
	app.do("POST", "/api/admin/users/"+daveID+"/approve", adminTok, nil)
	daveTok, _ := app.login(t, "dave", "pass123")

	// 2) dave 发笔记 → pending
	rn := app.do("POST", "/api/notes", daveTok, map[string]interface{}{
		"title": "初稿", "content": "v1", "mediaType": "image",
		"media": []string{"/uploads/" + daveID + "/a.jpg"},
	})
	noteID := ""
	if n, ok := rn.Body["note"].(map[string]interface{}); ok {
		noteID, _ = n["id"].(string)
	}

	// 3) 管理员审核通过为 green
	rev := app.do("POST", "/api/admin/notes/"+noteID+"/review", adminTok, map[string]string{"level": "green"})
	if rev.Status != 200 {
		t.Fatalf("审核失败 %d: %v", rev.Status, rev.Body)
	}
	feed := app.do("GET", "/api/feed", daveTok, nil)
	if !containsNoteID(feed.Body["items"].([]interface{}), noteID) {
		t.Fatal("审核通过后笔记应出现在 feed")
	}

	// 4) 编辑已发布笔记 → 强制回到 pending，feed 消失
	up := app.do("PUT", "/api/notes/"+noteID, daveTok, map[string]interface{}{"title": "修改稿", "content": "v2"})
	if up.Status != 200 {
		t.Fatalf("编辑失败 %d: %v", up.Status, up.Body)
	}
	feed2 := app.do("GET", "/api/feed", daveTok, nil)
	if containsNoteID(feed2.Body["items"].([]interface{}), noteID) {
		t.Fatal("编辑后的笔记不应出现在 feed（应重新审核）")
	}
	// 作者本人仍能通过 noteDetail 看到（pending 状态）
	det := app.do("GET", "/api/notes/"+noteID, daveTok, nil)
	if det.Status != 200 {
		t.Fatalf("作者查看自己的 pending 笔记应 200，实际 %d", det.Status)
	}

	// 5) 管理员重新审核 → 恢复可见
	rev2 := app.do("POST", "/api/admin/notes/"+noteID+"/review", adminTok, map[string]string{"level": "green"})
	if rev2.Status != 200 {
		t.Fatalf("重新审核失败 %d: %v", rev2.Status, rev2.Body)
	}
	feed3 := app.do("GET", "/api/feed", daveTok, nil)
	if !containsNoteID(feed3.Body["items"].([]interface{}), noteID) {
		t.Fatal("重新审核通过后笔记应恢复在 feed")
	}

	// 6) 被驳回的笔记可以编辑重新提交
	rn2 := app.do("POST", "/api/notes", daveTok, map[string]interface{}{
		"title": "第二篇", "content": "v1", "mediaType": "image",
		"media": []string{"/uploads/" + daveID + "/b.jpg"},
	})
	note2ID := ""
	if n, ok := rn2.Body["note"].(map[string]interface{}); ok {
		note2ID, _ = n["id"].(string)
	}
	if app.do("POST", "/api/admin/notes/"+note2ID+"/reject", adminTok, nil).Status != 200 {
		t.Fatal("驳回失败")
	}
	// 被驳回后作者可编辑（原来会 404）
	up2 := app.do("PUT", "/api/notes/"+note2ID, daveTok, map[string]interface{}{"title": "修改后的第二篇"})
	if up2.Status != 200 {
		t.Fatalf("被驳回笔记应可编辑重交，实际 %d: %v", up2.Status, up2.Body)
	}
	// 编辑后状态回到 pending，管理员可重新审核
	if app.do("POST", "/api/admin/notes/"+note2ID+"/review", adminTok, map[string]string{"level": "yellow"}).Status != 200 {
		t.Fatal("被驳回编辑后应能重新审核")
	}

	// 7) 非作者不能编辑
	reg3 := app.do("POST", "/api/register", "", map[string]string{
		"username": "eve", "nickname": "Eve", "password": "pass123",
	})
	eveID := ""
	if u, ok := reg3.Body["user"].(map[string]interface{}); ok {
		eveID, _ = u["id"].(string)
	}
	app.do("POST", "/api/admin/users/"+eveID+"/approve", adminTok, nil)
	eveTok, _ := app.login(t, "eve", "pass123")
	up3 := app.do("PUT", "/api/notes/"+noteID, eveTok, map[string]interface{}{"title": "篡改"})
	if up3.Status != 403 {
		t.Fatalf("非作者编辑应 403，实际 %d", up3.Status)
	}
}

// TestConcurrentHandlerLoad -race 并发读写
func TestConcurrentHandlerLoad(t *testing.T) {
	app, cleanup := newTestApp(t)
	defer cleanup()
	tok, adminID := app.login(t, "admin", "admin123")

	var wg sync.WaitGroup
	for g := 0; g < 6; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 15; i++ {
				// 写：管理员发笔记（AddNote + saveLocked）
				app.do("POST", "/api/notes", tok, map[string]interface{}{
					"title":     fmt.Sprintf("c%d-%d", g, i),
					"mediaType": "image",
					"media":     []string{"/uploads/" + adminID + "/x.jpg"},
				})
				// 读：feed 返回内部对象拷贝（旧实现此处会 data race）
				app.do("GET", "/api/feed", tok, nil)
				app.do("GET", "/api/admin/stats", tok, nil)
				app.do("GET", "/api/me", tok, nil)
			}
		}(g)
	}
	wg.Wait()
}

// --- 辅助 ---

func containsNoteID(notes []interface{}, id string) bool {
	for _, n := range notes {
		if m, ok := n.(map[string]interface{}); ok {
			if mid, _ := m["id"].(string); mid == id {
				return true
			}
		}
	}
	return false
}

func hasPasswordKey(m map[string]interface{}) bool {
	for k := range m {
		if k == "password" || k == "passwordHash" || k == "PasswordHash" {
			return true
		}
	}
	return false
}

// TestSensitiveNoteAndOfficialVerify 红标登录可见 + 官方认证流程
func TestSensitiveNoteAndOfficialVerify(t *testing.T) {
	app, cleanup := newTestApp(t)
	defer cleanup()

	adminTok, adminID := app.login(t, "admin", "admin123")

	// 1) 管理员发一篇笔记并定级为 red（红标=敏感内容）
	rn := app.do("POST", "/api/notes", adminTok, map[string]interface{}{
		"title": "红标测试", "content": "内容", "mediaType": "image",
		"media": []string{"/uploads/" + adminID + "/a.jpg"},
	})
	if rn.Status != 200 {
		t.Fatalf("发笔记失败 %d: %v", rn.Status, rn.Body)
	}
	noteID := ""
	if n, ok := rn.Body["note"].(map[string]interface{}); ok {
		noteID, _ = n["id"].(string)
	}
	rev := app.do("POST", "/api/admin/notes/"+noteID+"/review", adminTok, map[string]string{"level": "red"})
	if rev.Status != 200 {
		t.Fatalf("定级 red 失败 %d: %v", rev.Status, rev.Body)
	}

	// 2) 注册普通用户 bob 并通过审核
	reg := app.do("POST", "/api/register", "", map[string]string{
		"username": "bob", "nickname": "Bob", "password": "pass123",
	})
	if reg.Status != 200 {
		t.Fatalf("注册失败 %d: %v", reg.Status, reg.Body)
	}
	bobID := ""
	if u, ok := reg.Body["user"].(map[string]interface{}); ok {
		bobID, _ = u["id"].(string)
	}
	if app.do("POST", "/api/admin/users/"+bobID+"/approve", adminTok, nil).Status != 200 {
		t.Fatal("审核通过 bob 失败")
	}
	bobTok, _ := app.login(t, "bob", "pass123")

	// 3) 红标内容：普通登录用户即可浏览（返回 sensitive 标记，无需年龄认证）
	det := app.do("GET", "/api/notes/"+noteID, bobTok, nil)
	if det.Status != 200 {
		t.Fatalf("登录用户浏览红标内容应 200，实际 %d: %v", det.Status, det.Body)
	}
	if sn, _ := det.Body["sensitive"].(bool); !sn {
		t.Fatal("红标内容应返回 sensitive=true（前端需模糊封面+确认）")
	}

	// 4) 官方认证：用户提交材料申请 → 200
	req := app.do("POST", "/api/me/official/request", bobTok, map[string]interface{}{
		"material": "某某工作室主理人，提供设计服务",
		"files":    []string{"/uploads/" + bobID + "/cert.jpg"},
	})
	if req.Status != 200 {
		t.Fatalf("提交官方认证申请失败 %d: %v", req.Status, req.Body)
	}

	// 5) 重复申请 → 400
	req2 := app.do("POST", "/api/me/official/request", bobTok, map[string]interface{}{"material": "重复"})
	if req2.Status != 400 {
		t.Fatalf("重复申请应 400，实际 %d", req2.Status)
	}

	// 6) 管理后台待审核官方认证列表包含 bob（含材料）
	pend := app.do("GET", "/api/admin/users/official-pending", adminTok, nil)
	var plist []interface{}
	if err := json.Unmarshal(pend.Raw, &plist); err != nil || len(plist) == 0 {
		t.Fatalf("待审核官方认证列表为空 (raw=%s, err=%v)", string(pend.Raw), err)
	}
	found := false
	for _, it := range plist {
		if m, ok := it.(map[string]interface{}); ok && m["id"] == bobID {
			found = true
			if mat, _ := m["officialMaterial"].(string); mat == "" {
				t.Fatal("待审核列表应包含认证材料说明")
			}
		}
	}
	if !found {
		t.Fatal("待审核列表未包含 bob")
	}

	// 7) 管理员通过官方认证 → bob 的官方标识生效
	pass := app.do("POST", "/api/admin/users/"+bobID+"/official", adminTok, map[string]bool{"officialVerified": true})
	if pass.Status != 200 {
		t.Fatalf("通过官方认证失败 %d: %v", pass.Status, pass.Body)
	}
	me := app.do("GET", "/api/me", bobTok, nil)
	if ov, _ := me.Body["officialVerified"].(bool); !ov {
		t.Fatalf("bob 应有 officialVerified=true，实际 %v", me.Body)
	}

	// 8) 已认证后再次申请 → 400
	req3 := app.do("POST", "/api/me/official/request", bobTok, map[string]interface{}{"material": "again"})
	if req3.Status != 400 {
		t.Fatalf("已认证后申请应 400，实际 %d", req3.Status)
	}

	// 9) 新用户 carol 申请后被管理员拒绝 → 可再次申请
	reg2 := app.do("POST", "/api/register", "", map[string]string{
		"username": "carol", "nickname": "Carol", "password": "pass123",
	})
	carolID := ""
	if u, ok := reg2.Body["user"].(map[string]interface{}); ok {
		carolID, _ = u["id"].(string)
	}
	app.do("POST", "/api/admin/users/"+carolID+"/approve", adminTok, nil)
	carolTok, _ := app.login(t, "carol", "pass123")
	app.do("POST", "/api/me/official/request", carolTok, map[string]interface{}{"material": "材料"})
	rej := app.do("POST", "/api/admin/users/"+carolID+"/official-reject", adminTok, nil)
	if rej.Status != 200 {
		t.Fatalf("拒绝官方认证失败 %d: %v", rej.Status, rej.Body)
	}
	me2 := app.do("GET", "/api/me", carolTok, nil)
	if ov, _ := me2.Body["officialRejected"].(bool); !ov {
		t.Fatalf("carol 应有 officialRejected=true，实际 %v", me2.Body)
	}
	// 拒绝后可再次申请
	req4 := app.do("POST", "/api/me/official/request", carolTok, map[string]interface{}{"material": "再次申请"})
	if req4.Status != 200 {
		t.Fatalf("被拒后再次申请应 200，实际 %d: %v", req4.Status, req4.Body)
	}

	// 10) 公开接口（个人主页）展示官方标识，不泄露材料详情
	info := app.do("GET", "/api/user/"+bobID, carolTok, nil)
	uInfo, _ := info.Body["user"].(map[string]interface{})
	if ov, _ := uInfo["officialVerified"].(bool); !ov {
		t.Fatal("公开用户信息应展示 officialVerified")
	}
	if _, leak := uInfo["officialMaterial"]; leak {
		t.Fatal("公开用户信息不应泄露 officialMaterial（隐私）")
	}
}

// TestSessionPersistenceAndLoginIP 会话持久化 + 登录 IP 记录
func TestSessionPersistenceAndLoginIP(t *testing.T) {
	app, cleanup := newTestApp(t)
	defer cleanup()

	// 1) 登录，记录 token 与登录 IP
	req := httptest.NewRequest("POST", "/api/login", bytes.NewBufferString(`{"username":"admin","password":"admin123"}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "192.168.1.100:12345"
	rec := httptest.NewRecorder()
	app.mux.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("登录失败 %d", rec.Code)
	}
	var body map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	tok, _ := body["token"].(string)
	if tok == "" {
		t.Fatal("未拿到 token")
	}

	// 2) 管理员列表能看到该用户的登录 IP（用第一步登录的 token，避免再次登录覆盖 IP）
	users := app.do("GET", "/api/admin/users", tok, nil)
	var ulist []map[string]interface{}
	if err := json.Unmarshal(users.Raw, &ulist); err != nil {
		t.Fatalf("解析用户列表失败: %v", err)
	}
	found := false
	for _, u := range ulist {
		if u["username"] == "admin" {
			if ip, _ := u["lastLoginIP"].(string); ip != "192.168.1.100" {
				t.Fatalf("管理后台应显示登录 IP 192.168.1.100，实际 %q", ip)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("用户列表中未找到 admin")
	}

	// 3) 公开接口不应暴露 lastLoginIP（隐私）
	me := app.do("GET", "/api/me", tok, nil)
	if _, leak := me.Body["lastLoginIP"]; leak {
		t.Fatal("/api/me 不应暴露 lastLoginIP（隐私字段泄露）")
	}

	// 4) 会话持久化：token 写入磁盘后，重建 Store（模拟服务重启）仍有效
	tok2 := app.srv.Store.CreateSession("whatever")
	_ = tok2
	// 从同目录重建 store（模拟重启）
	st2 := store.New(app.dir)
	u2 := st2.UserByToken(tok)
	if u2 == nil {
		t.Fatal("服务重启后会话应仍有效（sessions.json 持久化失败）")
	}
	if u2.Username != "admin" {
		t.Fatalf("重启后会话应指向 admin，实际 %s", u2.Username)
	}
}
