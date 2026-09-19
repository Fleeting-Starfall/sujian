package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"sujian/internal/auth"
	"sujian/internal/model"
)

// notify 给目标用户写入一条消息通知（自己触发自己不通知，由 store 过滤）
func (s *Server) notify(targetID string, actor *model.User, typ, noteID, noteTitle, commentID string) {
	s.Store.AddNotification(&model.Notification{
		ID: auth.ID("nt_"), UserID: targetID, ActorID: actor.ID, ActorName: actor.Nickname,
		Type: typ, NoteID: noteID, NoteTitle: noteTitle, CommentID: commentID,
		CreatedAt: time.Now().Unix(),
	})
}

func (s *Server) feed(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	category := q.Get("category")
	keyword := q.Get("q")
	tab := q.Get("tab") // follow / "" (推荐)
	viewer := s.currentUser(r)
	viewerID := ""
	if viewer != nil {
		viewerID = viewer.ID
	}
	following := map[string]bool(nil)
	if tab == "follow" && viewer != nil {
		following = s.Store.FollowingSet(viewer.ID)
	}
	sortParam := q.Get("sort")
	if sortParam != "hot" {
		sortParam = "new"
	}
	// 首页「推荐」频道（综合排序、已登录）走个性化智能推荐；其余（分类/关注/最新/搜索）保持原逻辑
	var notes []*model.Note
	if tab == "" && category == "" && keyword == "" && sortParam == "hot" && viewer != nil {
		notes = s.Store.RecommendForUser(viewer.ID, 200)
	} else {
		notes = s.Store.NotesList(category, keyword, following, sortParam)
	}
	// 内容等级过滤：
	//   首页信息流绿/黄/红标均可展示（红标在前端做模糊+确认）；黑标绝不进首页/推荐，仅搜索可见。
	//   isSearch=true 表示用户主动搜索（黑标此时可见）。
	filtered := make([]*model.Note, 0, len(notes))
	isSearch := keyword != ""
	for _, n := range notes {
		if !s.noteVisible(n, viewer, isSearch) {
			continue
		}
		filtered = append(filtered, n)
	}
	notes = filtered
	page := 0
	if p, err := strconv.Atoi(q.Get("page")); err == nil && p > 0 {
		page = p
	}
	const size = 20
	start := page * size
	end := start + size
	if start > len(notes) {
		start = len(notes)
	}
	if end > len(notes) {
		end = len(notes)
	}
	views := make([]model.NoteView, 0, end-start)
	for _, n := range notes[start:end] {
		// 首页瀑布流仅做展示，不累计浏览量、不记浏览历史。
		// 浏览量与「我的历史」只在用户真正打开笔记详情（noteDetail）时记录，
		// 避免「划过卡片」被算作一次浏览——曝光不等于浏览。
		views = append(views, s.Store.View(n, viewerID))
	}
	s.writeJSON(w, 200, map[string]interface{}{
		"items":   views,
		"total":   len(notes),
		"hasMore": end < len(notes),
	})
}

