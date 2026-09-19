package handler

import (
	"encoding/json"
	"net/http"
)

// dislike「不感兴趣」：推荐剔除并降权同类标签
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
	// 已下架也记录，防再次推荐
	s.Store.AddDislike(u.ID, body.NoteID)
	s.writeJSON(w, 200, map[string]string{"message": "ok"})
}

// removeDislike 撤销「不感兴趣」
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
