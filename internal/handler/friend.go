package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"sujian/internal/model"
)

// userByUid 按 8 位用户号查找用户（加好友/搜索用），返回关系状态
func (s *Server) userByUid(w http.ResponseWriter, r *http.Request) {
	uid := strings.ToUpper(strings.TrimSpace(r.PathValue("uid"))) // 用户号大小写兼容
	target := s.Store.UserByUid(uid)
	if target == nil {
		s.writeJSON(w, 404, map[string]string{"error": "没有找到这个用户号"})
		return
	}
	me := s.currentUser(r)
	relation := "none"
	if me != nil {
		relation = s.Store.Relation(me.ID, target.ID)
	}
	s.writeJSON(w, 200, map[string]interface{}{
		"user":     model.ToPublic(target),
		"relation": relation,
	})
}

// friendRequest 按用户号发送好友请求
func (s *Server) friendRequest(w http.ResponseWriter, r *http.Request) {
	u := s.requireActive(w, r)
	if u == nil {
		return
	}
	var body struct {
		Uid string `json:"uid"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeJSON(w, 400, map[string]string{"error": "请求格式错误"})
		return
	}
	req, err := s.Store.SendFriendRequest(u.ID, strings.ToUpper(strings.TrimSpace(body.Uid)))
	if err != nil {
		s.writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	s.notify(req.ToID, u, "friend", "", "", "") // 请求方 → 接收方
	s.writeJSON(w, 200, req)
}

// friendRequests 我收到的 + 我发出的好友请求
func (s *Server) friendRequests(w http.ResponseWriter, r *http.Request) {
	u := s.requireLogin(w, r)
	if u == nil {
		return
	}
	s.writeJSON(w, 200, map[string]interface{}{
		"received": s.Store.FriendRequestsTo(u.ID),
		"sent":     s.Store.FriendRequestsFrom(u.ID),
	})
}

// friendAccept 接受好友请求
func (s *Server) friendAccept(w http.ResponseWriter, r *http.Request) {
	u := s.requireActive(w, r)
	if u == nil {
		return
	}
	id := r.PathValue("id")
	req := s.Store.ReqByID(id)
	if err := s.Store.AcceptFriendRequest(id, u.ID); err != nil {
		s.writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	if req != nil {
		if from := s.Store.UserByID(req.FromID); from != nil {
			s.notify(from.ID, u, "friendAccepted", "", "", "")
		}
	}
	s.writeJSON(w, 200, map[string]string{"message": "已成为好友"})
}

// friendReject 拒绝好友请求
func (s *Server) friendReject(w http.ResponseWriter, r *http.Request) {
	u := s.requireActive(w, r)
	if u == nil {
		return
	}
	if err := s.Store.RejectFriendRequest(r.PathValue("id"), u.ID); err != nil {
		s.writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	s.writeJSON(w, 200, map[string]string{"message": "已拒绝"})
}

// friendList 我的好友列表
func (s *Server) friendList(w http.ResponseWriter, r *http.Request) {
	u := s.requireLogin(w, r)
	if u == nil {
		return
	}
	out := []model.PublicUser{}
	for _, f := range s.Store.FriendsOf(u.ID) {
		out = append(out, model.ToPublic(f))
	}
	s.writeJSON(w, 200, out)
}

// friendRemove 删除好友
func (s *Server) friendRemove(w http.ResponseWriter, r *http.Request) {
	u := s.requireActive(w, r)
	if u == nil {
		return
	}
	if err := s.Store.RemoveFriend(u.ID, r.PathValue("id")); err != nil {
		s.writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	s.writeJSON(w, 200, map[string]string{"message": "已删除好友"})
}
