package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"sujian/internal/auth"
	"sujian/internal/model"
)

// Store 单文件 JSON 持久化 + 内存索引 + 会话管理。
// 进程内加锁保证并发安全；每次写操作后原子落盘（写临时文件再 rename）。
// RecoConfig 推荐打分权重配置（运营可调，data/reco_config.json）。
// 任一字段为 0 或缺失时回退到 DefaultRecoConfig 对应值。
type RecoConfig struct {
	WTag              float64 `json:"w_tag"`               // 兴趣画像权重
	WCes              float64 `json:"w_ces"`               // CES 互动分权重
	WRec              float64 `json:"w_rec"`               // 新鲜度权重
	WFollow           float64 `json:"w_follow"`            // 已关注作者社交权重
	WCf               float64 `json:"w_cf"`                // 协同过滤（Item-CF）权重
	DislikeTagPenalty float64 `json:"dislike_tag_penalty"` // 命中不感兴趣标签的每项降权
	MaxPerTag         int     `json:"max_per_tag"`         // 普通用户的单标签上限（防茧房）
	ColdMaxPerTag     int     `json:"cold_max_per_tag"`    // 冷启动（无真实行为）用户的单标签上限（更强多样）
}

// DefaultRecoConfig 默认权重（与历史硬编码一致）。
func DefaultRecoConfig() RecoConfig {
	return RecoConfig{
		WTag: 1.0, WCes: 5.0, WRec: 3.0, WFollow: 5.0, WCf: 2.0,
		DislikeTagPenalty: 1.2, MaxPerTag: 3, ColdMaxPerTag: 1,
	}
}

// loadRecoConfig 读取 data/reco_config.json；缺失或解析失败时保留默认值。
func (s *Store) loadRecoConfig() {
	cfg := DefaultRecoConfig()
	if b, err := os.ReadFile(filepath.Join(s.dir, "reco_config.json")); err == nil {
		var raw RecoConfig
		if json.Unmarshal(b, &raw) == nil {
			if raw.WTag != 0 {
				cfg.WTag = raw.WTag
			}
			if raw.WCes != 0 {
				cfg.WCes = raw.WCes
			}
			if raw.WRec != 0 {
				cfg.WRec = raw.WRec
			}
			if raw.WFollow != 0 {
				cfg.WFollow = raw.WFollow
			}
			if raw.WCf != 0 {
				cfg.WCf = raw.WCf
			}
			if raw.DislikeTagPenalty != 0 {
				cfg.DislikeTagPenalty = raw.DislikeTagPenalty
			}
			if raw.MaxPerTag != 0 {
				cfg.MaxPerTag = raw.MaxPerTag
			}
			if raw.ColdMaxPerTag != 0 {
				cfg.ColdMaxPerTag = raw.ColdMaxPerTag
			}
		}
	}
	s.RecoConfig = cfg
}

type Store struct {
	mu      sync.RWMutex
	dir     string
	Users   map[string]*model.User
	Notes   map[string]*model.Note
	Commen  map[string]*model.Comment
	Likes   map[string][]string // noteID -> [userID]
	Favs    map[string][]string // noteID -> [userID]
	Follow  map[string][]string // userID -> [followingUserID]
	Notifs  map[string]*model.Notification
	Drafts  map[string]*model.Draft
	Friends map[string][]string // userID -> [friendUserID]（双向确认）
	Reqs    map[string]*model.FriendRequest
	Reports map[string]*model.Report
	CLikes  map[string][]string // commentID -> [userID] 评论点赞
	Msgs    map[string]*model.Message
	// 浏览历史：userID -> noteID -> 最近浏览时间戳（同一篇只保留一条，上限 maxViewHistory 条）
	ViewHist map[string]map[string]int64
	// 负反馈（不感兴趣）：userID -> noteID -> true。用于推荐纠偏：跳过该笔记并对同类标签降权。
	Dislikes map[string]map[string]bool

	// 推荐权重配置（运营可调，data/reco_config.json；缺失项用默认值）
	RecoConfig RecoConfig

	sessMu  sync.RWMutex
	session map[string]string // token -> userID

	// 浏览量节流落盘：浏览量属于高频低价值数据，累计变更或距上次落盘超过阈值才写盘，
	// 避免每次浏览都全量写 JSON。崩溃最多丢 ~30s 的浏览数（进程退出时由 Save 兜底）。
	viewsDirty    int
	viewsLastSave time.Time

	// 浏览量去重：同一用户对同一笔记 30 分钟内只算 1 次浏览，避免疯狂刷新首页导致数字虚高。
	viewDedup   map[string]int64 // "userID|noteID" -> 最近一次浏览 unix 时间戳
	viewDedupMu sync.Mutex
}

func New(dir string) *Store {
	s := &Store{
		dir:       dir,
		Users:     map[string]*model.User{},
		Notes:     map[string]*model.Note{},
		Commen:    map[string]*model.Comment{},
		Likes:     map[string][]string{},
		Favs:      map[string][]string{},
		Follow:    map[string][]string{},
		Notifs:    map[string]*model.Notification{},
		Drafts:    map[string]*model.Draft{},
		Friends:   map[string][]string{},
		Reqs:      map[string]*model.FriendRequest{},
		Reports:   map[string]*model.Report{},
		CLikes:    map[string][]string{},
		Msgs:      map[string]*model.Message{},
		ViewHist:  map[string]map[string]int64{},
		Dislikes:  map[string]map[string]bool{},
		session:   map[string]string{},
		viewDedup: map[string]int64{},
	}
	s.viewsLastSave = time.Now()
	s.load()
	s.loadRecoConfig() // 推荐权重配置（缺失则用默认）
	s.ensureUids()     // 老用户补齐 8 位用户号
	return s
}

func (s *Store) load() {
	_ = os.MkdirAll(s.dir, 0o755)
	s.readJSON("users.json", &s.Users)
	s.readJSON("notes.json", &s.Notes)
	s.readJSON("comments.json", &s.Commen)
	s.readJSON("likes.json", &s.Likes)
	s.readJSON("favorites.json", &s.Favs)
	s.readJSON("follows.json", &s.Follow)
	s.readJSON("notifications.json", &s.Notifs)
	s.readJSON("drafts.json", &s.Drafts)
	s.readJSON("friends.json", &s.Friends)
	s.readJSON("friend_requests.json", &s.Reqs)
	s.readJSON("reports.json", &s.Reports)
	s.readJSON("comment_likes.json", &s.CLikes)
	s.readJSON("messages.json", &s.Msgs)
	s.readJSON("view_history.json", &s.ViewHist)
	s.readJSON("dislikes.json", &s.Dislikes)
	// 会话持久化：服务重启后登录状态仍保留
	s.readJSON("sessions.json", &s.session)
}

// ensureUids 为缺少用户号的老用户补齐 8 位唯一用户号
func (s *Store) ensureUids() {
	s.mu.Lock()
	defer s.mu.Unlock()
	changed := false
	for _, u := range s.Users {
		if u.Uid == "" {
			u.Uid = s.newUidLocked()
			changed = true
		}
	}
	if changed {
		s.saveLocked("users.json")
	}
}

// newUidLocked 生成未占用的 8 位用户号（调用方需持写锁）
func (s *Store) newUidLocked() string {
	for {
		candidate := auth.RandChars(8)
		dup := false
		for _, u := range s.Users {
			if u.Uid == candidate {
				dup = true
				break
			}
		}
		if !dup {
			return candidate
		}
	}
}

// UserByUid 按用户号查找用户
func (s *Store) UserByUid(uid string) *model.User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, u := range s.Users {
		if u.Uid == uid {
			cp := *u // 返回拷贝，避免调用方在锁外持有内部对象引发 data race
			return &cp
		}
	}
	return nil
}

func (s *Store) readJSON(name string, v interface{}) {
	data, err := os.ReadFile(filepath.Join(s.dir, name))
	if err != nil {
		return
	}
	_ = json.Unmarshal(data, v)
}

// Save 原子落盘（全量）。不持有锁时调用（自行加读锁）。
// 进程退出（SIGINT/SIGTERM）时兜底全量保存，保证任何遗漏的修改都不丢。
func (s *Store) Save() {
	s.mu.RLock()
	defer s.mu.RUnlock()
	s.saveLocked()
}

