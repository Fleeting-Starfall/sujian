package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"sujian/internal/model"
)

// saveDraft 保存草稿（id 为空则新建）
func (s *Server) saveDraft(w http.ResponseWriter, r *http.Request) {
	u := s.requireActive(w, r)
	if u == nil {
		return
	}
	var body struct {
		ID        string   `json:"id"`
		Title     string   `json:"title"`
		Content   string   `json:"content"`
		MediaType string   `json:"mediaType"`
		Media     []string `json:"media"`
		Cover     string   `json:"cover"`
		Tags      []string `json:"tags"`
		Category  string   `json:"category"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeJSON(w, 400, map[string]string{"error": "请求格式错误"})
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
	d := &model.Draft{
		ID: body.ID, UserID: u.ID, Title: body.Title, Content: body.Content,
		MediaType: body.MediaType, Media: body.Media, Cover: body.Cover,
		Tags: cleanTags(body.Tags), Category: body.Category,
	}
	saved, err := s.Store.SaveDraft(d)
	if err != nil {
		s.writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	s.writeJSON(w, 200, saved)
}

// listDrafts 我的草稿（按更新时间倒序）
func (s *Server) listDrafts(w http.ResponseWriter, r *http.Request) {
	u := s.requireLogin(w, r)
	if u == nil {
		return
	}
	s.writeJSON(w, 200, s.Store.DraftsOf(u.ID))
}

// deleteDraft 删除草稿
func (s *Server) deleteDraft(w http.ResponseWriter, r *http.Request) {
	u := s.requireActive(w, r)
	if u == nil {
		return
	}
	if err := s.Store.DeleteDraft(r.PathValue("id"), u.ID); err != nil {
		s.writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	s.writeJSON(w, 200, map[string]string{"message": "草稿已删除"})
}
