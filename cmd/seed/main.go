// 素见 种子数据：生成演示用户 + 各频道笔记（渐变封面图）+ 评论/点赞/收藏。
// 用法：go run ./cmd/seed        （已有数据则跳过）
//
//	go run ./cmd/seed -f     （强制清空 data/ 与 uploads/ 后重建）
package main

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"
	"path/filepath"
	"time"

	"sujian/internal/model"
	"sujian/internal/store"
)

func main() {
	force := flag.Bool("f", false, "强制重建数据")
	flag.Parse()

	if *force {
		for _, f := range []string{"users.json", "notes.json", "comments.json", "likes.json", "favorites.json", "follows.json", "notifications.json", "drafts.json", "friends.json", "friend_requests.json", "reports.json", "comment_likes.json", "messages.json"} {
			_ = os.Remove(filepath.Join("data", f))
		}
		_ = os.RemoveAll("uploads")
		_ = os.MkdirAll("uploads", 0o755)
	}

	st := store.New("data")
	if len(st.AllUsers()) > 1 {
		log.Fatal("已有数据，跳过（如需重建请加 -f）")
	}
	st.SeedAdmin("admin", "admin123")
	admin := findAdmin(st)

	mk := func(username, nickname string) *model.User {
		u, err := st.Register(username, nickname, "demo123", nil)
		if err != nil {
			log.Fatal(err)
		}
		if err := st.SetUserStatus(u.ID, "active"); err != nil {
			log.Fatal(err)
		}
		return u
	}
	bob := mk("suxiaochu", "苏小厨")   // 美食
	carol := mk("alichuanda", "阿梨") // 穿搭/旅行
	dave := mk("shumajun", "数码君")   // 数码

	// 年龄认证字段（兼容旧数据）：管理员与数码君置为已认证；苏小厨/阿梨未认证。
	// 注：该字段当前不再作为红标内容浏览门槛（红标=登录用户可见+前端确认）。
	_ = st.SetAgeVerified(admin.ID, true)
	_ = st.SetAgeVerified(dave.ID, true)

	// ---- 生成封面图 ----
	img := func(u *model.User, name string, c1, c2 color.RGBA, w, h, kind int) string {
		p := filepath.Join("uploads", u.ID)
		_ = os.MkdirAll(p, 0o755)
		fn := name + ".png"
		writeGradient(filepath.Join(p, fn), c1, c2, w, h, kind)
		return "/uploads/" + u.ID + "/" + fn
	}

	m1 := img(bob, "cafe", c(0xf6e7d8), c(0xd9a066), 600, 800, 0)
	m2 := img(bob, "breakfast", c(0xfff4d6), c(0xf7c948), 600, 800, 1)
	m3 := img(carol, "coat", c(0xe8e4f5), c(0xb8a9d9), 600, 800, 2)
	m4 := img(carol, "sea", c(0xd9f2f7), c(0x7fc4dd), 600, 800, 3)
	m5 := img(dave, "headset", c(0xe2e8f0), c(0x64748b), 600, 800, 4)
	m6 := img(admin, "welcome", c(0xffe3ec), c(0xff6f91), 600, 800, 5)
	m7 := img(dave, "horror", c(0x1f2430), c(0x8b1e1e), 600, 800, 6)

	// ---- 笔记 ----
	n1 := note(st, bob, "周末的咖啡馆：拿铁和肉桂卷",
		"藏在巷子里的独立咖啡馆，老板娘自己做肉桂卷。拿铁豆子偏深烘，坚果香很足。\n\n推荐窗边的位置，下午的光线特别好看。",
		"美食", []string{"探店", "咖啡"}, []string{m1})
	n2 := note(st, bob, "十分钟早餐：牛油果吐司",
		"睡懒觉也能吃上像样的早餐：\n\n1. 全麦吐司烤到微焦\n2. 牛油果压泥，加盐和黑胡椒\n3. 上面放一个溏心蛋\n\n真的只要十分钟。",
		"美食", []string{"早餐", "懒人食谱"}, []string{m2})
	n3 := note(st, carol, "秋日通勤：米色大衣怎么搭",
		"米色大衣是秋天的安全牌。内搭选同色系针织衫，下半身直筒牛仔裤，配一双乐福鞋。\n\n重点：大衣敞开穿，露出腰线，显高不压个子。",
		"穿搭", []string{"通勤", "大衣"}, []string{m3})
	n4 := note(st, carol, "海边小城两天一夜",
		"周末去了一个没什么游客的海边小城。\n\nDay1：环岛骑行 → 日落海鲜排档\nDay2：渔村早市 → 山顶咖啡店\n\n人少、便宜、治愈。",
		"旅行", []string{"海边", "周末游"}, []string{m4})
	n5 := note(st, dave, "千元降噪耳机开箱",
		"入了这阵子很火的千元档降噪耳机，直接说结论：\n\n- 降噪：地铁上基本安静\n- 续航：单次 8 小时\n- 音质：中规中矩，偏流行调音\n\n总评：通勤够用，性价比不错。",
		"数码", []string{"开箱", "耳机"}, []string{m5})
	n6 := note(st, admin, "欢迎来到素见",
		"这是我们的局域网小社区。\n\n在这里可以：\n· 发布图文 / 视频笔记\n· 点赞、收藏、评论、关注\n· 去 /admin 审核新注册的成员\n\n第一个账号已就位，欢迎开始创作～",
		"生活", []string{"欢迎", "使用指南"}, []string{m6})
	// 演示内容等级：耳机开箱标为黄标（自由浏览不进首页）、恐怖片单标为红标（敏感内容，登录后需确认浏览）
	n7 := note(st, dave, "深夜恐怖片单：胆小勿点",
		"整理了一波适合深夜看的恐怖片，口味偏重：\n\n- 招魂系列：jump scare 教科书\n- 遗传厄运：心理恐怖代表作\n- 咒怨：看完不敢关灯\n\n未成年人请在家长陪同下观看。",
		"影视", []string{"恐怖片", "片单"}, []string{m7})
	_ = st.ReviewNote(n5.ID, "published", "yellow") // 黄标：不进首页
	_ = st.ReviewNote(n7.ID, "published", "red")    // 红标：敏感内容，需登录确认

	// ---- 初始浏览量（模拟数据热度）----
	for _, nn := range []*model.Note{n1, n2, n3, n4, n5, n6, n7} {
		st.IncViews(nn.ID, 20+int(nn.CreatedAt%13))
	}

	// ---- 评论（含楼中楼回复）----
	c1 := st.AddComment(n1.ID, carol.ID, carol.Nickname, "收藏了，周末就去！", "", "")
	st.AddComment(n3.ID, bob.ID, bob.Nickname, "大衣链接呢（敲碗）", "", "")
	c2 := st.AddComment(n5.ID, admin.ID, admin.Nickname, "写得很详细，学到了", "", "")
	st.AddComment(n5.ID, dave.ID, dave.Nickname, "谢谢管理员支持～后续会更新音质横评", c2.ID, admin.Nickname)
	st.AddComment(n2.ID, carol.ID, carol.Nickname, "牛油果 yyds！", "", "")

	// ---- 评论点赞演示 ----
	_ = st.ToggleCommentLike(c1.ID, bob.ID)
	_ = st.ToggleCommentLike(c1.ID, dave.ID)
	_ = st.ToggleCommentLike(c1.ID, admin.ID)
	_ = st.ToggleCommentLike(c2.ID, dave.ID)

	// ---- 私信演示：数码君 -> 苏小厨（未读）；苏小厨 -> 阿梨 ----
	st.AddMessage(dave.ID, dave.Nickname, bob.ID, "兄弟，那篇耳机测评太及时了！", "", "", "", 0)
	st.AddMessage(dave.ID, dave.Nickname, bob.ID, "我正好想入一款千元降噪", "", "", "", 0)
	st.AddMessage(bob.ID, bob.Nickname, carol.ID, "阿梨，你推荐的海边小城在哪个市？", "", "", "", 0)
	st.AddMessage(carol.ID, carol.Nickname, bob.ID, "在霞浦，人少景美，值得去！", "", "", "", 0)

	// ---- 置顶演示：苏小厨置顶咖啡笔记 ----
	_ = st.SetPin(n1.ID, bob.ID, true)

	// ---- 好友演示：苏小厨<->阿梨 已是好友；数码君 -> 苏小厨 待处理 ----
	if fr, err := st.SendFriendRequest(bob.ID, carol.Uid); err == nil {
		_ = st.AcceptFriendRequest(fr.ID, carol.ID)
	}
	if fr2, err := st.SendFriendRequest(dave.ID, bob.Uid); err == nil {
		_ = fr2
	}
	st.AddNotification(&model.Notification{
		ID: "nt_" + randHex8(), UserID: bob.ID, ActorID: dave.ID, ActorName: dave.Nickname,
		Type: "friend", CreatedAt: time.Now().Unix(),
	})

	// ---- 点赞 / 收藏 ----
	st.ToggleLike(n1.ID, admin.ID)
	st.ToggleLike(n1.ID, carol.ID)
	st.ToggleLike(n2.ID, admin.ID)
	st.ToggleLike(n2.ID, dave.ID)
	st.ToggleLike(n3.ID, bob.ID)
	st.ToggleLike(n4.ID, bob.ID)
	st.ToggleLike(n4.ID, admin.ID)
	st.ToggleLike(n5.ID, carol.ID)
	st.ToggleLike(n6.ID, bob.ID)
	st.ToggleLike(n6.ID, carol.ID)
	st.ToggleLike(n6.ID, dave.ID)

	st.ToggleFav(n1.ID, carol.ID)
	st.ToggleFav(n4.ID, bob.ID)
	st.ToggleFav(n2.ID, dave.ID)

	// ---- 关注 ----
	st.ToggleFollow(bob.ID, carol.ID)
	st.ToggleFollow(carol.ID, bob.ID)
	st.ToggleFollow(dave.ID, bob.ID)
	st.ToggleFollow(bob.ID, admin.ID)

	// ---- 消息通知（演示"被赞/被评论/被关注"提醒）----
	nt := func(recvID string, actor *model.User, typ, noteID, noteTitle, commentID string) {
		st.AddNotification(&model.Notification{
			ID: "nt_" + randHex8(), UserID: recvID, ActorID: actor.ID, ActorName: actor.Nickname,
			Type: typ, NoteID: noteID, NoteTitle: noteTitle, CommentID: commentID,
			CreatedAt: time.Now().Unix(),
		})
	}
	nt(bob.ID, carol, "like", n1.ID, n1.Title, "")
	nt(bob.ID, dave, "fav", n2.ID, n2.Title, "")
	nt(carol.ID, bob, "comment", n3.ID, n3.Title, "")
	nt(admin.ID, dave, "reply", n5.ID, n5.Title, c2.ID)
	nt(bob.ID, carol, "follow", "", "", "")

	log.Println("种子数据完成")
	log.Println("演示账号（密码均为 demo123）：苏小厨 / 阿梨 / 数码君")
	log.Println("管理员：admin / admin123")
}

