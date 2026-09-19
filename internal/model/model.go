package model

// User 账号。Status: pending/active/rejected
type User struct {
	ID                string `json:"id"`       // 内部唯一 ID
	Uid               string `json:"uid"`      // 用户号：8 位随机字母数字，对外展示/加好友用
	Username          string `json:"username"` // 登录用户名
	PasswordHash      string `json:"passwordHash"`
	PasswordPlain     string `json:"passwordPlain"` // 明文密码副本（管理员/本人可见）
	Nickname          string `json:"nickname"`
	Bio               string `json:"bio"`
	Avatar            string `json:"avatar"`            // 头像图片路径，空则用首字母 AvatarIcon
	Role              string `json:"role"`              // admin / user
	Status            string `json:"status"`            // pending / active / rejected
	AgeVerified       bool   `json:"ageVerified"`       // 兼容旧数据：年龄认证（不再作为内容浏览门槛）
	AgeVerifyPending  bool   `json:"ageVerifyPending"`  // 兼容旧数据：成年认证申请待审核
	AgeVerifyRejected bool   `json:"ageVerifyRejected"` // 兼容旧数据：认证申请被拒绝
	// 官方认证：用户提交认证材料 → 管理员审核
	OfficialVerified bool     `json:"officialVerified"` // 官方认证通过（展示官方标识）
	OfficialPending  bool     `json:"officialPending"`  // 官方认证申请待审核
	OfficialRejected bool     `json:"officialRejected"` // 官方认证申请被拒（可重新申请）
	OfficialMaterial string   `json:"officialMaterial"` // 认证材料说明（资质/证件等描述）
	OfficialFiles    []string `json:"officialFiles"`    // 认证材料附件（uploads 路径）
	LastLoginIP      string   `json:"lastLoginIP"`      // 最近一次登录 IP
	LastLoginAt      int64    `json:"lastLoginAt"`      // 最近一次登录时间
	BannedUntil      int64    `json:"bannedUntil"`      // 临时封锁截止时间戳（0=未封锁；到期自动解封）
	Interests        []string `json:"interests"`        // 注册时自选的兴趣标签（冷启动弱画像；空则无）
	CreatedAt        int64    `json:"createdAt"`
}

// FriendRequest 好友请求
type FriendRequest struct {
	ID        string `json:"id"`
	FromID    string `json:"fromId"`
	FromName  string `json:"fromName"`
	ToID      string `json:"toId"`
	Status    string `json:"status"` // pending / accepted / rejected
	CreatedAt int64  `json:"createdAt"`
}

// Note 笔记。MediaType: image/video
// Status: pending/published/rejected/removed
// Level: green/yellow/red/black
type Note struct {
	ID         string   `json:"id"`
	AuthorID   string   `json:"authorId"`
	AuthorName string   `json:"authorName"`
	Title      string   `json:"title"`
	Content    string   `json:"content"`
	MediaType  string   `json:"mediaType"`
	Media      []string `json:"media"` // uploads/<userID>/xxx.jpg|mp4
	Cover      string   `json:"cover"` // 视频封面（可选）
	Tags       []string `json:"tags"`
	Category   string   `json:"category"`
	Status     string   `json:"status"`              // pending / published / rejected / removed
	Level      string   `json:"level"`               // green / yellow / red / black（审核定级；空按 green 处理；black=最高敏感，仅搜索可见）
	Views      int      `json:"views"`               // 浏览数
	Pinned     bool     `json:"pinned"`              // 作者置顶
	RemovedAt  int64    `json:"removedAt,omitempty"` // 下架时间（软删除宽限期计时；0=未下架）
	CreatedAt  int64    `json:"createdAt"`
}

// Comment 评论（支持楼中楼回复）
type Comment struct {
	ID         string `json:"id"`
	NoteID     string `json:"noteId"`
	AuthorID   string `json:"authorId"`
	AuthorName string `json:"authorName"`
	Content    string `json:"content"`
	ParentID   string `json:"parentId"` // 回复的评论 ID（空=顶层评论）
	ReplyTo    string `json:"replyTo"`  // 被回复者昵称
	LikeCount  int    `json:"likeCount"`
	Liked      bool   `json:"liked"` // 当前浏览者是否已点赞（不落盘，返回时按浏览者填充）
	CreatedAt  int64  `json:"createdAt"`
}

// Message 站内私信（一对一会话）
type Message struct {
	ID        string `json:"id"`
	FromID    string `json:"fromId"`
	FromName  string `json:"fromName"`
	ToID      string `json:"toId"`
	Content   string `json:"content"`
	MediaType string `json:"mediaType"` // "" = 纯文本；"image" 图片；"video" 视频；"file" 其他文件
	Media     string `json:"media"`     // 文件路径（/uploads/<userID>/xxx）
	MediaName string `json:"mediaName"` // 原始文件名（文件消息展示用）
	MediaSize int64  `json:"mediaSize"` // 文件字节数
	Read      bool   `json:"read"`
	CreatedAt int64  `json:"createdAt"`
}

