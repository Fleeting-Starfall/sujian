package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"sujian/internal/model"
)

// report 提交举报：笔记或评论
func (s *Server) report(w http.ResponseWriter, r *http.Request) {
	u := s.requireActive(w, r)
	if u == nil {
		return
	}
	target := r.PathValue("target") // note / comment
	if target != "note" && target != "comment" {
		s.writeJSON(w, 400, map[string]string{"error": "无效的举报类型"})
		return
	}
	targetID := r.PathValue("id")
	var body struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeJSON(w, 400, map[string]string{"error": "请求格式错误"})
		return
	}
	body.Reason = strings.TrimSpace(body.Reason)
	if body.Reason == "" {
		s.writeJSON(w, 400, map[string]string{"error": "请填写举报理由"})
		return
	}
	if len([]rune(body.Reason)) > 200 {
		s.writeJSON(w, 400, map[string]string{"error": "举报理由最多 200 字"})
		return
	}
	title := ""
	if target == "note" {
		n := s.Store.NoteRaw(targetID)
		if n == nil || n.Status != "published" {
			s.writeJSON(w, 404, map[string]string{"error": "笔记不存在"})
			return
		}
		if n.AuthorID == u.ID {
			s.writeJSON(w, 400, map[string]string{"error": "不能举报自己的笔记"})
			return
		}
		title = n.Title
	} else {
		c := s.Store.CommentByID(targetID)
		if c == nil {
			s.writeJSON(w, 404, map[string]string{"error": "评论不存在"})
			return
		}
		if c.AuthorID == u.ID {
			s.writeJSON(w, 400, map[string]string{"error": "不能举报自己的评论"})
			return
		}
		title = c.Content
		if len([]rune(title)) > 40 {
			title = string([]rune(title)[:40]) + "…"
		}
	}
	rp, err := s.Store.AddReport(target, targetID, u.ID, u.Nickname, body.Reason, title)
	if err != nil {
		s.writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	s.writeJSON(w, 200, rp)
}

// userSearch 搜索用户：昵称 / 用户名 / 用户号（大小写不敏感）
func (s *Server) userSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		s.writeJSON(w, 200, []model.PublicUser{})
		return
	}
	ql := strings.ToLower(q)
	qu := strings.ToUpper(q)
	out := []model.PublicUser{}
	for _, u := range s.Store.AllUsers() { // []model.PublicUser
		if u.Status != "active" {
			continue // 未通过/被禁用户不展示
		}
		if strings.Contains(strings.ToLower(u.Nickname), ql) ||
			strings.Contains(strings.ToLower(u.Username), ql) ||
			(u.Uid != "" && strings.Contains(u.Uid, qu)) {
			out = append(out, u)
		}
	}
	s.writeJSON(w, 200, out)
}

// adminReports 举报列表（管理后台）
func (s *Server) adminReports(w http.ResponseWriter, r *http.Request) {
	if s.requireAdmin(w, r) == nil {
		return
	}
	s.writeJSON(w, 200, s.Store.AllReports())
}

// adminResolveReport 处理举报：resolved=已处理（删除内容由前端另行调接口）/ ignored=忽略
func (s *Server) adminResolveReport(w http.ResponseWriter, r *http.Request) {
	if s.requireAdmin(w, r) == nil {
		return
	}
	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeJSON(w, 400, map[string]string{"error": "请求格式错误"})
		return
	}
	if body.Status != "resolved" && body.Status != "ignored" {
		s.writeJSON(w, 400, map[string]string{"error": "状态必须是 resolved / ignored"})
		return
	}
	if err := s.Store.SetReportStatus(r.PathValue("id"), body.Status); err != nil {
		s.writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	s.writeJSON(w, 200, map[string]string{"message": "已处理"})
}