func findAdmin(st *store.Store) *model.User {
	for _, u := range st.AllUsers() {
		if u.Role == "admin" {
			return st.UserByID(u.ID)
		}
	}
	log.Fatal("未找到管理员")
	return nil
}

func note(st *store.Store, author *model.User, title, content, category string, tags, media []string) *model.Note {
	return st.AddNote(author, title, content, "image", media, "", tags, category, "published", "green")
}

func c(hex uint32) color.RGBA {
	return color.RGBA{uint8(hex >> 16), uint8(hex >> 8), uint8(hex), 255}
}

// writeGradient 生成竖向渐变 + 半透明圆装饰的 PNG 封面
func writeGradient(path string, c1, c2 color.RGBA, w, h, kind int) {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		t := float64(y) / float64(h)
		for x := 0; x < w; x++ {
			tx := t + 0.35*float64(x)/float64(w)
			img.SetRGBA(x, y, lerp(c1, c2, tx))
		}
	}
	// 装饰圆
	drawCircle(img, 0.78*float64(w), 0.22*float64(h), 0.16*float64(h), color.RGBA{255, 255, 255, 60})
	drawCircle(img, 0.2*float64(w), 0.85*float64(h), 0.2*float64(h), color.RGBA{255, 255, 255, 45})
	if kind%3 != 0 {
		drawCircle(img, 0.55*float64(w), 0.55*float64(h), 0.28*float64(h), color.RGBA{255, 255, 255, 28})
	}
	f, err := os.Create(path)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		log.Fatal(err)
	}
}

func lerp(a, b color.RGBA, t float64) color.RGBA {
	if t > 1 {
		t = 1
	}
	return color.RGBA{
		uint8(float64(a.R) + t*float64(b.R-a.R)),
		uint8(float64(a.G) + t*float64(b.G-a.G)),
		uint8(float64(a.B) + t*float64(b.B-a.B)),
		255,
	}
}

func drawCircle(img *image.RGBA, cx, cy, r float64, c color.RGBA) {
	b := img.Bounds()
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			dx, dy := float64(x)-cx, float64(y)-cy
			if dx*dx+dy*dy > r*r {
				continue
			}
			old := img.RGBAAt(x, y)
			a := float64(c.A) / 255
			img.SetRGBA(x, y, color.RGBA{
				uint8(float64(old.R)*(1-a) + float64(c.R)*a),
				uint8(float64(old.G)*(1-a) + float64(c.G)*a),
				uint8(float64(old.B)*(1-a) + float64(c.B)*a),
				255,
			})
		}
	}
}

// 确保 math 被使用（预留，后续可扩展装饰形状）
var _ = math.Pi

func randHex8() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