func (s *Server) noteDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	n := s.Store.NoteRaw(id)
	if n == nil || n.Status == "removed" {
		s.writeJSON(w, 404, map[string]string{"error": "笔记不存在或已下架"})
		return
	}
	viewer := s.currentUser(r)
	viewerID := ""
	isAdmin := false
	if viewer != nil {
		viewerID = viewer.ID
		isAdmin = viewer.Role == "admin"
	}
	// 审核可见性
	if n.Status == "pending" {
		if !isAdmin && viewerID != n.AuthorID {
			s.writeJSON(w, 404, map[string]string{"error": "笔记不存在或已下架"})
			return
		}
		view := s.Store.View(n, viewerID)
		s.writeJSON(w, 200, map[string]interface{}{"note": view, "comments": []*model.Comment{}, "following": false, "related": []model.NoteView{}, "pending": true})
		return
	}
	if n.Status == "rejected" {
		// 作者本人/管理员可见（带"未通过"标记），其余人 404
		if !isAdmin && viewerID != n.AuthorID {
			s.writeJSON(w, 404, map[string]string{"error": "笔记不存在或已下架"})
			return
		}
		view := s.Store.View(n, viewerID)
		s.writeJSON(w, 200, map[string]interface{}{"note": view, "comments": []*model.Comment{}, "following": false, "related": []model.NoteView{}, "rejected": true})
		return
	}
	// 已发布：作者被禁则普通用户不可见（管理员豁免）
	if author := s.Store.UserByID(n.AuthorID); author != nil && author.Status != "active" && !isAdmin {
		s.writeJSON(w, 404, map[string]string{"error": "笔记不存在或已下架"})
		return
	}
	// 已发布：红标/黑标为敏感内容，所有登录用户可浏览（前端做模糊封面+确认提示）
	if !s.canViewLevel(n, viewer) {
		s.writeJSON(w, 401, map[string]string{"error": "请先登录后查看该内容"})
		return
	}
	if viewer == nil || viewer.ID != n.AuthorID {
		s.Store.IncViews(id, 1) // 浏览数 +1（作者本人查看不计入）
		if viewer != nil {
			s.Store.RecordView(viewer.ID, id) // 登录用户浏览 → 记入「我的历史」
		}
		n = s.Store.NoteRaw(id) // 重新取最新快照：让本次响应立即返回 +1 后的浏览量
	}
	view := s.Store.View(n, viewerID)
	order := r.URL.Query().Get("order") // 评论排序：hot(按回复数) / new(最新)
	comments := s.Store.CommentsByNote(id, order)
	s.Store.FillCommentViews(comments, viewerID) // 评论"我赞过"状态
	following := viewer != nil && s.Store.IsFollowing(viewer.ID, n.AuthorID)
	related := make([]model.NoteView, 0, 4)
	for _, rn := range s.Store.RelatedNotes(n.ID, n.Category, 4) {
		related = append(related, s.Store.View(rn, viewerID))
	}
	s.writeJSON(w, 200, map[string]interface{}{
		"note": view, "comments": comments, "following": following, "related": related,
		"sensitive":     s.noteLevel(n) == "red" || s.noteLevel(n) == "black", // 红/黑标：前端需模糊封面+确认后才能查看
		"sensitiveLevel": s.noteLevel(n), // green/yellow/red/black；前端据此决定提示强度与文案
	})
}

// noteLevel 等级归一：空等级按 green 处理（老数据兼容）
func (s *Server) noteLevel(n *model.Note) string {
	if n.Level == "" {
		return "green"
	}
	return n.Level
}

// canViewLevel 判定某用户能否浏览某等级内容（红/黑标为敏感内容：登录用户均可浏览，前端做模糊+确认）
func (s *Server) canViewLevel(n *model.Note, viewer *model.User) bool {
	lv := s.noteLevel(n)
	if lv == "red" || lv == "black" {
		return viewer != nil
	}
	return true
}

// noteVisible 判断某笔记在「浏览类」场景下是否对 viewer 可见。
//   - 黑标最敏感：作者/管理员始终可见；其余用户仅「已登录 + 主动搜索」时可见（比红标更严，与详情页一致）
//   - 黑标在浏览/分类/首页/标签/他人主页等场景下隐藏
//   - 红标需登录可见；绿/黄标始终可见
func (s *Server) noteVisible(n *model.Note, viewer *model.User, isSearch bool) bool {
	lv := s.noteLevel(n)
	if lv == "black" {
		if viewer != nil && (viewer.Role == "admin" || viewer.ID == n.AuthorID || isSearch) {
			return true
		}
		return false
	}
	if lv == "red" {
		return viewer != nil
	}
	return true
}

