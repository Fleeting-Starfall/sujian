package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"sujian/internal/model"
)

// messages 会话列表
func (s *Server) messages(w http.ResponseWriter, r *http.Request) {
	u := s.requireLogin(w, r)
	if u == nil {
		return
	}
	s.writeJSON(w, 200, s.Store.ConversationsOf(u.ID))
}

// messageUnreadCount 未读私信总数
func (s *Server) messageUnreadCount(w http.ResponseWriter, r *http.Request) {
	u := s.requireLogin(w, r)
	if u == nil {
		return
	}
	s.writeJSON(w, 200, map[string]int{"unread": s.Store.UnreadMessagesCount(u.ID)})
}

// messageThread 与某人的对话记录（拉取即标记已读）
func (s *Server) messageThread(w http.ResponseWriter, r *http.Request) {
	u := s.requireLogin(w, r)
	if u == nil {
		return
	}
	peerID := r.PathValue("id")
	peer := s.Store.UserByID(peerID)
	if peer == nil {
		s.writeJSON(w, 404, map[string]string{"error": "用户不存在"})
		return
	}
	s.Store.MarkMessagesRead(u.ID, peerID)
	msgs := s.Store.MessagesBetween(u.ID, peerID)
	s.writeJSON(w, 200, map[string]interface{}{
		"peer":     model.ToPublic(peer), // 仅公开字段，防泄露 passwordHash
		"messages": msgs,
	})
}

// messageSend 发送私信（文本 / 图片 / 视频 / 任意文件）
func (s *Server) messageSend(w http.ResponseWriter, r *http.Request) {
	u := s.requireActive(w, r)
	if u == nil {
		return
	}
	var body struct {
		To        string `json:"to"`
		Content   string `json:"content"`
		MediaType string `json:"mediaType"` // "" 文本 / "image" 图片 / "video" 视频 / "file" 文件
		Media     string `json:"media"`     // 文件路径（/uploads/<userID>/xxx）
		MediaName string `json:"mediaName"` // 原始文件名
		MediaSize int64  `json:"mediaSize"` // 文件字节数
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeJSON(w, 400, map[string]string{"error": "请求格式错误"})
		return
	}
	// 文件消息：只校验媒体路径（防穿越）
	switch body.MediaType {
	case "image", "video", "file":
		if body.Media == "" {
			s.writeJSON(w, 400, map[string]string{"error": "请上传文件"})
			return
		}
		if !strings.HasPrefix(body.Media, "/uploads/"+u.ID+"/") {
			s.writeJSON(w, 400, map[string]string{"error": "文件不属于当前账号"})
			return
		}
	default:
		body.MediaType = ""
		body.Content = strings.TrimSpace(body.Content)
		if body.Content == "" {
			s.writeJSON(w, 400, map[string]string{"error": "消息内容不能为空"})
			return
		}
		if len([]rune(body.Content)) > 500 {
			s.writeJSON(w, 400, map[string]string{"error": "消息最多 500 字"})
			return
		}
	}
	target := s.Store.UserByID(body.To)
	if target == nil || target.Status != "active" {
		s.writeJSON(w, 404, map[string]string{"error": "对方不存在或不可用"})
		return
	}
	if target.ID == u.ID {
		s.writeJSON(w, 400, map[string]string{"error": "不能给自己发私信"})
		return
	}
	m := s.Store.AddMessage(u.ID, u.Nickname, target.ID, body.Content, body.MediaType, body.Media, body.MediaName, body.MediaSize)
	s.writeJSON(w, 200, m)
}