// saveLocked 落盘。调用方必须已持有 s.mu 锁（读或写）。
// 性能优化：写方法只落盘自己修改的文件（传入文件名），避免每次小写操作
// 都全量序列化全部 13 个数据文件（写放大）。无参数时全量落盘（Save 兜底）。
func (s *Store) saveLocked(names ...string) {
	if len(names) == 0 {
		s.writeJSON("users.json", s.Users)
		s.writeJSON("notes.json", s.Notes)
		s.writeJSON("comments.json", s.Commen)
		s.writeJSON("likes.json", s.Likes)
		s.writeJSON("favorites.json", s.Favs)
		s.writeJSON("follows.json", s.Follow)
		s.writeJSON("notifications.json", s.Notifs)
		s.writeJSON("drafts.json", s.Drafts)
		s.writeJSON("friends.json", s.Friends)
		s.writeJSON("friend_requests.json", s.Reqs)
		s.writeJSON("reports.json", s.Reports)
		s.writeJSON("comment_likes.json", s.CLikes)
		s.writeJSON("messages.json", s.Msgs)
		s.writeJSON("view_history.json", s.ViewHist)
		s.writeJSON("dislikes.json", s.Dislikes)
		return
	}
	for _, name := range names {
		switch name {
		case "users.json":
			s.writeJSON("users.json", s.Users)
		case "notes.json":
			s.writeJSON("notes.json", s.Notes)
		case "comments.json":
			s.writeJSON("comments.json", s.Commen)
		case "likes.json":
			s.writeJSON("likes.json", s.Likes)
		case "favorites.json":
			s.writeJSON("favorites.json", s.Favs)
		case "follows.json":
			s.writeJSON("follows.json", s.Follow)
		case "notifications.json":
			s.writeJSON("notifications.json", s.Notifs)
		case "drafts.json":
			s.writeJSON("drafts.json", s.Drafts)
		case "friends.json":
			s.writeJSON("friends.json", s.Friends)
		case "friend_requests.json":
			s.writeJSON("friend_requests.json", s.Reqs)
		case "reports.json":
			s.writeJSON("reports.json", s.Reports)
		case "comment_likes.json":
			s.writeJSON("comment_likes.json", s.CLikes)
		case "messages.json":
			s.writeJSON("messages.json", s.Msgs)
		case "view_history.json":
			s.writeJSON("view_history.json", s.ViewHist)
		case "dislikes.json":
			s.writeJSON("dislikes.json", s.Dislikes)
		}
	}
}

func (s *Store) writeJSON(name string, v interface{}) {
	// 紧凑序列化（去掉缩进空白），所有 data/*.json 落盘体积约减 20-30%
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	p := filepath.Join(s.dir, name)
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, p)
}

// ---------- 会话 ----------

// saveSessionsLocked 持久化会话表（调用方需已持有 sessMu）
func (s *Store) saveSessionsLocked() {
	s.writeJSON("sessions.json", s.session)
}

func (s *Store) CreateSession(userID string) string {
	t := auth.Token()
	s.sessMu.Lock()
	s.session[t] = userID
	s.saveSessionsLocked()
	s.sessMu.Unlock()
	return t
}

func (s *Store) UserByToken(t string) *model.User {
	if t == "" {
		return nil
	}
	s.sessMu.RLock()
	uid, ok := s.session[t]
	s.sessMu.RUnlock()
	if !ok {
		return nil
	}
	s.mu.RLock()
	u, ok := s.Users[uid]
	if !ok {
		s.mu.RUnlock()
		return nil
	}
	cp := *u // 持锁期间完成拷贝，避免与写锁内的修改构成 data race
	s.mu.RUnlock()
	return &cp
}

func (s *Store) DeleteSession(t string) {
	s.sessMu.Lock()
	delete(s.session, t)
	s.saveSessionsLocked()
	s.sessMu.Unlock()
}

// DeleteUserSessions 使某用户的所有会话失效（重置密码时调用，踢掉该用户所有旧登录）
func (s *Store) DeleteUserSessions(userID string) {
	s.sessMu.Lock()
	for t, uid := range s.session {
		if uid == userID {
			delete(s.session, t)
		}
	}
	s.saveSessionsLocked()
	s.sessMu.Unlock()
}

// RecordLogin 记录用户最近一次登录 IP 与时间（用于后台展示）
func (s *Store) RecordLogin(userID, ip string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.Users[userID]
	if !ok {
		return
	}
	u.LastLoginIP = ip
	u.LastLoginAt = time.Now().Unix()
	s.saveLocked("users.json")
}

// ---------- 注册 / 登录 / 审核 ----------

func (s *Store) Register(username, nickname, pw string, interests []string) (*model.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, u := range s.Users {
		if u.Username == username {
			return nil, fmt.Errorf("用户名已被占用")
		}
	}
	// 清洗兴趣标签：去空、去重、上限 10 个
	clean := []string{}
	seen := map[string]bool{}
	for _, t := range interests {
		t = strings.TrimSpace(t)
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		clean = append(clean, t)
		if len(clean) >= 10 {
			break
		}
	}
	u := &model.User{
		ID:            auth.ID("u_"),
		Uid:           s.newUidLocked(), // 8 位唯一用户号
		Username:      username,
		PasswordHash:  auth.HashPassword(pw),
		PasswordPlain: pw, // 明文密码副本：管理员/本人可见
		Nickname:      nickname,
		Interests:     clean,
		Role:          "user",
		Status:        "pending", // 需管理员审核
		CreatedAt:     time.Now().Unix(),
	}
	s.Users[u.ID] = u
	s.saveLocked("users.json")
	return u, nil
}

func (s *Store) Login(username, pw string) (*model.User, error) {
	s.mu.RLock()
	var found *model.User
	for _, u := range s.Users {
		if u.Username == username {
			found = u
			break
		}
	}
	if found == nil {
		s.mu.RUnlock()
		return nil, fmt.Errorf("用户不存在")
	}
	cp := *found // 持锁期间完成拷贝，避免与写锁内的修改构成 data race
	s.mu.RUnlock()
	if !auth.CheckPassword(pw, cp.PasswordHash) {
		return nil, fmt.Errorf("密码错误")
	}
	if cp.BannedUntil > time.Now().Unix() {
		return nil, fmt.Errorf("账号已被封锁，请于 %s 后重试", time.Unix(cp.BannedUntil, 0).Format("01-02 15:04"))
	}
	if cp.Status != "active" {
		if cp.Status == "pending" {
			return nil, fmt.Errorf("账号待审核，请联系管理员")
		}
		return nil, fmt.Errorf("账号审核未通过")
	}
	return &cp, nil
}

// SeedAdmin 首次启动若无任何管理员，则创建预置管理员
func (s *Store) SeedAdmin(username, pw string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, u := range s.Users {
		if u.Role == "admin" {
			return
		}
	}
	u := &model.User{
		ID:            auth.ID("u_"),
		Uid:           s.newUidLocked(),
		Username:      username,
		PasswordHash:  auth.HashPassword(pw),
		PasswordPlain: pw, // 明文密码副本
		Nickname:      "管理员",
		Role:          "admin",
		Status:        "active",
		CreatedAt:     time.Now().Unix(),
	}
	s.Users[u.ID] = u
	s.saveLocked("users.json")
}

// AdminCreateUser 管理员批量创建用户：直接 active，可设定官方认证
func (s *Store) AdminCreateUser(username, nickname, pw string, officialVerified bool) (*model.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, u := range s.Users {
		if u.Username == username {
			return nil, fmt.Errorf("用户名 %s 已被占用", username)
		}
	}
	u := &model.User{
		ID:               auth.ID("u_"),
		Uid:              s.newUidLocked(),
		Username:         username,
		PasswordHash:     auth.HashPassword(pw),
		PasswordPlain:    pw, // 明文密码副本
		Nickname:         nickname,
		Role:             "user",
		Status:           "active", // 管理员创建，直接可用
		OfficialVerified: officialVerified,
		CreatedAt:        time.Now().Unix(),
	}
	s.Users[u.ID] = u
	s.saveLocked("users.json")
	return u, nil
}

// SetAgeVerified 设置/取消用户年龄认证（成年标记）。管理员通过审核时清除申请状态；取消认证时一并重置申请状态
func (s *Store) SetAgeVerified(id string, v bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.Users[id]
	if !ok {
		return fmt.Errorf("用户不存在")
	}
	u.AgeVerified = v
	u.AgeVerifyPending = false
	u.AgeVerifyRejected = false
	s.saveLocked("users.json")
	return nil
}

// RequestAgeVerify 用户主动提交成年认证申请（待管理员审核）
func (s *Store) RequestAgeVerify(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.Users[id]
	if !ok {
		return fmt.Errorf("用户不存在")
	}
	if u.AgeVerified {
		return fmt.Errorf("您已完成成年认证，无需重复申请")
	}
	if u.AgeVerifyPending {
		return fmt.Errorf("认证申请已提交，请等待管理员审核")
	}
	u.AgeVerifyPending = true
	u.AgeVerifyRejected = false
	s.saveLocked("users.json")
	return nil
}

// RejectAgeVerify 管理员拒绝用户的成年认证申请（用户可再次申请）
func (s *Store) RejectAgeVerify(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.Users[id]
	if !ok {
		return fmt.Errorf("用户不存在")
	}
	u.AgeVerifyPending = false
	u.AgeVerifyRejected = true
	s.saveLocked("users.json")
	return nil
}

// PendingAgeVerifyUsers 返回已提交成年认证申请、待管理员审核的用户
func (s *Store) PendingAgeVerifyUsers() []model.PublicUser {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []model.PublicUser{}
	for _, u := range s.Users {
		if u.AgeVerifyPending {
			out = append(out, model.ToPublic(u))
		}
	}
	return out
}

// RequestOfficialVerify 用户提交官方认证申请（材料说明 + 附件），待管理员审核
func (s *Store) RequestOfficialVerify(id, material string, files []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.Users[id]
	if !ok {
		return fmt.Errorf("用户不存在")
	}
	if u.OfficialVerified {
		return fmt.Errorf("您已完成官方认证")
	}
	if u.OfficialPending {
		return fmt.Errorf("官方认证申请已提交，请等待管理员审核")
	}
	u.OfficialPending = true
	u.OfficialRejected = false
	u.OfficialMaterial = material
	u.OfficialFiles = files
	s.saveLocked("users.json")
	return nil
}