// deleteComment 删除评论（作者本人或管理员）
func (s *Server) deleteComment(w http.ResponseWriter, r *http.Request) {
	u := s.requireActive(w, r)
	if u == nil {
		return
	}
	if err := s.Store.DeleteCommentFor(r.PathValue("id"), u.ID, u.Role == "admin"); err != nil {
		s.writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	s.writeJSON(w, 200, map[string]string{"message": "评论已删除"})
}

// commentLike 评论点赞/取消赞；点赞时通知评论作者
func (s *Server) commentLike(w http.ResponseWriter, r *http.Request) {
	u := s.requireActive(w, r)
	if u == nil {
		return
	}
	cid := r.PathValue("id")
	c := s.Store.CommentByID(cid)
	if c == nil {
		s.writeJSON(w, 404, map[string]string{"error": "评论不存在"})
		return
	}
	on := s.Store.ToggleCommentLike(cid, u.ID)
	if on && c.AuthorID != u.ID {
		n := s.Store.NoteRaw(c.NoteID)
		title := ""
		if n != nil {
			title = n.Title
		}
		s.notify(c.AuthorID, u, "commentLike", c.NoteID, title, cid)
	}
	lc := 0
	if cc := s.Store.CommentByID(cid); cc != nil {
		lc = cc.LikeCount
	}
	s.writeJSON(w, 200, map[string]interface{}{"liked": on, "likeCount": lc})
}

// cleanTags 清理标签：去首尾空格、去重、单标签最长 20 字、最多 10 个。
// 统一用于发布笔记 / 编辑笔记 / 保存草稿，保证标签数据一致。
func cleanTags(tags []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		if len([]rune(t)) > 20 {
			t = string([]rune(t)[:20])
		}
		key := strings.ToLower(t)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, t)
		if len(out) >= 10 {
			break
		}
	}
	return out
}

