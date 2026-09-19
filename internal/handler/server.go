package handler

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"time"

	"sujian/internal/model"
	"sujian/internal/store"
)

// Config 服务配置（端口、管理员预置账号、上传限制等）
type Config struct {
	Port       int
	AdminUser  string
	AdminPass  string
	MaxImageMB int
	MaxVideoMB int
	UploadDir  string
	PublicDir  string
}

// Server 持有数据层与配置，所有 handler 均为其方法
type Server struct {
	Store *store.Store
	Cfg   Config
}

func (s *Server) currentUser(r *http.Request) *model.User {
	c, err := r.Cookie("session")
	if err != nil {
		return nil
	}
	return s.Store.UserByToken(c.Value)
}

func (s *Server) writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// userJSON 构造用户 JSON（对外的 PublicUser + 可选明文密码副本）。
// includePlain 只在「本人查看自己」或「管理员查看」时为 true；其余场景绝不含密码。
func userJSON(u *model.User, includePlain bool) map[string]interface{} {
	b, _ := json.Marshal(model.ToPublic(u))
	var m map[string]interface{}
	_ = json.Unmarshal(b, &m)
	if includePlain {
		m["passwordPlain"] = u.PasswordPlain
	}
	return m
}

func (s *Server) requireLogin(w http.ResponseWriter, r *http.Request) *model.User {
	u := s.currentUser(r)
	if u == nil {
		s.writeJSON(w, 401, map[string]string{"error": "未登录"})
		return nil
	}
	// 临时封锁在此统一拦截：封禁期间所有已登录接口(含只读)均拒绝，到期自动恢复
	if u.BannedUntil > time.Now().Unix() {
		s.writeJSON(w, 403, map[string]string{"error": "账号已被封锁，请于 " + time.Unix(u.BannedUntil, 0).Format("01-02 15:04") + " 后重试"})
		return nil
	}
	return u
}

func (s *Server) requireActive(w http.ResponseWriter, r *http.Request) *model.User {
	u := s.requireLogin(w, r)
	if u == nil {
		return nil
	}
	if u.Status != "active" {
		s.writeJSON(w, 403, map[string]string{"error": "账号未激活，请等待管理员审核"})
		return nil
	}
	return u
}

func (s *Server) requireAdmin(w http.ResponseWriter, r *http.Request) *model.User {
	u := s.requireLogin(w, r)
	if u == nil {
		return nil
	}
	if u.Role != "admin" {
		s.writeJSON(w, 403, map[string]string{"error": "需要管理员权限"})
		return nil
	}
	return u
}

// Page 返回一个带门禁的页面处理器。mode: public / auth / admin
func (s *Server) Page(file, mode string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if mode == "auth" && s.currentUser(r) == nil {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		if mode == "admin" {
			u := s.currentUser(r)
			if u == nil {
				http.Redirect(w, r, "/login", http.StatusFound)
				return
			}
			if u.Role != "admin" {
				http.Redirect(w, r, "/", http.StatusFound)
				return
			}
		}
		http.ServeFile(w, r, filepath.Join(s.Cfg.PublicDir, file))
	}
}
