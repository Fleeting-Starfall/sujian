package handler

import "net/http"

// notifications 我的消息通知（含未读数）
func (s *Server) notifications(w http.ResponseWriter, r *http.Request) {
	u := s.requireLogin(w, r)
	if u == nil {
		return
	}
	s.writeJSON(w, 200, map[string]interface{}{
		"items":  s.Store.NotificationsOf(u.ID),
		"unread": s.Store.UnreadCount(u.ID),
	})
}

// notificationsReadAll 全部已读
func (s *Server) notificationsReadAll(w http.ResponseWriter, r *http.Request) {
	u := s.requireLogin(w, r)
	if u == nil {
		return
	}
	s.Store.MarkAllRead(u.ID)
	s.writeJSON(w, 200, map[string]string{"message": "已全部标记为已读"})
}

// notificationRead 单条已读
func (s *Server) notificationRead(w http.ResponseWriter, r *http.Request) {
	u := s.requireLogin(w, r)
	if u == nil {
		return
	}
	s.Store.MarkRead(r.PathValue("id"), u.ID)
	s.writeJSON(w, 200, map[string]string{"message": "ok"})
}
