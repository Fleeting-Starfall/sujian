package handler

import (
	"net/http"
)

// myHistory 我的浏览历史：按最近浏览倒序返回笔记列表。
// 已删除/下架的笔记自动不展示；红标按等级可见性过滤。
func (s *Server) myHistory(w http.ResponseWriter, r *http.Request) {
	u := s.requireLogin(w, r)
	if u == nil {
		return
	}
	hist := s.Store.ViewHistory(u.ID)
	out := make([]map[string]interface{}, 0, len(hist))
	for _, h := range hist {
		n := s.Store.NoteRaw(h.NoteID)
		if n == nil || n.Status != "published" {
			continue // 笔记已删除/下架：历史里不再展示（保留也无意义，直接跳过）
		}
		if s.noteLevel(n) == "red" && !s.canViewLevel(n, u) {
			continue // 红标：未登录不可见（此处已登录，理论不会走到，防御性保留）
		}
		out = append(out, map[string]interface{}{
			"note":     s.Store.View(n, u.ID),
			"viewedAt": h.ViewedAt,
		})
	}
	s.writeJSON(w, 200, out)
}

// deleteHistory 删除历史中的一条
func (s *Server) deleteHistory(w http.ResponseWriter, r *http.Request) {
	u := s.requireLogin(w, r)
	if u == nil {
		return
	}
	s.Store.DeleteHistoryItem(u.ID, r.PathValue("id"))
	s.writeJSON(w, 200, map[string]string{"message": "已移除"})
}

// clearHistory 清空我的全部浏览历史
func (s *Server) clearHistory(w http.ResponseWriter, r *http.Request) {
	u := s.requireLogin(w, r)
	if u == nil {
		return
	}
	s.Store.ClearHistory(u.ID)
	s.writeJSON(w, 200, map[string]string{"message": "历史已清空"})
}
