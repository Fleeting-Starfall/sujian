package handler

import (
	"encoding/json"
	"net/http"
)

// dislike 用户标记某笔记「不感兴趣」：记负反馈，推荐时剔除该笔记并对同类标签降权。
// 前端不展示任何解释文案，仅静默生效。
func (s *Server) dislike(w http.ResponseWriter, r *http.Request) {
	u := s.requireLogin(w, r)
	if u == nil {
		return
	}
	var body struct {
		NoteID string `json:"noteID"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.NoteID == "" {
		s.writeJSON(w, 400, map[string]string{"error": "缺少 noteID"})
		return
	}
	// 不校验笔记是否存在：即使已下架，记下 ID 也能防止其重新出现时再推
	s.Store.AddDislike(u.ID, body.NoteID)
	s.writeJSON(w, 200, map[string]string{"message": "ok"})
}

// removeDislike 撤销「不感兴趣」：前端点击「撤销」时调用，使该笔记与同类标签恢复推荐。
func (s *Server) removeDislike(w http.ResponseWriter, r *http.Request) {
	u := s.requireLogin(w, r)
	if u == nil {
		return
	}
	var body struct {
		NoteID string `json:"noteID"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.NoteID == "" {
		s.writeJSON(w, 400, map[string]string{"error": "缺少 noteID"})
		return
	}
	s.Store.RemoveDislike(u.ID, body.NoteID)
	s.writeJSON(w, 200, map[string]string{"message": "ok"})
}