func (s *Server) createNote(w http.ResponseWriter, r *http.Request) {
	u := s.requireActive(w, r)
	if u == nil {
		return
	}
	var body struct {
		Title     string   `json:"title"`
		Content   string   `json:"content"`
		MediaType string   `json:"mediaType"` // image / video
		Media     []string `json:"media"`
		Cover     string   `json:"cover"`
		Tags      []string `json:"tags"`
		Category  string   `json:"category"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeJSON(w, 400, map[string]string{"error": "请求格式错误"})
		return
	}
	if body.MediaType != "image" && body.MediaType != "video" {
		body.MediaType = "image"
	}
	if len(body.Media) == 0 {
		s.writeJSON(w, 400, map[string]string{"error": "请至少上传一张图片或一个视频"})
		return
	}
	for _, m := range body.Media {
		if !strings.HasPrefix(m, "/uploads/"+u.ID+"/") {
			s.writeJSON(w, 400, map[string]string{"error": "媒体文件不属于当前账号"})
			return
		}
	}
	if body.Cover != "" && !strings.HasPrefix(body.Cover, "/uploads/"+u.ID+"/") {
		s.writeJSON(w, 400, map[string]string{"error": "封面文件不属于当前账号"})
		return
	}
	body.Title = strings.TrimSpace(body.Title)
	if body.Title == "" {
		body.Title = "无标题笔记"
	}
	if len([]rune(body.Title)) > 60 {
		s.writeJSON(w, 400, map[string]string{"error": "标题最长 60 个字符"})
		return
	}
	if len([]rune(body.Content)) > 10000 {
		s.writeJSON(w, 400, map[string]string{"error": "正文过长，最多 10000 字"})
		return
	}
	cleanTags := cleanTags(body.Tags)
	// 新笔记默认进入待审核（pending）；管理员自己发布直接通过并标绿
	status, level := "pending", ""
	if u.Role == "admin" {
		status, level = "published", "green"
	}
	n := s.Store.AddNote(u, body.Title, body.Content, body.MediaType, body.Media, body.Cover, cleanTags, body.Category, status, level)
	view := s.Store.View(n, u.ID)
	view.Status = status
	view.Level = level
	if status == "pending" {
		view.Level = "" // 待审核无等级
	}
	s.writeJSON(w, 200, map[string]interface{}{"note": view})
}

func (s *Server) deleteNote(w http.ResponseWriter, r *http.Request) {
	u := s.requireActive(w, r)
	if u == nil {
		return
	}
	id := r.PathValue("id")
	n := s.Store.NoteRaw(id)
	if n == nil {
		s.writeJSON(w, 404, map[string]string{"error": "笔记不存在"})
		return
	}
	isAdmin := u.Role == "admin"
	if err := s.Store.RemoveNote(id, u.ID, isAdmin); err != nil {
		s.writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	s.cleanupMedia(n.AuthorID, n.Media)
	s.writeJSON(w, 200, map[string]string{"message": "已删除"})
}

// cleanupMedia 删除笔记对应的上传文件（仅限该作者目录内的路径，防穿越）
func (s *Server) cleanupMedia(authorID string, media []string) {
	for _, p := range media {
		prefix := "/uploads/" + authorID + "/"
		if !strings.HasPrefix(p, prefix) {
			continue
		}
		rel := strings.TrimPrefix(p, "/uploads/")
		if strings.Contains(rel, "..") {
			continue
		}
		_ = os.Remove(filepath.Join(s.Cfg.UploadDir, rel))
	}
}

func (s *Server) like(w http.ResponseWriter, r *http.Request) {
	u := s.requireActive(w, r)
	if u == nil {
		return
	}
	n := s.Store.NoteByID(r.PathValue("id"))
	if n == nil {
		s.writeJSON(w, 404, map[string]string{"error": "笔记不存在"})
		return
	}
	on := s.Store.ToggleLike(n.ID, u.ID)
	if on {
		s.notify(n.AuthorID, u, "like", n.ID, n.Title, "")
	}
	// 返回服务端权威计数，前端直接用（避免多端操作时本地 ±1 猜测导致显示漂移）
	s.writeJSON(w, 200, map[string]interface{}{"liked": on, "likeCount": s.Store.LikeCount(n.ID)})
}

func (s *Server) favorite(w http.ResponseWriter, r *http.Request) {
	u := s.requireActive(w, r)
	if u == nil {
		return
	}
	n := s.Store.NoteByID(r.PathValue("id"))
	if n == nil {
		s.writeJSON(w, 404, map[string]string{"error": "笔记不存在"})
		return
	}
	on := s.Store.ToggleFav(n.ID, u.ID)
	if on {
		s.notify(n.AuthorID, u, "fav", n.ID, n.Title, "")
	}
	// 服务端权威计数，前端直接用
	s.writeJSON(w, 200, map[string]interface{}{"favorited": on, "favoriteCount": s.Store.FavCount(n.ID)})
}

func (s *Server) comment(w http.ResponseWriter, r *http.Request) {
	u := s.requireActive(w, r)
	if u == nil {
		return
	}
	noteID := r.PathValue("id")
	n := s.Store.NoteByID(noteID)
	if n == nil {
		s.writeJSON(w, 404, map[string]string{"error": "笔记不存在"})
		return
	}
	var body struct {
		Content  string `json:"content"`
		ParentID string `json:"parentId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeJSON(w, 400, map[string]string{"error": "请求格式错误"})
		return
	}
	body.Content = strings.TrimSpace(body.Content)
	if body.Content == "" {
		s.writeJSON(w, 400, map[string]string{"error": "评论内容不能为空"})
		return
	}
	if len([]rune(body.Content)) > 500 {
		s.writeJSON(w, 400, map[string]string{"error": "评论最多 500 字"})
		return
	}
	replyTo, parentAuthor := "", ""
	if body.ParentID != "" {
		pc := s.Store.CommentByID(body.ParentID)
		if pc == nil || pc.NoteID != noteID {
			s.writeJSON(w, 400, map[string]string{"error": "回复的评论不存在"})
			return
		}
		replyTo, parentAuthor = pc.AuthorName, pc.AuthorID
	}
	c := s.Store.AddComment(noteID, u.ID, u.Nickname, body.Content, body.ParentID, replyTo)
	if body.ParentID != "" && parentAuthor != "" && parentAuthor != u.ID {
		s.notify(parentAuthor, u, "reply", noteID, n.Title, c.ID)
	} else if n.AuthorID != u.ID {
		s.notify(n.AuthorID, u, "comment", noteID, n.Title, "")
	}
	s.writeJSON(w, 200, c)
}

func (s *Server) userInfo(w http.ResponseWriter, r *http.Request) {
	u := s.Store.UserByID(r.PathValue("id"))
	if u == nil {
		s.writeJSON(w, 404, map[string]string{"error": "用户不存在"})
		return
	}
	me := s.currentUser(r)
	following := false
	relation := "none"
	includePlain := false // 明文密码副本：仅本人查看自己 / 管理员查看 时可见
	if me != nil {
		if me.ID != u.ID {
			following = s.Store.IsFollowing(me.ID, u.ID)
			relation = s.Store.Relation(me.ID, u.ID)
		}
		includePlain = me.ID == u.ID || me.Role == "admin"
	}
	s.writeJSON(w, 200, map[string]interface{}{
		"user":      userJSON(u, includePlain),
		"counts":    s.Store.UserCounts(u.ID),
		"following": following,
		"relation":  relation,
	})
}