// SetOfficialVerified 管理员通过/取消用户的官方认证
func (s *Store) SetOfficialVerified(id string, v bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.Users[id]
	if !ok {
		return fmt.Errorf("用户不存在")
	}
	u.OfficialVerified = v
	u.OfficialPending = false
	u.OfficialRejected = false
	s.saveLocked("users.json")
	return nil
}

// RejectOfficialVerify 管理员拒绝官方认证申请（用户可再次申请）
func (s *Store) RejectOfficialVerify(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.Users[id]
	if !ok {
		return fmt.Errorf("用户不存在")
	}
	u.OfficialPending = false
	u.OfficialRejected = true
	s.saveLocked("users.json")
	return nil
}

// PendingOfficialUsers 返回已提交官方认证申请、待管理员审核的用户
func (s *Store) PendingOfficialUsers() []model.AdminUserView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []model.AdminUserView{}
	for _, u := range s.Users {
		if u.OfficialPending {
			out = append(out, model.ToAdminView(u))
		}
	}
	return out
}

// SetUserStatus 修改账号状态（active / pending / rejected / …）。
// 管理员账号不允许被置为非 active：否则系统会失去唯一可登录的管理员，
// 后台再也进不去，只能手工改 data/users.json 才能恢复（与 SetUserBan 的保护保持一致）。
func (s *Store) SetUserStatus(id, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.Users[id]
	if !ok {
		return fmt.Errorf("用户不存在")
	}
	if u.Role == "admin" && status != "active" {
		return fmt.Errorf("不能停用管理员账号")
	}
	u.Status = status
	s.saveLocked("users.json")
	return nil
}

// SetUserBan 设置/解除临时封锁。until=0 表示解除封锁；>0 表示封锁至该时间戳。
// 到期自动解封：鉴权处实时比较 BannedUntil 与当前时间，无需定时任务。
func (s *Store) SetUserBan(id string, until int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.Users[id]
	if !ok {
		return fmt.Errorf("用户不存在")
	}
	if u.Role == "admin" {
		return fmt.Errorf("不能封锁管理员账号")
	}
	u.BannedUntil = until
	s.saveLocked("users.json")
	return nil
}

// PendingUsers 返回待审核用户（内部加锁，供管理后台调用）
func (s *Store) PendingUsers() []model.PublicUser {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []model.PublicUser{}
	for _, u := range s.Users {
		if u.Status == "pending" {
			out = append(out, model.ToPublic(u))
		}
	}
	return out
}

// AllUsers 返回全部用户
func (s *Store) AllUsers() []model.PublicUser {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []model.PublicUser{}
	for _, u := range s.Users {
		out = append(out, model.ToPublic(u))
	}
	return out
}

// AllUsersAdminView 返回全部用户（含登录 IP 等隐私，仅供管理后台）
func (s *Store) AllUsersAdminView() []model.AdminUserView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []model.AdminUserView{}
	for _, u := range s.Users {
		out = append(out, model.ToAdminView(u))
	}
	return out
}

// UserByID 按 ID 查用户
func (s *Store) UserByID(id string) *model.User {
	s.mu.RLock()
	u, ok := s.Users[id]
	if !ok {
		s.mu.RUnlock()
		return nil
	}
	cp := *u // 持锁期间完成拷贝，避免与写锁内的修改构成 data race
	s.mu.RUnlock()
	return &cp
}

// UpdateProfile 更新昵称/简介/头像（空字符串表示不修改对应字段；bio 允许清空需显式传值）
// UpdateProfile 更新昵称/简介/头像。bio 用指针以支持"清空简介"。
func (s *Store) UpdateProfile(id, nickname string, bio *string, avatar string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.Users[id]
	if !ok {
		return fmt.Errorf("用户不存在")
	}
	if nickname != "" {
		u.Nickname = nickname
	}
	if bio != nil {
		u.Bio = *bio
	}
	if avatar != "" {
		u.Avatar = avatar
	}
	s.saveLocked("users.json")
	return nil
}

// UserCounts 个人主页统计：笔记数 / 获赞 / 关注中 / 粉丝
func (s *Store) UserCounts(userID string) map[string]int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	notes, likes := 0, 0
	for _, n := range s.Notes {
		if n.AuthorID == userID && n.Status == "published" {
			notes++
			likes += len(s.Likes[n.ID])
		}
	}
	following := len(s.Follow[userID])
	followers := 0
	for _, list := range s.Follow {
		for _, id := range list {
			if id == userID {
				followers++
			}
		}
	}
	return map[string]int{"notes": notes, "likes": likes, "following": following, "followers": followers}
}

// IsFollowing 判断 userID 是否已关注 targetID
func (s *Store) IsFollowing(userID, targetID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, id := range s.Follow[userID] {
		if id == targetID {
			return true
		}
	}
	return false
}

