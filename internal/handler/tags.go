package handler

import (
	"net/http"
	"sort"
	"strings"
)

// hotTags 高频标签 top 12
// 仅统计已发布笔记的标签
func (s *Server) hotTags(w http.ResponseWriter, r *http.Request) {
	if s.requireLogin(w, r) == nil {
		return
	}
	notes := s.Store.AllNotes()
	counts := make(map[string]int)
	for _, n := range notes {
		if n.Status != "published" {
			continue
		}
		for _, t := range n.Tags {
			t = strings.TrimSpace(t)
			if t != "" {
				counts[t]++
			}
		}
	}

	type tagCount struct {
		Tag   string `json:"tag"`
		Count int    `json:"count"`
	}
	list := make([]tagCount, 0, len(counts))
	for t, c := range counts {
		list = append(list, tagCount{Tag: t, Count: c})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].Count != list[j].Count {
			return list[i].Count > list[j].Count
		}
		return list[i].Tag < list[j].Tag
	})
	if len(list) > 12 {
		list = list[:12]
	}
	s.writeJSON(w, 200, list)
}
