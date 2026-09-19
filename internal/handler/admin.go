package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"sujian/internal/auth"
	"sujian/internal/model"
)

func (s *Server) adminStats(w http.ResponseWriter, r *http.Request) {
	if s.requireAdmin(w, r) == nil {
		return
	}
	s.writeJSON(w, 200, s.Store.Stats())
}

func (s *Server) adminPending(w http.ResponseWriter, r *http.Request) {
	if s.requireAdmin(w, r) == nil {
		return
	}
	s.writeJSON(w, 200, s.Store.PendingUsers())
}

func (s *Server) adminUsers(w http.ResponseWriter, r *http.Request) {
	if s.requireAdmin(w, r) == nil {
		return
	}
	s.writeJSON(w, 200, s.Store.AllUsersAdminView())
}

func (s *Server) adminApprove(w http.ResponseWriter, r *http.Request) {
	if s.requireAdmin(w, r) == nil {
		return
	}
	if err := s.Store.SetUserStatus(r.PathValue("id"), "active"); err != nil {
		s.writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	s.writeJSON(w, 200, map[string]string{"message": "已通过"})
}

func (s *Server) adminReject(w http.ResponseWriter, r *http.Request) {
	if s.requireAdmin(w, r) == nil {
		return
	}
	if err := s.Store.SetUserStatus(r.PathValue("id"), "rejected"); err != nil {
		s.writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	s.writeJSON(w, 200, map[string]string{"message": "已拒绝"})
}

// adminBanUser 设置/解除临时封锁（到期自动解封，封锁即踢出旧登录）
func (s *Server) adminBanUser(w http.ResponseWriter, r *http.Request) {
	if s.requireAdmin(w, r) == nil {
		return
	}
	var body struct {
		Hours float64 `json:"hours"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeJSON(w, 400, map[string]string{"error": "请求格式错误"})
		return
	}
	id := r.PathValue("id")
	var until int64
	if body.Hours > 0 {
		if body.Hours > 87600 { // 上限 10 年，防误输
			s.writeJSON(w, 400, map[string]string{"error": "封锁时长超出范围（最多 87600 小时）"})
			return
		}
		until = time.Now().Add(time.Duration(body.Hours * float64(time.Hour))).Unix()
	}
	if err := s.Store.SetUserBan(id, until); err != nil {
		s.writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	if until > 0 {
		s.Store.DeleteUserSessions(id) // 封锁后旧登录立即失效
		s.writeJSON(w, 200, map[string]string{"message": "已封锁至 " + time.Unix(until, 0).Format("2006-01-02 15:04")})
		return
	}
	s.writeJSON(w, 200, map[string]string{"message": "已解除封锁"})
}

func (s *Server) adminNotes(w http.ResponseWriter, r *http.Request) {
	if s.requireAdmin(w, r) == nil {
		return
	}
	notes := s.Store.AllNotes() // 全部状态（pending/published/rejected/removed）
	out := make([]map[string]interface{}, 0, len(notes))
	for _, n := range notes {
		v := s.Store.View(n, "")
		out = append(out, map[string]interface{}{
			"id": n.ID, "title": n.Title, "authorName": n.AuthorName,
			"category": n.Category, "mediaType": n.MediaType,
			"status": n.Status, "level": n.Level, "pinned": n.Pinned,
			"createdAt": n.CreatedAt,
			"likeCount": v.LikeCount, "commentCount": v.CommentCount,
		})
	}
	s.writeJSON(w, 200, out)
}

// adminReviewNote 审核定级
func (s *Server) adminReviewNote(w http.ResponseWriter, r *http.Request) {
	admin := s.requireAdmin(w, r)
	if admin == nil {
		return
	}
	id := r.PathValue("id")
	var body struct {
		Level string `json:"level"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeJSON(w, 400, map[string]string{"error": "请求格式错误"})
		return
	}
	n := s.Store.NoteRaw(id)
	if n == nil {
		s.writeJSON(w, 404, map[string]string{"error": "笔记不存在"})
		return
	}
	if err := s.Store.ReviewNote(id, "published", body.Level); err != nil {
		s.writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	// 通知作者审核结果
	if author := s.Store.UserByID(n.AuthorID); author != nil {
		s.notify(author.ID, admin, "approve", n.ID, n.Title, "")
	}
	s.writeJSON(w, 200, map[string]string{"message": "已通过审核"})
}

// adminRejectNote 驳回笔记
func (s *Server) adminRejectNote(w http.ResponseWriter, r *http.Request) {
	admin := s.requireAdmin(w, r)
	if admin == nil {
		return
	}
	id := r.PathValue("id")
	n := s.Store.NoteRaw(id)
	if n == nil {
		s.writeJSON(w, 404, map[string]string{"error": "笔记不存在"})
		return
	}
	if err := s.Store.ReviewNote(id, "rejected", ""); err != nil {
		s.writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	if author := s.Store.UserByID(n.AuthorID); author != nil {
		s.notify(author.ID, admin, "reject", n.ID, n.Title, "")
	}
	s.writeJSON(w, 200, map[string]string{"message": "已驳回"})
}

// adminBatchUsers 批量创建用户
func (s *Server) adminBatchUsers(w http.ResponseWriter, r *http.Request) {
	if s.requireAdmin(w, r) == nil {
		return
	}
	var body struct {
		Users []struct {
			Username string `json:"username"`
			Password string `json:"password"`
			Nickname string `json:"nickname"`
		} `json:"users"`
		OfficialVerified bool `json:"officialVerified"` // 统一设置官方认证
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeJSON(w, 400, map[string]string{"error": "请求格式错误"})
		return
	}
	if len(body.Users) == 0 {
		s.writeJSON(w, 400, map[string]string{"error": "请至少填写一个用户"})
		return
	}
	if len(body.Users) > 100 {
		s.writeJSON(w, 400, map[string]string{"error": "单次最多创建 100 个用户"})
		return
	}
	created := []model.PublicUser{}
	failed := []map[string]string{}
	for _, in := range body.Users {
		username := strings.TrimSpace(in.Username)
		nickname := strings.TrimSpace(in.Nickname)
		if username == "" || in.Password == "" {
			failed = append(failed, map[string]string{"username": username, "error": "用户名和密码不能为空"})
			continue
		}
		if len(username) < 2 || len(username) > 20 {
			failed = append(failed, map[string]string{"username": username, "error": "用户名需 2-20 位"})
			continue
		}
		if len(in.Password) < 6 {
			failed = append(failed, map[string]string{"username": username, "error": "密码至少 6 位"})
			continue
		}
		if nickname == "" {
			nickname = username
		}
		u, err := s.Store.AdminCreateUser(username, nickname, in.Password, body.OfficialVerified)
		if err != nil {
			failed = append(failed, map[string]string{"username": username, "error": err.Error()})
			continue
		}
		created = append(created, model.ToPublic(u))
	}
	s.writeJSON(w, 200, map[string]interface{}{"created": created, "failed": failed})
}

// adminAgeVerify 设置/取消年龄认证
func (s *Server) adminAgeVerify(w http.ResponseWriter, r *http.Request) {
	if s.requireAdmin(w, r) == nil {
		return
	}
	var body struct {
		AgeVerified bool `json:"ageVerified"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeJSON(w, 400, map[string]string{"error": "请求格式错误"})
		return
	}
	if err := s.Store.SetAgeVerified(r.PathValue("id"), body.AgeVerified); err != nil {
		s.writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	s.writeJSON(w, 200, map[string]bool{"ageVerified": body.AgeVerified})
}

// adminRejectAgeVerify 拒绝年龄认证申请
func (s *Server) adminRejectAgeVerify(w http.ResponseWriter, r *http.Request) {
	if s.requireAdmin(w, r) == nil {
		return
	}
	if err := s.Store.RejectAgeVerify(r.PathValue("id")); err != nil {
		s.writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	s.writeJSON(w, 200, map[string]string{"message": "已拒绝该认证申请"})
}

// adminAgeVerifyPending 待审核的年龄认证申请
func (s *Server) adminAgeVerifyPending(w http.ResponseWriter, r *http.Request) {
	if s.requireAdmin(w, r) == nil {
		return
	}
	s.writeJSON(w, 200, s.Store.PendingAgeVerifyUsers())
}

// adminOfficialVerify 通过/取消官方认证
func (s *Server) adminOfficialVerify(w http.ResponseWriter, r *http.Request) {
	if s.requireAdmin(w, r) == nil {
		return
	}
	var body struct {
		OfficialVerified bool `json:"officialVerified"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeJSON(w, 400, map[string]string{"error": "请求格式错误"})
		return
	}
	if err := s.Store.SetOfficialVerified(r.PathValue("id"), body.OfficialVerified); err != nil {
		s.writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	s.writeJSON(w, 200, map[string]bool{"officialVerified": body.OfficialVerified})
}

// adminRejectOfficialVerify 拒绝官方认证申请
func (s *Server) adminRejectOfficialVerify(w http.ResponseWriter, r *http.Request) {
	if s.requireAdmin(w, r) == nil {
		return
	}
	if err := s.Store.RejectOfficialVerify(r.PathValue("id")); err != nil {
		s.writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	s.writeJSON(w, 200, map[string]string{"message": "已拒绝该官方认证申请"})
}

// adminOfficialPending 待审核的官方认证申请
func (s *Server) adminOfficialPending(w http.ResponseWriter, r *http.Request) {
	if s.requireAdmin(w, r) == nil {
		return
	}
	s.writeJSON(w, 200, s.Store.PendingOfficialUsers())
}

// adminRemoveNote 下架笔记（软删除，30 天可恢复）
// 不删媒体文件，恢复后仍需展示
func (s *Server) adminRemoveNote(w http.ResponseWriter, r *http.Request) {
	if s.requireAdmin(w, r) == nil {
		return
	}
	id := r.PathValue("id")
	if err := s.Store.RemoveNote(id, "", true); err != nil {
		s.writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	s.writeJSON(w, 200, map[string]string{"message": "笔记已下架"})
}

// adminPurgeNote 永久删除笔记（不可恢复）
func (s *Server) adminPurgeNote(w http.ResponseWriter, r *http.Request) {
	if s.requireAdmin(w, r) == nil {
		return
	}
	id := r.PathValue("id")
	n := s.Store.NoteRaw(id)
	media, err := s.Store.PurgeNote(id)
	if err != nil {
		s.writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	if n != nil {
		s.cleanupMedia(n.AuthorID, media)
	}
	s.writeJSON(w, 200, map[string]string{"message": "已永久删除"})
}

// adminPurgeOldNotes 清理下架超 N 天的笔记
func (s *Server) adminPurgeOldNotes(w http.ResponseWriter, r *http.Request) {
	if s.requireAdmin(w, r) == nil {
		return
	}
	days := 30
	if v := r.URL.Query().Get("days"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			days = n // 0 = 立即清理所有下架笔记（宽限 0 天）
		}
	}
	n, mediaByAuthor, err := s.Store.PurgeOldRemovedNotes(time.Duration(days) * 24 * time.Hour)
	if err != nil {
		s.writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	// 媒体文件一并清理，避免孤儿文件
	for authorID, media := range mediaByAuthor {
		s.cleanupMedia(authorID, media)
	}
	s.writeJSON(w, 200, map[string]interface{}{"purged": n, "days": days, "message": fmt.Sprintf("已清理 %d 篇超过 %d 天的下架笔记", n, days)})
}

// adminResetPassword 重置用户密码
func (s *Server) adminResetPassword(w http.ResponseWriter, r *http.Request) {
	if s.requireAdmin(w, r) == nil {
		return
	}
	var body struct {
		NewPassword string `json:"newPassword"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeJSON(w, 400, map[string]string{"error": "请求格式错误"})
		return
	}
	if len(body.NewPassword) < 6 {
		s.writeJSON(w, 400, map[string]string{"error": "密码至少 6 位"})
		return
	}
	if err := s.Store.SetUserPassword(r.PathValue("id"), auth.HashPassword(body.NewPassword), body.NewPassword); err != nil {
		s.writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	// 改密后踢掉所有旧会话
	s.Store.DeleteUserSessions(r.PathValue("id"))
	s.writeJSON(w, 200, map[string]string{"message": "密码已重置"})
}

func (s *Server) adminComments(w http.ResponseWriter, r *http.Request) {
	if s.requireAdmin(w, r) == nil {
		return
	}
	notes := s.Store.AllNotes()
	out := []map[string]interface{}{}
	for _, n := range notes {
		for _, c := range s.Store.CommentsByNote(n.ID, "") {
			out = append(out, map[string]interface{}{
				"id": c.ID, "noteId": c.NoteID, "noteTitle": n.Title,
				"authorName": c.AuthorName, "content": c.Content, "createdAt": c.CreatedAt,
			})
		}
	}
	s.writeJSON(w, 200, out)
}

func (s *Server) adminDeleteComment(w http.ResponseWriter, r *http.Request) {
	if s.requireAdmin(w, r) == nil {
		return
	}
	if err := s.Store.DeleteComment(r.PathValue("id")); err != nil {
		s.writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	s.writeJSON(w, 200, map[string]string{"message": "评论已删除"})
}