// FavNotesOf 返回某用户收藏的已发布笔记（按时间倒序）
func (s *Store) FavNotesOf(userID string) []*model.Note {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []*model.Note{}
	for noteID, users := range s.Favs {
		for _, id := range users {
			if id == userID {
				if n, ok := s.Notes[noteID]; ok && n.Status == "published" {
					cp := *n // 拷贝，避免返回内部对象引发 data race
					out = append(out, &cp)
				}
				break
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out
}

// ---------- 笔记 ----------

func (s *Store) AddNote(author *model.User, title, content, mediaType string, media []string, cover string, tags []string, category, status, level string) *model.Note {
	n := &model.Note{
		ID: auth.ID("n_"), AuthorID: author.ID, AuthorName: author.Nickname,
		Title: title, Content: content, MediaType: mediaType, Media: media,
		Cover: cover, Tags: tags, Category: category,
		Status: status, Level: level, CreatedAt: time.Now().Unix(),
	}
	s.mu.Lock()
	s.Notes[n.ID] = n
	s.saveLocked("notes.json")
	cp := *n // 持锁期间拷贝，避免返回内部对象引发 data race
	s.mu.Unlock()
	return &cp
}

// ReviewNote 审核定级：status 传 "published"（通过）或 "rejected"（驳回）；通过时必须给 level
func (s *Store) ReviewNote(id, status, level string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, ok := s.Notes[id]
	if !ok {
		return fmt.Errorf("笔记不存在")
	}
	if status == "published" {
		if level != "green" && level != "yellow" && level != "red" && level != "black" {
			return fmt.Errorf("等级必须为 green / yellow / red / black")
		}
		n.Status = "published"
		n.Level = level
	} else if status == "rejected" {
		n.Status = "rejected"
	} else {
		return fmt.Errorf("无效的审核状态")
	}
	s.saveLocked("notes.json")
	return nil
}

func (s *Store) RemoveNote(id, userID string, isAdmin bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, ok := s.Notes[id]
	if !ok {
		return fmt.Errorf("笔记不存在")
	}
	if !isAdmin && n.AuthorID != userID {
		return fmt.Errorf("无权限")
	}
	n.Status = "removed"
	n.RemovedAt = time.Now().Unix() // 记录下架时间，宽限期计时
	delete(s.Likes, id)             // 清理孤儿点赞记录
	delete(s.Favs, id)              // 清理孤儿收藏记录
	s.clearNotifsOf(id, "")         // 清理引用该笔记的通知
	s.saveLocked("notes.json", "likes.json", "favorites.json", "notifications.json")
	return nil
}

// purgeNoteLocked 物理删除一条笔记及其全部关联数据（likes / favs / comments / clikes / notifs）
// 调用方需持写锁。返回被删笔记的作者 ID 与媒体列表，便于 handler 清理磁盘文件。
func (s *Store) purgeNoteLocked(id string) (string, []string) {
	n, ok := s.Notes[id]
	if !ok {
		return "", nil
	}
	media := append([]string{}, n.Media...)
	authorID := n.AuthorID
	delete(s.Notes, id)
	delete(s.Likes, id)
	delete(s.Favs, id)
	// 清理该笔记下的所有评论及其点赞
	for cid, c := range s.Commen {
		if c.NoteID == id {
			delete(s.Commen, cid)
			delete(s.CLikes, cid)
		}
	}
	// 清理引用该笔记的通知
	s.clearNotifsOf(id, "")
	return authorID, media
}

// PurgeNote 永久删除一条笔记（任何状态；管理员专用）
func (s *Store) PurgeNote(id string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.Notes[id]; !ok {
		return nil, fmt.Errorf("笔记不存在")
	}
	_, media := s.purgeNoteLocked(id)
	s.saveLocked("notes.json", "likes.json", "favorites.json", "comments.json", "comment_likes.json", "notifications.json")
	return media, nil
}

// PurgeOldRemovedNotes 永久清理已下架超过 maxAge 的笔记（含其 likes/favs/comments/notifs）。
// 返回被清理的笔记数，以及「作者 ID -> 该作者待删媒体路径」的映射，供 handler 删除磁盘文件
// （下架时为保留可恢复能力不删文件，因此这里是媒体文件唯一的清理时机，漏掉就是磁盘泄漏）。
// 已下架但 RemovedAt=0 的视为历史数据（兜底清理）。
func (s *Store) PurgeOldRemovedNotes(maxAge time.Duration) (int, map[string][]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cutoff := time.Now().Add(-maxAge).Unix()
	// 先收集待清理的 id 再删除，避免 map 迭代中修改导致的计数偏差
	ids := []string{}
	for id, n := range s.Notes {
		if n.Status != "removed" {
			continue
		}
		// 历史数据 RemovedAt=0 → 视为待清理；否则下架距今 >= maxAge 即到期
		if n.RemovedAt == 0 || n.RemovedAt <= cutoff {
			ids = append(ids, id)
		}
	}
	mediaByAuthor := map[string][]string{}
	for _, id := range ids {
		authorID, media := s.purgeNoteLocked(id)
		if len(media) > 0 {
			mediaByAuthor[authorID] = append(mediaByAuthor[authorID], media...)
		}
	}
	if len(ids) > 0 {
		s.saveLocked("notes.json", "likes.json", "favorites.json", "comments.json", "comment_likes.json", "notifications.json")
	}
	return len(ids), mediaByAuthor, nil
}

// NotesByAuthor 某作者的已发布笔记（置顶优先，再按时间倒序）
func (s *Store) NotesByAuthor(authorID string) []*model.Note {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []*model.Note{}
	for _, n := range s.Notes {
		if n.AuthorID == authorID && n.Status == "published" {
			if author, ok := s.Users[authorID]; ok && author.Status != "active" {
				continue // 被禁用户的笔记不再展示（临时封锁不影响内容展示）
			}
			cp := *n
			out = append(out, &cp)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Pinned != out[j].Pinned {
			return out[i].Pinned
		}
		return out[i].CreatedAt > out[j].CreatedAt
	})
	return out
}

// NotesByAuthorAll 某作者的全部笔记（含 pending/rejected，仅供作者本人查看）
func (s *Store) NotesByAuthorAll(authorID string) []*model.Note {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []*model.Note{}
	for _, n := range s.Notes {
		if n.AuthorID == authorID {
			cp := *n
			out = append(out, &cp)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out
}

// SetPin 置顶/取消置顶（仅作者本人）
func (s *Store) SetPin(id, userID string, pinned bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, ok := s.Notes[id]
	if !ok {
		return fmt.Errorf("笔记不存在")
	}
	if n.AuthorID != userID {
		return fmt.Errorf("无权限")
	}
	n.Pinned = pinned
	s.saveLocked("notes.json")
	return nil
}

// SetUserPassword 重置用户密码（管理后台）。newPlain 为明文副本，一并记录以便管理员查看。
func (s *Store) SetUserPassword(id, newHash, newPlain string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.Users[id]
	if !ok {
		return fmt.Errorf("用户不存在")
	}
	u.PasswordHash = newHash
	u.PasswordPlain = newPlain
	s.saveLocked("users.json")
	return nil
}

// ChangePassword 用户自助修改密码：校验旧密码正确后写入新密码哈希与明文副本。
// 调用方（handler）负责在成功后使该用户旧会话失效（DeleteUserSessions）。
func (s *Store) ChangePassword(userID, oldPw, newHash, newPlain string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.Users[userID]
	if !ok {
		return fmt.Errorf("用户不存在")
	}
	if !auth.CheckPassword(oldPw, u.PasswordHash) {
		return fmt.Errorf("旧密码不正确")
	}
	u.PasswordHash = newHash
	u.PasswordPlain = newPlain
	s.saveLocked("users.json")
	return nil
}

// NotesList 返回已发布笔记（可选 category / 关键词 / 仅关注 / 排序 order=hot|new）
func (s *Store) NotesList(category, q string, following map[string]bool, order string) []*model.Note {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []*model.Note{}
	for _, n := range s.Notes {
		if n.Status != "published" {
			continue
		}
		if author, ok := s.Users[n.AuthorID]; ok && author.Status != "active" {
			continue // 被禁用户的笔记不再展示（临时封锁不影响内容展示）
		}
		if q == "" && n.Level == "black" {
			continue // 黑标：仅搜索可见，浏览/分类/首页不展示
		}
		if category != "" && category != "推荐" && n.Category != category {
			continue
		}
		if following != nil && !following[n.AuthorID] {
			continue
		}
		if q != "" {
			hit := strings.Contains(n.Title, q) || strings.Contains(n.AuthorName, q)
			if !hit {
				for _, t := range n.Tags {
					if strings.Contains(t, q) {
						hit = true
						break
					}
				}
			}
			if !hit {
				continue
			}
		}
		cp := *n
		out = append(out, &cp)
	}
	if order == "hot" {
		sort.Slice(out, func(i, j int) bool {
			si, sj := s.noteScore(out[i]), s.noteScore(out[j])
			if si != sj {
				return si > sj
			}
			return out[i].CreatedAt > out[j].CreatedAt
		})
	} else {
		sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	}
	return out
}

// noteScore 热度分：点赞 + 评论×2（调用方需持有读锁）
func (s *Store) noteScore(n *model.Note) int {
	cc := 0
	for _, c := range s.Commen {
		if c.NoteID == n.ID {
			cc++
		}
	}
	return len(s.Likes[n.ID]) + cc*2
}

// NoteRaw 按 ID 取笔记（含已下架，供管理/删除清理用）
func (s *Store) NoteRaw(id string) *model.Note {
	s.mu.RLock()
	n, ok := s.Notes[id]
	if !ok {
		s.mu.RUnlock()
		return nil
	}
	cp := *n // 持锁期间完成拷贝，避免与写锁内的修改构成 data race
	s.mu.RUnlock()
	return &cp
}

// AllNotes 全部笔记（含 pending/rejected/removed，管理后台用），时间倒序
func (s *Store) AllNotes() []*model.Note {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []*model.Note{}
	for _, n := range s.Notes {
		cp := *n
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out
}

func (s *Store) NoteByID(id string) *model.Note {
	s.mu.RLock()
	n, ok := s.Notes[id]
	if !ok || n.Status != "published" {
		s.mu.RUnlock()
		return nil
	}
	cp := *n // 持锁期间完成拷贝，避免与写锁内的修改构成 data race
	s.mu.RUnlock()
	return &cp
}

// ---------- 互动 ----------

func (s *Store) ToggleLike(noteID, userID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	list := s.Likes[noteID]
	for i, id := range list {
		if id == userID {
			s.Likes[noteID] = append(list[:i], list[i+1:]...)
			s.saveLocked("likes.json")
			return false
		}
	}
	s.Likes[noteID] = append(list, userID)
	s.saveLocked("likes.json")
	return true
}

func (s *Store) ToggleFav(noteID, userID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	list := s.Favs[noteID]
	for i, id := range list {
		if id == userID {
			s.Favs[noteID] = append(list[:i], list[i+1:]...)
			s.saveLocked("favorites.json")
			return false
		}
	}
	s.Favs[noteID] = append(list, userID)
	s.saveLocked("favorites.json")
	return true
}

// LikeCount 笔记点赞总数（实时内存计数）
func (s *Store) LikeCount(id string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.Likes[id])
}

// FavCount 笔记收藏总数（实时内存计数）
func (s *Store) FavCount(id string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.Favs[id])
}

func (s *Store) AddComment(noteID, userID, userName, content, parentID, replyTo string) *model.Comment {
	c := &model.Comment{
		ID: auth.ID("c_"), NoteID: noteID, AuthorID: userID,
		AuthorName: userName, Content: content,
		ParentID: parentID, ReplyTo: replyTo, CreatedAt: time.Now().Unix(),
	}
	s.mu.Lock()
	s.Commen[c.ID] = c
	s.saveLocked("comments.json")
	cp := *c // 持锁期间拷贝，避免返回内部对象引发 data race
	s.mu.Unlock()
	return &cp
}

func (s *Store) CommentByID(id string) *model.Comment {
	s.mu.RLock()
	c, ok := s.Commen[id]
	if !ok {
		s.mu.RUnlock()
		return nil
	}
	cp := *c // 持锁期间完成拷贝，避免与写锁内的修改构成 data race
	s.mu.RUnlock()
	return &cp
}

// CommentsByNote 返回某笔记的评论（楼中楼结构）。
// order: "hot" 顶层按回复数降序，其余按时间正序；子评论固定跟在父评论后。
func (s *Store) CommentsByNote(noteID, order string) []*model.Comment {
	s.mu.RLock()
	defer s.mu.RUnlock()
	children := map[string][]*model.Comment{}
	tops := []*model.Comment{}
	for _, c := range s.Commen {
		if c.NoteID != noteID {
			continue
		}
		cp := *c
		if c.ParentID == "" {
			tops = append(tops, &cp)
		} else {
			children[c.ParentID] = append(children[c.ParentID], &cp)
		}
	}
	for _, sc := range children {
		sort.Slice(sc, func(i, j int) bool { return sc[i].CreatedAt < sc[j].CreatedAt })
	}
	if order == "hot" {
		sort.Slice(tops, func(i, j int) bool {
			ri, rj := len(children[tops[i].ID]), len(children[tops[j].ID])
			if ri != rj {
				return ri > rj
			}
			return tops[i].CreatedAt < tops[j].CreatedAt
		})
	} else {
		sort.Slice(tops, func(i, j int) bool { return tops[i].CreatedAt < tops[j].CreatedAt })
	}
	out := []*model.Comment{}
	for _, t := range tops {
		out = append(out, t)
		out = append(out, children[t.ID]...)
	}
	return out
}

// clearNotifsOf 清理引用指定笔记或评论的通知（调用方需持写锁）
func (s *Store) clearNotifsOf(noteID, commentID string) {
	for id, n := range s.Notifs {
		if (noteID != "" && n.NoteID == noteID) || (commentID != "" && n.CommentID == commentID) {
			delete(s.Notifs, id)
		}
	}
}

// deleteCommentLocked 删除一条评论并级联删除其楼中楼子回复（调用方需持写锁）。
// 若不清理子评论，父评论被删后子回复会变成"孤儿"：既不展示、又残留计数与通知。
func (s *Store) deleteCommentLocked(id string) {
	// 先递归删除子评论
	children := []string{}
	for cid, c := range s.Commen {
		if c.ParentID == id {
			children = append(children, cid)
		}
	}
	for _, cid := range children {
		s.deleteCommentLocked(cid)
	}
	delete(s.Commen, id)
	delete(s.CLikes, id) // 清理该评论的点赞记录
	s.clearNotifsOf("", id)
}

func (s *Store) DeleteComment(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.Commen[id]; !ok {
		return fmt.Errorf("评论不存在")
	}
	s.deleteCommentLocked(id)
	s.saveLocked("comments.json", "comment_likes.json")
	return nil
}

// DeleteCommentFor 删除评论：作者本人或管理员（级联删除子回复）
func (s *Store) DeleteCommentFor(id, userID string, isAdmin bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.Commen[id]
	if !ok {
		return fmt.Errorf("评论不存在")
	}
	if !isAdmin && c.AuthorID != userID {
		return fmt.Errorf("无权限删除该评论")
	}
	s.deleteCommentLocked(id)
	s.saveLocked("comments.json", "comment_likes.json")
	return nil
}

// IncViews 笔记浏览数 +n（n 通常为 1）。
// 节流落盘：累计 20 次变更或距上次落盘超过 30 秒才写一次盘，避免高并发浏览触发频繁全量 JSON 写。
func (s *Store) IncViews(id string, n int) {
	if n <= 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	note, ok := s.Notes[id]
	if !ok {
		return
	}
	note.Views += n
	s.viewsDirty++
	if s.viewsDirty >= 20 || time.Since(s.viewsLastSave) >= 30*time.Second {
		s.saveLocked("notes.json")
		s.viewsDirty = 0
		s.viewsLastSave = time.Now()
	}
}

// viewDedupWindow 同一用户对同一笔记的浏览去重窗口（默认 30 分钟）。
// 防止首页瀑布流卡片被反复看到时浏览量疯涨；同时也让「浏览记录」语义合理——
// 30 分钟内的重复曝光算作 1 次「看过」，而不是 10 次。
const viewDedupWindow = 30 * time.Minute

// IncViewsDedup 带用户维度的浏览去重：同一用户对同一笔记 30 分钟内只 +1 次浏览。
// 返回 true 表示本次实际累计了浏览量（用于给上层决定是否写浏览历史等）。
// 进程退出时不持久化去重状态——重启后重新计数无副作用。
func (s *Store) IncViewsDedup(noteID, userID string, n int) bool {
	if userID == "" || noteID == "" || n <= 0 {
		return false
	}
	key := userID + "|" + noteID
	now := time.Now().Unix()
	s.viewDedupMu.Lock()
	last, ok := s.viewDedup[key]
	if ok && now-last < int64(viewDedupWindow.Seconds()) {
		s.viewDedupMu.Unlock()
		return false
	}
	s.viewDedup[key] = now
	// 顺手清理过期条目，防止长跑后 map 膨胀
	if len(s.viewDedup) > 4096 {
		for k, t := range s.viewDedup {
			if now-t >= int64(viewDedupWindow.Seconds()) {
				delete(s.viewDedup, k)
			}
		}
	}
	s.viewDedupMu.Unlock()
	s.IncViews(noteID, n)
	return true
}

// ---------- 浏览历史 ----------

// maxViewHistory 每个用户浏览历史上限（条），超出自动淘汰最旧的。
const maxViewHistory = 200

// HistoryEntry 一条浏览历史记录（noteID + 最近浏览时间）
type HistoryEntry struct {
	NoteID   string
	ViewedAt int64
}

// RecordView 记录一次浏览历史：同一篇笔记只保留一条，更新时间戳（列表自动按时间倒序）。
// 仅登录用户调用（匿名访问无"我的历史"概念）。
func (s *Store) RecordView(userID, noteID string) {
	if userID == "" || noteID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	m := s.ViewHist[userID]
	if m == nil {
		m = map[string]int64{}
		s.ViewHist[userID] = m
	}
	// 时间戳全局单调递增：新浏览始终严格大于现有全部记录与墙钟，
	// 同一秒内先后浏览的不同笔记也能保持「最近浏览」顺序稳定可区分。
	// （真实场景每秒浏览多篇概率极低；即便轻微超前墙钟，前端也按「刚刚」展示，无碍）
	now := time.Now().Unix()
	for _, ts := range m {
		if ts >= now {
			now = ts + 1
		}
	}
	m[noteID] = now
	// 上限：超出时淘汰最旧一条（时间戳相同则按笔记 ID 字典序，保证确定性）
	if len(m) > maxViewHistory {
		oldestID := ""
		for id, ts := range m {
			if oldestID == "" || ts < m[oldestID] || (ts == m[oldestID] && id < oldestID) {
				oldestID = id
			}
		}
		if oldestID != "" {
			delete(m, oldestID)
		}
	}
	s.saveLocked("view_history.json")
}

// ViewHistory 返回某用户的浏览历史（按最近浏览时间倒序）
func (s *Store) ViewHistory(userID string) []HistoryEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m := s.ViewHist[userID]
	out := make([]HistoryEntry, 0, len(m))
	for id, ts := range m {
		out = append(out, HistoryEntry{NoteID: id, ViewedAt: ts})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ViewedAt > out[j].ViewedAt })
	return out
}

// DeleteHistoryItem 删除某用户历史中的一条
func (s *Store) DeleteHistoryItem(userID, noteID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if m, ok := s.ViewHist[userID]; ok {
		if _, exists := m[noteID]; exists {
			delete(m, noteID)
			s.saveLocked("view_history.json")
		}
	}
}

// ClearHistory 清空某用户的全部浏览历史
func (s *Store) ClearHistory(userID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.ViewHist[userID]) > 0 {
		delete(s.ViewHist, userID)
		s.saveLocked("view_history.json")
	}
}

// ---------- 负反馈（不感兴趣） ----------

// AddDislike 记录用户对某笔记的「不感兴趣」。幂等：重复调用不报错。
func (s *Store) AddDislike(userID, noteID string) {
	if userID == "" || noteID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Dislikes[userID] == nil {
		s.Dislikes[userID] = map[string]bool{}
	}
	s.Dislikes[userID][noteID] = true
	s.saveLocked("dislikes.json")
}

// RemoveDislike 撤销用户对某笔记的「不感兴趣」（前端「撤销」操作时调用），
// 使该笔记与同类标签恢复推荐。
func (s *Store) RemoveDislike(userID, noteID string) {
	if userID == "" || noteID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Dislikes[userID] != nil {
		delete(s.Dislikes[userID], noteID)
		if len(s.Dislikes[userID]) == 0 {
			delete(s.Dislikes, userID)
		}
	}
	s.saveLocked("dislikes.json")
}

// IsDisliked 该用户是否对某笔记点过「不感兴趣」（调用方需酌情持锁；这里读不持锁，
// 仅在单线程推荐流程内调用，且 Dislikes 写操作带锁 + map 不扩容读取安全）。
func (s *Store) IsDisliked(userID, noteID string) bool {
	m := s.Dislikes[userID]
	if m == nil {
		return false
	}
	return m[noteID]
}

// DislikedTags 该用户所有「不感兴趣」笔记的标签集合（去重）。用于推荐时同类标签降权。
// 调用方需持读锁（内部遍历 s.Notes / s.Dislikes）。
func (s *Store) DislikedTags(userID string) map[string]bool {
	out := map[string]bool{}
	m := s.Dislikes[userID]
	if len(m) == 0 {
		return out
	}
	for noteID := range m {
		if n, ok := s.Notes[noteID]; ok {
			for _, t := range n.Tags {
				out[t] = true
			}
		}
	}
	return out
}

// ToggleCommentLike 评论点赞/取消赞，返回当前是否已点赞
func (s *Store) ToggleCommentLike(commentID, userID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.Commen[commentID]
	if !ok {
		return false
	}
	list := s.CLikes[commentID]
	for i, id := range list {
		if id == userID {
			s.CLikes[commentID] = append(list[:i], list[i+1:]...)
			if c.LikeCount > 0 {
				c.LikeCount--
			}
			s.saveLocked("comments.json", "comment_likes.json")
			return false
		}
	}
	s.CLikes[commentID] = append(list, userID)
	c.LikeCount++
	s.saveLocked("comments.json", "comment_likes.json")
	return true
}

// CommentLikedBy 该用户是否已点赞该评论
func (s *Store) CommentLikedBy(commentID, userID string) bool {
	if userID == "" {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, id := range s.CLikes[commentID] {
		if id == userID {
			return true
		}
	}
	return false
}

// FillCommentViews 给评论填充"当前浏览者是否已赞"（不落盘）
func (s *Store) FillCommentViews(comments []*model.Comment, viewerID string) {
	if viewerID == "" {
		return
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, c := range comments {
		for _, id := range s.CLikes[c.ID] {
			if id == viewerID {
				c.Liked = true
				break
			}
		}
	}
}

func (s *Store) ToggleFollow(userID, targetID string) bool {
	if userID == targetID {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	list := s.Follow[userID]
	for i, id := range list {
		if id == targetID {
			s.Follow[userID] = append(list[:i], list[i+1:]...)
			s.saveLocked("follows.json")
			return false
		}
	}
	s.Follow[userID] = append(list, targetID)
	s.saveLocked("follows.json")
	return true
}

// FollowingSet 返回某用户关注的对象集合，供"关注"流过滤
func (s *Store) FollowingSet(userID string) map[string]bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	set := map[string]bool{}
	for _, id := range s.Follow[userID] {
		set[id] = true
	}
	return set
}

// ---------- 好友 ----------

// SendFriendRequest 按用户号发送好友请求
func (s *Store) SendFriendRequest(fromID, toUid string) (*model.FriendRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var to *model.User
	for _, u := range s.Users {
		if u.Uid == toUid {
			to = u
			break
		}
	}
	if to == nil {
		return nil, fmt.Errorf("没有找到这个用户号，请确认后重试")
	}
	if to.ID == fromID {
		return nil, fmt.Errorf("不能添加自己为好友")
	}
	if s.isFriendLocked(fromID, to.ID) {
		return nil, fmt.Errorf("你们已经是好友了")
	}
	// 存在任一 pending 请求则拒绝重复发送
	for _, r := range s.Reqs {
		if r.Status == "pending" && ((r.FromID == fromID && r.ToID == to.ID) || (r.FromID == to.ID && r.ToID == fromID)) {
			return nil, fmt.Errorf("已存在待处理的好友请求")
		}
	}
	from := s.Users[fromID]
	if from == nil {
		return nil, fmt.Errorf("用户不存在")
	}
	req := &model.FriendRequest{
		ID: auth.ID("fr_"), FromID: fromID, FromName: from.Nickname,
		ToID: to.ID, Status: "pending", CreatedAt: time.Now().Unix(),
	}
	s.Reqs[req.ID] = req
	s.saveLocked("friend_requests.json")
	return req, nil
}

func (s *Store) isFriendLocked(a, b string) bool {
	for _, id := range s.Friends[a] {
		if id == b {
			return true
		}
	}
	return false
}

// FriendRequestsTo 我收到的好友请求（pending）
func (s *Store) FriendRequestsTo(userID string) []*model.FriendRequest {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []*model.FriendRequest{}
	for _, r := range s.Reqs {
		if r.ToID == userID && r.Status == "pending" {
			cp := *r // 拷贝，避免返回内部对象引发 data race
			out = append(out, &cp)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out
}

// FriendRequestsFrom 我发出的好友请求（pending）
func (s *Store) FriendRequestsFrom(userID string) []*model.FriendRequest {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []*model.FriendRequest{}
	for _, r := range s.Reqs {
		if r.FromID == userID && r.Status == "pending" {
			cp := *r // 拷贝，避免返回内部对象引发 data race
			out = append(out, &cp)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out
}

// ReqByID 按 ID 取好友请求（供 accept 后通知使用）
func (s *Store) ReqByID(id string) *model.FriendRequest {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.Reqs[id]
	if !ok {
		return nil
	}
	cp := *r // 拷贝，避免返回内部对象引发 data race
	return &cp
}

// AcceptFriendRequest 接受好友请求（仅接收方可）
func (s *Store) AcceptFriendRequest(id, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.Reqs[id]
	if !ok {
		return fmt.Errorf("请求不存在")
	}
	if r.ToID != userID {
		return fmt.Errorf("无权限")
	}
	if r.Status != "pending" {
		return fmt.Errorf("请求已处理")
	}
	r.Status = "accepted"
	s.Friends[r.FromID] = append(s.Friends[r.FromID], r.ToID)
	s.Friends[r.ToID] = append(s.Friends[r.ToID], r.FromID)
	s.saveLocked("friends.json", "friend_requests.json")
	return nil
}

// RejectFriendRequest 拒绝好友请求
func (s *Store) RejectFriendRequest(id, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.Reqs[id]
	if !ok {
		return fmt.Errorf("请求不存在")
	}
	if r.ToID != userID {
		return fmt.Errorf("无权限")
	}
	r.Status = "rejected"
	s.saveLocked("friend_requests.json")
	return nil
}

// FriendsOf 我的好友列表
func (s *Store) FriendsOf(userID string) []*model.User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []*model.User{}
	for _, id := range s.Friends[userID] {
		if u := s.Users[id]; u != nil {
			cp := *u
			out = append(out, &cp)
		}
	}
	return out
}

// RemoveFriend 删除好友（双向移除）
func (s *Store) RemoveFriend(userID, friendID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.isFriendLocked(userID, friendID) {
		return fmt.Errorf("你们还不是好友")
	}
	rm := func(list []string, target string) []string {
		out := list[:0]
		for _, id := range list {
			if id != target {
				out = append(out, id)
			}
		}
		return out
	}
	s.Friends[userID] = rm(s.Friends[userID], friendID)
	s.Friends[friendID] = rm(s.Friends[friendID], userID)
	s.saveLocked("friends.json")
	return nil
}

// Relation 两人关系：none / friend / requestSent / requestReceived
func (s *Store) Relation(meID, otherID string) string {
	if meID == otherID {
		return "self"
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.isFriendLocked(meID, otherID) {
		return "friend"
	}
	for _, r := range s.Reqs {
		if r.Status != "pending" {
			continue
		}
		if r.FromID == meID && r.ToID == otherID {
			return "requestSent"
		}
		if r.FromID == otherID && r.ToID == meID {
			return "requestReceived"
		}
	}
	return "none"
}

// ---------- 私信 ----------

// AddMessage 发送一条站内私信
func (s *Store) AddMessage(fromID, fromName, toID, content, mediaType, media, mediaName string, mediaSize int64) *model.Message {
	m := &model.Message{
		ID: auth.ID("m_"), FromID: fromID, FromName: fromName, ToID: toID,
		Content: content, MediaType: mediaType, Media: media,
		MediaName: mediaName, MediaSize: mediaSize, CreatedAt: time.Now().Unix(),
	}
	s.mu.Lock()
	s.Msgs[m.ID] = m
	s.saveLocked("messages.json")
	cp := *m // 持锁期间拷贝，避免返回内部对象引发 data race
	s.mu.Unlock()
	return &cp
}

// MessagesBetween 两人之间的全部消息（时间正序）
func (s *Store) MessagesBetween(a, b string) []*model.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []*model.Message{}
	for _, m := range s.Msgs {
		if (m.FromID == a && m.ToID == b) || (m.FromID == b && m.ToID == a) {
			cp := *m
			out = append(out, &cp)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt < out[j].CreatedAt })
	return out
}

// ConversationsOf 某用户的所有会话（按最后消息时间倒序）
func (s *Store) ConversationsOf(userID string) []model.Conversation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// peerID -> 最后消息
	last := map[string]*model.Message{}
	unread := map[string]int{}
	for _, m := range s.Msgs {
		var peer string
		if m.FromID == userID {
			peer = m.ToID
		} else if m.ToID == userID {
			peer = m.FromID
			if !m.Read {
				unread[peer]++
			}
		} else {
			continue
		}
		if cur, ok := last[peer]; !ok || m.CreatedAt > cur.CreatedAt {
			last[peer] = m
		}
	}
	out := []model.Conversation{}
	for peer, lm := range last {
		u := s.Users[peer]
		if u == nil {
			continue
		}
		lmCp := *lm
		out = append(out, model.Conversation{Peer: model.ToPublic(u), LastMsg: &lmCp, Unread: unread[peer]})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LastMsg.CreatedAt > out[j].LastMsg.CreatedAt })
	return out
}

// UnreadMessagesCount 某用户所有未读私信总数
func (s *Store) UnreadMessagesCount(userID string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n := 0
	for _, m := range s.Msgs {
		if m.ToID == userID && !m.Read {
			n++
		}
	}
	return n
}

// MarkMessagesRead 把某用户发给我的所有未读私信标记为已读
func (s *Store) MarkMessagesRead(userID, peerID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	changed := false
	for _, m := range s.Msgs {
		if m.FromID == peerID && m.ToID == userID && !m.Read {
			m.Read = true
			changed = true
		}
	}
	if changed {
		s.saveLocked("messages.json")
	}
}

// ---------- 通知 ----------

// AddNotification 写入通知（自己触发自己不通知）
func (s *Store) AddNotification(n *model.Notification) {
	if n.UserID == "" || n.ActorID == n.UserID {
		return
	}
	s.mu.Lock()
	s.Notifs[n.ID] = n
	s.saveLocked("notifications.json")
	s.mu.Unlock()
}

func (s *Store) NotificationsOf(userID string) []*model.Notification {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []*model.Notification{}
	for _, n := range s.Notifs {
		if n.UserID == userID {
			cp := *n // 拷贝，避免返回内部对象引发 data race
			out = append(out, &cp)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out
}

func (s *Store) UnreadCount(userID string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c := 0
	for _, n := range s.Notifs {
		if n.UserID == userID && !n.Read {
			c++
		}
	}
	return c
}

func (s *Store) MarkAllRead(userID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	changed := false
	for _, n := range s.Notifs {
		if n.UserID == userID && !n.Read {
			n.Read = true
			changed = true
		}
	}
	if changed {
		s.saveLocked("notifications.json")
	}
}

func (s *Store) MarkRead(id, userID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n, ok := s.Notifs[id]; ok && n.UserID == userID && !n.Read {
		n.Read = true
		s.saveLocked("notifications.json")
	}
}

// ---------- 举报 ----------

// AddReport 提交举报（同一目标同一举报人重复举报不重复记录）
func (s *Store) AddReport(target, targetID, reporterID, reporter, reason, title string) (*model.Report, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.Reports {
		if r.Target == target && r.TargetID == targetID && r.ReporterID == reporterID && r.Status == "pending" {
			return nil, fmt.Errorf("你已举报过该内容，等待管理员处理")
		}
	}
	rp := &model.Report{
		ID: auth.ID("rp_"), Target: target, TargetID: targetID,
		ReporterID: reporterID, Reporter: reporter, Reason: reason, Title: title,
		Status: "pending", CreatedAt: time.Now().Unix(),
	}
	s.Reports[rp.ID] = rp
	s.saveLocked("reports.json")
	return rp, nil
}

// SetReportStatus 处理举报：resolved / ignored
func (s *Store) SetReportStatus(id, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rp, ok := s.Reports[id]
	if !ok {
		return fmt.Errorf("举报不存在")
	}
	rp.Status = status
	s.saveLocked("reports.json")
	return nil
}

// AllReports 全部举报（时间倒序）
func (s *Store) AllReports() []*model.Report {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []*model.Report{}
	for _, rp := range s.Reports {
		cp := *rp // 拷贝，避免返回内部对象引发 data race
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out
}

// ---------- 草稿 ----------

// SaveDraft 保存草稿：id 为空则新建，否则必须是自己名下的草稿（防越权覆盖）
func (s *Store) SaveDraft(d *model.Draft) (*model.Draft, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if d.ID == "" {
		d.ID = auth.ID("d_")
	} else if old, ok := s.Drafts[d.ID]; ok && old.UserID != d.UserID {
		return nil, fmt.Errorf("无权限修改该草稿")
	}
	d.UpdatedAt = time.Now().Unix()
	s.Drafts[d.ID] = d
	s.saveLocked("drafts.json")
	return d, nil
}

func (s *Store) DraftsOf(userID string) []*model.Draft {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []*model.Draft{}
	for _, d := range s.Drafts {
		if d.UserID == userID {
			out = append(out, d)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt > out[j].UpdatedAt })
	return out
}

func (s *Store) DeleteDraft(id, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.Drafts[id]
	if !ok {
		return fmt.Errorf("草稿不存在")
	}
	if d.UserID != userID {
		return fmt.Errorf("无权限")
	}
	delete(s.Drafts, id)
	s.saveLocked("drafts.json")
	return nil
}

// ---------- 关注 / 粉丝 / 标签 / 相关推荐 ----------

// FollowingUsers 我关注的人
func (s *Store) FollowingUsers(userID string) []*model.User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []*model.User{}
	for _, id := range s.Follow[userID] {
		if u := s.Users[id]; u != nil {
			cp := *u
			out = append(out, &cp)
		}
	}
	return out
}

// FollowersOf 我的粉丝
func (s *Store) FollowersOf(userID string) []*model.User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []*model.User{}
	for uid, list := range s.Follow {
		for _, id := range list {
			if id == userID {
				if u := s.Users[uid]; u != nil {
					cp := *u
					out = append(out, &cp)
				}
				break
			}
		}
	}
	return out
}

// NotesByTag 某标签下的已发布笔记（倒序）
func (s *Store) NotesByTag(tag string) []*model.Note {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []*model.Note{}
	for _, n := range s.Notes {
		if n.Status != "published" {
			continue
		}
		if author, ok := s.Users[n.AuthorID]; ok && author.Status != "active" {
			continue // 被禁用户的笔记不再展示（临时封锁不影响内容展示）
		}
		for _, t := range n.Tags {
			if t == tag {
				cp := *n
				out = append(out, &cp)
				break
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out
}

// ----- 智能推荐引擎 -----
// 设计参考小红书: 标签匹配 + CES 互动分(点赞×1+收藏×1+评论×4) + 兴趣画像 + 多样性防茧房。

// recoScored 带分的候选笔记（推荐/相关通用）
type recoScored struct {
	n     *model.Note
	score float64
}

// tagOverlap 两个标签集合的交集数量
func tagOverlap(a, b []string) int {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	set := make(map[string]bool, len(a))
	for _, t := range a {
		set[t] = true
	}
	n := 0
	for _, t := range b {
		if set[t] {
			n++
		}
	}
	return n
}

// recencyScore 时间衰减分: 越新越高, 约 30 天半衰期, 范围 (0,1]
func recencyScore(createdAt int64) float64 {
	ageDays := float64(time.Now().Unix()-createdAt) / 86400
	if ageDays < 0 {
		ageDays = 0
	}
	return 1 / (1 + ageDays/30)
}

// commentCountLocked 统计某笔记评论数（调用方需持读锁）
func (s *Store) commentCountLocked(noteID string) int {
	n := 0
	for _, c := range s.Commen {
		if c.NoteID == noteID {
			n++
		}
	}
	return n
}

// cesScoreLocked 笔记 CES 互动分（点赞×1 + 收藏×1 + 评论×4）
func (s *Store) cesScoreLocked(n *model.Note) float64 {
	return float64(len(s.Likes[n.ID]) + len(s.Favs[n.ID]) + 4*s.commentCountLocked(n.ID))
}

// interestTags 根据用户互动行为构建兴趣标签画像（标签 -> 权重）。调用方需持读锁。
func (s *Store) interestTags(userID string) map[string]float64 {
	prof := map[string]float64{}
	add := func(tags []string, w float64) {
		for _, t := range tags {
			prof[t] += w
		}
	}
	// 点赞过的笔记标签（基础兴趣信号）
	for noteID, users := range s.Likes {
		for _, u := range users {
			if u == userID {
				if n, ok := s.Notes[noteID]; ok {
					add(n.Tags, 3)
				}
				break
			}
		}
	}
	// 收藏过的笔记标签（"想留着看"，强信号）
	for noteID, users := range s.Favs {
		for _, u := range users {
			if u == userID {
				if n, ok := s.Notes[noteID]; ok {
					add(n.Tags, 4)
				}
				break
			}
		}
	}
	// 评论过的笔记标签
	for _, c := range s.Commen {
		if c.AuthorID == userID {
			if n, ok := s.Notes[c.NoteID]; ok {
				add(n.Tags, 2)
			}
		}
	}
	// 浏览过的笔记标签（轻量但高频的兴趣信号；带时间衰减，越旧越弱，避免陈年浏览常驻画像）
	if viewed := s.ViewHist[userID]; len(viewed) > 0 {
		for noteID, ts := range viewed {
			if n, ok := s.Notes[noteID]; ok {
				w := recencyScore(ts) // 近30天≈1，越旧趋近0
				if w > 0.05 {
					add(n.Tags, 1*w)
				}
			}
		}
	}
	// 自己发布过的笔记标签
	for _, n := range s.Notes {
		if n.AuthorID == userID {
			add(n.Tags, 2)
		}
	}
	// 关注的作者的笔记标签（社交兴趣扩散）
	if fids := s.Follow[userID]; len(fids) > 0 {
		fs := make(map[string]bool, len(fids))
		for _, fid := range fids {
			fs[fid] = true
		}
		for _, n := range s.Notes {
			if fs[n.AuthorID] {
				add(n.Tags, 2)
			}
		}
	}
	// 冷启动弱画像：真实互动为零、但注册时选了兴趣标签，用其做轻量画像
	// （权重 0.5，低于任何真实行为，避免喧宾夺主；一旦产生真实互动即被覆盖）
	if len(prof) == 0 {
		if u, ok := s.Users[userID]; ok && len(u.Interests) > 0 {
			for _, t := range u.Interests {
				prof[t] += 0.5
			}
		}
	}
	return prof
}

// cfScore 基于收藏行为的 Item-CF 协同过滤：
// 用户「互动过」(浏览或收藏) 的笔记集合 U；凡与 U 中某篇被同一批用户收藏过的其他笔记，
// 累加共现强度作为协同分。本质是「和我口味相似（同样收藏了某篇）的人还收藏了什么」。
// 调用方需持读锁（内部遍历 s.Favs / s.ViewHist / s.Notes）。
// 返回候选笔记的协同分（0 表示无可参考的相似行为）。
func (s *Store) cfScore(userID, noteID string) float64 {
	// 1) 该用户互动过的笔记集合
	interacted := map[string]bool{}
	if v := s.ViewHist[userID]; len(v) > 0 {
		for id := range v {
			interacted[id] = true
		}
	}
	if favs, ok := s.Favs[userID]; ok {
		for _, id := range favs {
			interacted[id] = true
		}
	}
	if len(interacted) == 0 {
		return 0 // 冷启动：无行为，协同过滤无从谈起（由热度分支兜底）
	}
	// 2) 共现累加：遍历收藏关系，找「相似用户」(与 U 中某篇同被收藏) 还收藏了哪些笔记
	score := 0.0
	for aID, users := range s.Favs {
		if !interacted[aID] {
			continue // A 不是用户互动过的笔记，跳过
		}
		for bID, u2 := range s.Favs {
			if bID == aID || bID == noteID {
				continue
			}
			// 找与 A 的共同收藏者数量（共现强度）
			common := 0
			for _, u := range users {
				for _, v := range u2 {
					if u == v {
						common++
						break
					}
				}
			}
			if common > 0 {
				score += float64(common)
			}
		}
	}
	return score
}

// RecommendForUser 首页「为你推荐」: 基于兴趣画像 + CES + 新鲜度 + 社交关系的个性化排序。
// 冷启动（无互动历史）自动退化为「热度 + 新鲜度」排序, 保证非空。
func (s *Store) RecommendForUser(userID string, limit int) []*model.Note {
	s.mu.RLock()
	defer s.mu.RUnlock()
	prof := s.interestTags(userID)
	followed := map[string]bool{}
	for _, fid := range s.Follow[userID] {
		followed[fid] = true
	}
	disliked := s.Dislikes[userID]      // 已「不感兴趣」的笔记集合
	dislikedTags := s.DislikedTags(userID) // 不感兴趣笔记的标签集合（同类降权）
	cfg := s.RecoConfig
	cands := []recoScored{}
	for _, n := range s.Notes {
		if n.Status != "published" {
			continue
		}
		if author, ok := s.Users[n.AuthorID]; ok && author.Status != "active" {
			continue // 被禁用户的笔记不再展示（临时封锁不影响内容展示）
		}
		if n.Level == "black" {
			continue // 黑标不进入「为你推荐」推送
		}
		if disliked[n.ID] {
			continue // 已「不感兴趣」：直接剔除
		}
		tagScore := 0.0
		hitDislikedTag := false
		for _, t := range n.Tags {
			tagScore += prof[t]
			if dislikedTags[t] {
				hitDislikedTag = true
			}
		}
		ces := s.cesScoreLocked(n)
		cesNorm := ces / (ces + 5) // 0~1 饱和, 避免高互动笔记压制其他内容
		rec := recencyScore(n.CreatedAt)
		followBonus := 0.0
		if followed[n.AuthorID] {
			followBonus = 5 // 已关注作者: 社交关系加权
		}
		cf := s.cfScore(userID, n.ID) // 协同过滤: 相似用户也喜欢的
		score := tagScore*cfg.WTag + cesNorm*cfg.WCes + rec*cfg.WRec + followBonus*cfg.WFollow + cf*cfg.WCf
		if hitDislikedTag {
			// 命中不感兴趣标签：降权，但非硬剔除（用户仍可偶然看到同类之外的其他内容）
			score -= cfg.DislikeTagPenalty * float64(len(n.Tags))
		}
		cp := *n
		cands = append(cands, recoScored{&cp, score})
	}
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].score != cands[j].score {
			return cands[i].score > cands[j].score
		}
		return cands[i].n.CreatedAt > cands[j].n.CreatedAt
	})
	// 冷启动（无真实互动行为）时单标签上限更严，强制跨类目多样；否则用常规上限防茧房
	maxPerTag := cfg.MaxPerTag
	if len(prof) == 0 {
		maxPerTag = cfg.ColdMaxPerTag
	}
	return diversify(cands, limit, maxPerTag)
}

// diversify 多样性打散: 单个标签最多 maxPerTag 篇, 避免信息茧房; 不足时放宽补齐。
func diversify(cands []recoScored, limit, maxPerTag int) []*model.Note {
	out := []*model.Note{}
	tagCount := map[string]int{}
	for _, c := range cands {
		if len(out) >= limit {
			break
		}
		over := false
		for _, t := range c.n.Tags {
			if tagCount[t] >= maxPerTag {
				over = true
				break
			}
		}
		if over {
			continue
		}
		out = append(out, c.n)
		for _, t := range c.n.Tags {
			tagCount[t]++
		}
	}
	if len(out) < limit { // 打散导致不足, 放宽限制补齐
		used := map[string]bool{}
		for _, n := range out {
			used[n.ID] = true
		}
		for _, c := range cands {
			if len(out) >= limit {
				break
			}
			if used[c.n.ID] {
				continue
			}
			out = append(out, c.n)
			used[c.n.ID] = true
		}
	}
	return out
}

// RelatedNotes 相关推荐（智能）: 按标签重叠度 + CES + 新鲜度 + 同频道加权排序,
// 并做多样性打散, 避免清一色同标签。category 仅作同频道加权。
func (s *Store) RelatedNotes(noteID, category string, limit int) []*model.Note {
	s.mu.RLock()
	defer s.mu.RUnlock()
	src := s.Notes[noteID]
	if src == nil {
		return []*model.Note{}
	}
	// 源笔记本身不计入; 仅已发布且作者未被禁
	cands := []recoScored{}
	for _, n := range s.Notes {
		if n.ID == noteID || n.Status != "published" {
			continue
		}
		if author, ok := s.Users[n.AuthorID]; ok && author.Status != "active" {
			continue // 被禁用户的笔记不进入相关推荐（临时封锁不影响内容展示）
		}
		if n.Level == "black" {
			continue // 黑标不进入相关推荐
		}
		shared := tagOverlap(n.Tags, src.Tags)
		catBonus := 0.0
		if category != "" && n.Category == category {
			catBonus = 2
		}
		ces := s.cesScoreLocked(n)
		cesNorm := ces / (ces + 5)
		rec := recencyScore(n.CreatedAt)
		score := float64(shared)*5.0 + catBonus + cesNorm*3.0 + rec*2.0
		cp := *n
		cands = append(cands, recoScored{&cp, score})
	}
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].score != cands[j].score {
			return cands[i].score > cands[j].score
		}
		return cands[i].n.CreatedAt > cands[j].n.CreatedAt
	})
	return diversify(cands, limit, 2)
}

// UpdateNote 作者编辑笔记（媒体保持不变）
// UpdateNote 作者编辑笔记。编辑后强制重新进入审核流程：
//   - status 重置为 pending（已发布笔记也会下架待审）
//   - level 清空（需管理员重新审核定级）
func (s *Store) UpdateNote(id, userID string, title, content string, tags []string, category, cover string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, ok := s.Notes[id]
	if !ok {
		return fmt.Errorf("笔记不存在")
	}
	if n.AuthorID != userID {
		return fmt.Errorf("无权限")
	}
	if title != "" {
		n.Title = title
	}
	n.Content = content
	n.Tags = tags
	if category != "" {
		n.Category = category
	}
	if cover != "" {
		n.Cover = cover
	}
	// 编辑后重新进入待审核（重审），管理员需重新定级
	n.Status = "pending"
	n.Level = ""
	s.saveLocked("notes.json")
	return nil
}

// ---------- 视图 / 统计 ----------

func (s *Store) View(n *model.Note, viewerID string) model.NoteView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	liked, fav := false, false
	for _, id := range s.Likes[n.ID] {
		if id == viewerID {
			liked = true
			break
		}
	}
	for _, id := range s.Favs[n.ID] {
		if id == viewerID {
			fav = true
			break
		}
	}
	cc := 0
	for _, c := range s.Commen {
		if c.NoteID == n.ID {
			cc++
		}
	}
	// 关键：NoteView 里的视图字段（Views/Status/Level 等）必须是「最新」值。
	// 调用方传入的 n 可能只是 NotesList/NotesByAuthor 等在持锁期间做的值拷贝（cp := *n），
	// IncViews 在持锁期间写的是 s.Notes[id]，与 cp 不共享内存。因此这里要从 s.Notes[id]
	// 再拷一份最新视图，避免首页 feed 累加浏览量后前端仍看到旧值。
	noteCopy := *n
	if fresh, ok := s.Notes[n.ID]; ok {
		noteCopy = *fresh
	}
	return model.NoteView{
		Note: noteCopy, LikeCount: len(s.Likes[n.ID]),
		FavoriteCount: len(s.Favs[n.ID]), CommentCount: cc,
		Liked: liked, Favorited: fav,
	}
}

func (s *Store) Stats() map[string]int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	st := map[string]int{"users": 0, "pending": 0, "active": 0, "notes": 0, "pendingNotes": 0, "rejectedNotes": 0, "green": 0, "yellow": 0, "red": 0, "removed": 0, "comments": 0, "likes": 0}
	for _, u := range s.Users {
		st["users"]++
		if u.Status == "pending" {
			st["pending"]++
		} else if u.Status == "active" {
			st["active"]++
		}
	}
	for _, n := range s.Notes {
		switch n.Status {
		case "removed":
			st["removed"]++
		case "pending":
			st["pendingNotes"]++
		case "rejected":
			st["rejectedNotes"]++
		case "published":
			st["notes"]++
			switch n.Level {
			case "yellow":
				st["yellow"]++
			case "red":
				st["red"]++
			case "black":
				st["black"]++
			default:
				st["green"]++
			}
		}
	}
	st["comments"] = len(s.Commen)
	st["likes"] = len(s.Likes)
	return st
}