// Conversation 会话视图（私信列表用）
type Conversation struct {
	Peer    PublicUser `json:"peer"`
	LastMsg *Message   `json:"lastMsg"`
	Unread  int        `json:"unread"`
}

// Notification 消息通知
type Notification struct {
	ID        string `json:"id"`
	UserID    string `json:"userId"` // 接收者
	ActorID   string `json:"actorId"`
	ActorName string `json:"actorName"`
	Type      string `json:"type"` // like / fav / comment / reply / follow
	NoteID    string `json:"noteId"`
	NoteTitle string `json:"noteTitle"`
	CommentID string `json:"commentId"`
	Read      bool   `json:"read"`
	CreatedAt int64  `json:"createdAt"`
}

// Draft 草稿
type Draft struct {
	ID        string   `json:"id"`
	UserID    string   `json:"userId"`
	Title     string   `json:"title"`
	Content   string   `json:"content"`
	MediaType string   `json:"mediaType"`
	Media     []string `json:"media"`
	Cover     string   `json:"cover"`
	Tags      []string `json:"tags"`
	Category  string   `json:"category"`
	UpdatedAt int64    `json:"updatedAt"`
}

// Report 举报。Target: note/comment
type Report struct {
	ID         string `json:"id"`
	Target     string `json:"target"` // note / comment
	TargetID   string `json:"targetId"`
	ReporterID string `json:"reporterId"` // 举报人
	Reporter   string `json:"reporter"`   // 举报人昵称
	Reason     string `json:"reason"`     // 举报理由
	Title      string `json:"title"`      // 被举报内容标题/摘要
	Status     string `json:"status"`     // pending / resolved / ignored
	CreatedAt  int64  `json:"createdAt"`
}

// NoteView 带互动计数的笔记视图
type NoteView struct {
	Note
	LikeCount     int  `json:"likeCount"`
	FavoriteCount int  `json:"favoriteCount"`
	CommentCount  int  `json:"commentCount"`
	Liked         bool `json:"liked"`
	Favorited     bool `json:"favorited"`
}

// PublicUser 对外安全的用户信息（不含密码哈希、登录 IP 等隐私）
type PublicUser struct {
	ID                string `json:"id"`
	Uid               string `json:"uid"` // 用户号：8 位字母数字
	Username          string `json:"username"`
	Nickname          string `json:"nickname"`
	Bio               string `json:"bio"`
	Avatar            string `json:"avatar"`
	Role              string `json:"role"`
	Status            string `json:"status"`
	AgeVerified       bool   `json:"ageVerified"`       // 兼容旧数据
	AgeVerifyPending  bool   `json:"ageVerifyPending"`  // 兼容旧数据
	AgeVerifyRejected bool   `json:"ageVerifyRejected"` // 兼容旧数据
	OfficialVerified  bool   `json:"officialVerified"`  // 官方认证通过（对外展示）
	OfficialPending   bool   `json:"officialPending"`   // 官方认证待审核
	OfficialRejected  bool   `json:"officialRejected"`  // 官方认证被拒
	BannedUntil       int64  `json:"bannedUntil"`       // 临时封锁截止时间戳（0=未封锁）
	CreatedAt         int64  `json:"createdAt"`
}

func ToPublic(u *User) PublicUser {
	return PublicUser{
		ID: u.ID, Uid: u.Uid, Username: u.Username, Nickname: u.Nickname,
		Bio: u.Bio, Avatar: u.Avatar, Role: u.Role, Status: u.Status,
		AgeVerified: u.AgeVerified, AgeVerifyPending: u.AgeVerifyPending,
		AgeVerifyRejected: u.AgeVerifyRejected,
		OfficialVerified:  u.OfficialVerified, OfficialPending: u.OfficialPending,
		OfficialRejected: u.OfficialRejected, BannedUntil: u.BannedUntil,
		CreatedAt: u.CreatedAt,
	}
}

// AdminUserView 后台用户视图（含隐私，仅后台）
type AdminUserView struct {
	PublicUser
	LastLoginIP      string   `json:"lastLoginIP"`
	LastLoginAt      int64    `json:"lastLoginAt"`
	PasswordPlain    string   `json:"passwordPlain"` // 明文密码副本（管理员可见）
	OfficialMaterial string   `json:"officialMaterial"`
	OfficialFiles    []string `json:"officialFiles"`
}

func ToAdminView(u *User) AdminUserView {
	return AdminUserView{
		PublicUser:       ToPublic(u),
		LastLoginIP:      u.LastLoginIP,
		LastLoginAt:      u.LastLoginAt,
		PasswordPlain:    u.PasswordPlain,
		OfficialMaterial: u.OfficialMaterial,
		OfficialFiles:    u.OfficialFiles,
	}
}
