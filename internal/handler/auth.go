package handler

import (
	"encoding/json"
	"net"
	"net/http"
	"regexp"
	"strings"

	"sujian/internal/auth"
	"sujian/internal/model"
)

// 用户名：字母/数字/下划线/中文，2-20 位。
var usernameRe = regexp.MustCompile(`^[A-Za-z0-9_\p{Han}]+$`)

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username  string   `json:"username"`
		Nickname  string   `json:"nickname"`
		Password  string   `json:"password"`
		Interests []string `json:"interests"` // 注册时自选兴趣标签（可选，冷启动弱画像）
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeJSON(w, 400, map[string]string{"error": "请求格式错误"})
		return
	}
	body.Username = strings.TrimSpace(body.Username)
	if body.Username == "" || body.Password == "" {
		s.writeJSON(w, 400, map[string]string{"error": "用户名和密码不能为空"})
		return
	}
	if len([]rune(body.Username)) < 2 || len([]rune(body.Username)) > 20 {
		s.writeJSON(w, 400, map[string]string{"error": "用户名长度需为 2-20 个字符"})
		return
	}
	if !usernameRe.MatchString(body.Username) {
		s.writeJSON(w, 400, map[string]string{"error": "用户名仅支持字母、数字、下划线和中文"})
		return
	}
	if len([]rune(body.Nickname)) > 24 {
		s.writeJSON(w, 400, map[string]string{"error": "昵称最长 24 个字符"})
		return
	}
	if len(body.Password) < 6 {
		s.writeJSON(w, 400, map[string]string{"error": "密码至少 6 位"})
		return
	}
	body.Nickname = strings.TrimSpace(body.Nickname)
	if body.Nickname == "" {
		body.Nickname = body.Username
	}
	u, err := s.Store.Register(body.Username, body.Nickname, body.Password, body.Interests)
	if err != nil {
		s.writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	s.writeJSON(w, 200, map[string]interface{}{
		"message": "注册申请已提交，等待管理员审核通过后即可登录",
		"user":    model.ToPublic(u),
	})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeJSON(w, 400, map[string]string{"error": "请求格式错误"})
		return
	}
	u, err := s.Store.Login(strings.TrimSpace(body.Username), body.Password)
	if err != nil {
		s.writeJSON(w, 401, map[string]string{"error": err.Error()})
		return
	}
	token := s.Store.CreateSession(u.ID)
	// 记录登录 IP
	s.Store.RecordLogin(u.ID, s.clientIP(r))
	// 会话 30 天有效
	http.SetCookie(w, &http.Cookie{
		Name: "session", Value: token, Path: "/",
		HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: 30 * 24 * 3600,
	})
	s.writeJSON(w, 200, map[string]interface{}{"token": token, "user": model.ToPublic(u)})
}

// clientIP 取客户端真实 IP
func (s *Server) clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ip := strings.Split(xff, ",")[0]
		if ip = strings.TrimSpace(ip); ip != "" {
			return ip
		}
	}
	if xr := r.Header.Get("X-Real-IP"); xr != "" {
		return xr
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie("session"); err == nil {
		s.Store.DeleteSession(c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: "session", Value: "", Path: "/", MaxAge: -1})
	s.writeJSON(w, 200, map[string]string{"message": "已退出"})
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	u := s.currentUser(r)
	if u == nil {
		s.writeJSON(w, 401, map[string]string{"error": "未登录"})
		return
	}
	// 本人查看自己：附带明文密码副本
	s.writeJSON(w, 200, userJSON(u, true))
}

// changePassword 改密
// 验旧密码；成功后踢掉全部会话
func (s *Server) changePassword(w http.ResponseWriter, r *http.Request) {
	u := s.requireActive(w, r)
	if u == nil {
		return
	}
	var body struct {
		OldPassword string `json:"oldPassword"`
		NewPassword string `json:"newPassword"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeJSON(w, 400, map[string]string{"error": "请求格式错误"})
		return
	}
	if body.OldPassword == "" || body.NewPassword == "" {
		s.writeJSON(w, 400, map[string]string{"error": "旧密码和新密码不能为空"})
		return
	}
	if len(body.NewPassword) < 6 {
		s.writeJSON(w, 400, map[string]string{"error": "新密码至少 6 位"})
		return
	}
	if body.OldPassword == body.NewPassword {
		s.writeJSON(w, 400, map[string]string{"error": "新密码不能与旧密码相同"})
		return
	}
	if err := s.Store.ChangePassword(u.ID, body.OldPassword, auth.HashPassword(body.NewPassword), body.NewPassword); err != nil {
		s.writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	// 改密后使所有旧会话失效
	s.Store.DeleteUserSessions(u.ID)
	s.writeJSON(w, 200, map[string]string{"message": "密码已修改，请重新登录"})
}

// requestAgeVerify 提交年龄认证申请
func (s *Server) requestAgeVerify(w http.ResponseWriter, r *http.Request) {
	u := s.currentUser(r)
	if u == nil {
		s.writeJSON(w, 401, map[string]string{"error": "未登录"})
		return
	}
	if err := s.Store.RequestAgeVerify(u.ID); err != nil {
		s.writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	s.writeJSON(w, 200, map[string]interface{}{"message": "成年认证申请已提交，请等待管理员审核"})
}

// requestOfficialVerify 提交官方认证申请
func (s *Server) requestOfficialVerify(w http.ResponseWriter, r *http.Request) {
	u := s.currentUser(r)
	if u == nil {
		s.writeJSON(w, 401, map[string]string{"error": "未登录"})
		return
	}
	var body struct {
		Material string   `json:"material"`
		Files    []string `json:"files"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeJSON(w, 400, map[string]string{"error": "请求格式错误"})
		return
	}
	if strings.TrimSpace(body.Material) == "" && len(body.Files) == 0 {
		s.writeJSON(w, 400, map[string]string{"error": "请填写认证材料说明或上传材料附件"})
		return
	}
	// 附件必须属于当前账号目录，防路径穿越
	for _, f := range body.Files {
		if !strings.HasPrefix(f, "/uploads/"+u.ID+"/") {
			s.writeJSON(w, 400, map[string]string{"error": "材料附件不属于当前账号"})
			return
		}
	}
	if err := s.Store.RequestOfficialVerify(u.ID, strings.TrimSpace(body.Material), body.Files); err != nil {
		s.writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	s.writeJSON(w, 200, map[string]interface{}{"message": "官方认证申请已提交，请等待管理员审核"})
}