// updateMe 编辑自己的资料（昵称/简介/头像）
func (s *Server) updateMe(w http.ResponseWriter, r *http.Request) {
	u := s.requireActive(w, r)
	if u == nil {
		return
	}
	var body struct {
		Nickname string  `json:"nickname"`
		Bio      *string `json:"bio"` // 指针：支持显式清空简介
		Avatar   string  `json:"avatar"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeJSON(w, 400, map[string]string{"error": "请求格式错误"})
		return
	}
	body.Nickname = strings.TrimSpace(body.Nickname)
	if body.Nickname == "" && body.Bio == nil && body.Avatar == "" {
		s.writeJSON(w, 400, map[string]string{"error": "没有要修改的内容"})
		return
	}
	if len([]rune(body.Nickname)) > 24 {
		s.writeJSON(w, 400, map[string]string{"error": "昵称最长 24 个字符"})
		return
	}
	if body.Bio != nil && len([]rune(*body.Bio)) > 120 {
		s.writeJSON(w, 400, map[string]string{"error": "简介最长 120 个字符"})
		return
	}
	if err := s.Store.UpdateProfile(u.ID, body.Nickname, body.Bio, body.Avatar); err != nil {
		s.writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	// 换头像时清理旧头像文件（仅限本人目录，防穿越）
	if body.Avatar != "" && body.Avatar != u.Avatar && u.Avatar != "" && strings.HasPrefix(u.Avatar, "/uploads/"+u.ID+"/") {
		rel := strings.TrimPrefix(u.Avatar, "/uploads/")
		if !strings.Contains(rel, "..") {
			_ = os.Remove(filepath.Join(s.Cfg.UploadDir, rel))
		}
	}
	s.writeJSON(w, 200, model.ToPublic(s.Store.UserByID(u.ID)))
}

// myFavorites 我收藏的笔记
func (s *Server) myFavorites(w http.ResponseWriter, r *http.Request) {
	u := s.requireLogin(w, r)
	if u == nil {
		return
	}
	notes := s.Store.FavNotesOf(u.ID)
	views := make([]model.NoteView, 0, len(notes))
	for _, n := range notes {
		if n.Status != "published" {
			continue
		}
		if !s.canViewLevel(n, u) {
			continue // 红/黑标：未登录不可见（此处已登录，理论不会走到，防御性保留）
		}
		views = append(views, s.Store.View(n, u.ID))
	}
	s.writeJSON(w, 200, views)
}

func (s *Server) userNotes(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	notes := s.Store.NotesByAuthor(id) // 置顶优先，再按时间倒序（仅已发布）
	viewer := s.currentUser(r)
	viewerID := ""
	isSelf := viewer != nil && viewer.ID == id
	if viewer != nil {
		viewerID = viewer.ID
	}
	out := []model.NoteView{}
	for _, n := range notes {
		if !s.noteVisible(n, viewer, false) {
			continue // 黑标不对外展示（仅作者本人/管理员/搜索可见）
		}
		out = append(out, s.Store.View(n, viewerID))
	}
	// 作者本人额外可见待审核/已驳回笔记（带状态标记），统一按时间倒序排列
	if isSelf {
		pending := s.Store.NotesByAuthorAll(id)
		for _, n := range pending {
			if n.Status != "published" {
				out = append(out, s.Store.View(n, viewerID))
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	s.writeJSON(w, 200, out)
}

// pinNote 置顶/取消置顶自己的笔记
func (s *Server) pinNote(w http.ResponseWriter, r *http.Request) {
	u := s.requireActive(w, r)
	if u == nil {
		return
	}
	var body struct {
		Pinned bool `json:"pinned"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeJSON(w, 400, map[string]string{"error": "请求格式错误"})
		return
	}
	if err := s.Store.SetPin(r.PathValue("id"), u.ID, body.Pinned); err != nil {
		s.writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	s.writeJSON(w, 200, map[string]bool{"pinned": body.Pinned})
}

func (s *Server) follow(w http.ResponseWriter, r *http.Request) {
	u := s.requireActive(w, r)
	if u == nil {
		return
	}
	targetID := r.PathValue("id")
	target := s.Store.UserByID(targetID)
	if target == nil {
		s.writeJSON(w, 404, map[string]string{"error": "用户不存在"})
		return
	}
	if target.ID == u.ID {
		s.writeJSON(w, 400, map[string]string{"error": "不能关注自己"})
		return
	}
	on := s.Store.ToggleFollow(u.ID, targetID)
	if on {
		s.notify(target.ID, u, "follow", "", "", "")
	}
	s.writeJSON(w, 200, map[string]bool{"following": on})
}

// updateNote 作者编辑笔记。编辑后强制重新进入审核（pending），管理员重新定级后才重新可见
func (s *Server) updateNote(w http.ResponseWriter, r *http.Request) {
	u := s.requireActive(w, r)
	if u == nil {
		return
	}
	id := r.PathValue("id")
	raw := s.Store.NoteRaw(id)
	if raw == nil || raw.Status == "removed" {
		s.writeJSON(w, 404, map[string]string{"error": "笔记不存在或已删除"})
		return
	}
	// 已发布/待审核/被驳回的笔记都只能作者本人编辑（被驳回可修改后重新提交）
	if raw.AuthorID != u.ID {
		s.writeJSON(w, 403, map[string]string{"error": "只能编辑自己的笔记"})
		return
	}
	var body struct {
		Title    string   `json:"title"`
		Content  string   `json:"content"`
		Tags     []string `json:"tags"`
		Category string   `json:"category"`
		Cover    string   `json:"cover"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeJSON(w, 400, map[string]string{"error": "请求格式错误"})
		return
	}
	if body.Title == "" && body.Content == "" && len(body.Tags) == 0 && body.Category == "" && body.Cover == "" {
		s.writeJSON(w, 400, map[string]string{"error": "没有要修改的内容"})
		return
	}
	if len([]rune(body.Title)) > 60 {
		s.writeJSON(w, 400, map[string]string{"error": "标题最长 60 个字符"})
		return
	}
	if len([]rune(body.Content)) > 10000 {
		s.writeJSON(w, 400, map[string]string{"error": "正文过长，最多 10000 字"})
		return
	}
	if body.Cover != "" && !strings.HasPrefix(body.Cover, "/uploads/"+u.ID+"/") {
		s.writeJSON(w, 400, map[string]string{"error": "封面文件不属于当前账号"})
		return
	}
	cleanTags := cleanTags(body.Tags)
	if err := s.Store.UpdateNote(id, u.ID, strings.TrimSpace(body.Title), body.Content, cleanTags, body.Category, body.Cover); err != nil {
		s.writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	n := s.Store.NoteRaw(id)
	if n == nil {
		s.writeJSON(w, 404, map[string]string{"error": "笔记不存在"})
		return
	}
	view := s.Store.View(n, u.ID)
	s.writeJSON(w, 200, map[string]interface{}{
		"note":    view,
		"message": "修改已提交，笔记将重新进入审核，管理员审核定级后才会对其他用户可见",
	})
}

func (s *Server) userFollowers(w http.ResponseWriter, r *http.Request) {
	s.followList(w, r, s.Store.FollowersOf(r.PathValue("id")))
}

func (s *Server) userFollowing(w http.ResponseWriter, r *http.Request) {
	s.followList(w, r, s.Store.FollowingUsers(r.PathValue("id")))
}

func (s *Server) followList(w http.ResponseWriter, r *http.Request, users []*model.User) {
	me := s.currentUser(r)
	out := []map[string]interface{}{}
	for _, u := range users {
		following := me != nil && me.ID != u.ID && s.Store.IsFollowing(me.ID, u.ID)
		out = append(out, map[string]interface{}{"user": model.ToPublic(u), "following": following})
	}
	s.writeJSON(w, 200, out)
}

// tagNotes 标签聚合页
func (s *Server) tagNotes(w http.ResponseWriter, r *http.Request) {
	tag := r.PathValue("tag")
	notes := s.Store.NotesByTag(tag)
	viewer := s.currentUser(r)
	viewerID := ""
	if viewer != nil {
		viewerID = viewer.ID
	}
	views := make([]model.NoteView, 0, len(notes))
	for _, n := range notes {
		if !s.noteVisible(n, viewer, false) {
			continue // 黑标不对外展示（仅作者本人/管理员/搜索可见）
		}
		views = append(views, s.Store.View(n, viewerID))
	}
	s.writeJSON(w, 200, map[string]interface{}{"tag": tag, "total": len(views), "items": views})
}
