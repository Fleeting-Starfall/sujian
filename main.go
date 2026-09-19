package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"sujian/internal/handler"
	"sujian/internal/store"
)

func main() {
	// 端口优先级：命令行 --port > 环境变量 PORT > 默认 8099
	port := 8099
	host := "" // 空 = 监听所有网卡
	if p := os.Getenv("PORT"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			port = v
		}
	}
	flag.IntVar(&port, "port", port, "监听端口（命令行参数，优先级高于 PORT 环境变量）")
	flag.StringVar(&host, "host", host, "监听地址（默认空=监听所有网卡 0.0.0.0；可指定 127.0.0.1 仅本机访问）")
	flag.Parse()

	// 路径基准：默认以「二进制所在目录」为根，无论项目文件夹移到哪里都能正常运行。
	// 调试（go run 临时目录）与单测兜底到当前工作目录。
	base := baseDir()

	cfg := handler.Config{
		Port:       port,
		AdminUser:  "admin",
		AdminPass:  "admin123",
		MaxImageMB: 10,
		MaxVideoMB: 200,
		UploadDir:  filepath.Join(base, "uploads"),
		PublicDir:  filepath.Join(base, "public"),
	}

	st := store.New(filepath.Join(base, "data"))
	st.SeedAdmin(cfg.AdminUser, cfg.AdminPass)
	srv := &handler.Server{Store: st, Cfg: cfg}

	mux := http.NewServeMux()

	// 静态资源
	mux.Handle("/css/", http.StripPrefix("/css/", http.FileServer(http.Dir(filepath.Join(cfg.PublicDir, "css")))))
	mux.Handle("/js/", http.StripPrefix("/js/", http.FileServer(http.Dir(filepath.Join(cfg.PublicDir, "js")))))
	// 用户上传目录：禁止目录列表（直接访问 /uploads/ 或 /uploads/<dir>/ 返回 404，只允许访问具体文件）
	// 安全：非多媒体扩展名（如 .html/.js/.zip/.exe）强制 Content-Disposition: attachment（下载而非浏览器渲染），
	// 防危险文件在浏览器直接执行。
	mux.Handle("/uploads/", http.StripPrefix("/uploads/", noDirListing(safeUploads(http.FileServer(http.Dir(cfg.UploadDir))))))

	srv.RegisterRoutes(mux)

	// 优雅退出：收到 Ctrl+C / kill 时先把内存数据（含节流中的浏览量）落盘，再退出
	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
		<-ch
		log.Println("收到退出信号，正在保存数据…")
		st.Save()
		os.Exit(0)
	}()

	addr := fmt.Sprintf(":%d", cfg.Port)
	if host != "" {
		addr = fmt.Sprintf("%s:%d", host, cfg.Port)
	}
	log.Printf("素见 (Sujian) 已启动")
	log.Printf("  本机访问:   http://localhost:%d", cfg.Port)
	if host == "" {
		log.Printf("  局域网访问: http://%s:%d", lanIP(), cfg.Port)
	} else {
		log.Printf("  仅本机访问: http://%s:%d（外部无法直接访问 %d 端口）", host, cfg.Port, cfg.Port)
	}
	log.Printf("  管理员账号: %s / %s", cfg.AdminUser, cfg.AdminPass)
	log.Printf("  数据目录:   %s", filepath.Join(base, "data"))
	log.Printf("  上传目录:   %s", cfg.UploadDir)
	log.Printf("按 Ctrl+C 停止")
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Printf("启动失败：%v", err)
		log.Printf("提示：若为端口 %d 被占用，可用 --port 端口 或 PORT=端口 重新启动", cfg.Port)
		os.Exit(1)
	}
}

// baseDir 解析项目根目录：默认用「二进制所在目录」，调试/单测时兜底到当前工作目录。
// 用 os.Executable() 拿到二进制绝对路径后取父目录；go run 会把临时编译产物放到 $TMPDIR/go-build*/exe/，
// 退回 os.Getwd()（go run 时在项目根）。
func baseDir() string {
	if exe, err := os.Executable(); err == nil {
		// 解析符号链接，得到真实路径（macOS 上 sujian-server 通常是普通文件）
		real, _ := filepath.EvalSymlinks(exe)
		if real == "" {
			real = exe
		}
		d := filepath.Dir(real)
		// 排除 go run 临时目录
		if tmp := os.TempDir(); tmp != "" && !strings.HasPrefix(d, tmp) {
			return d
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return "."
}

// noDirListing 包装文件服务：请求目录路径（空路径或 "/" 结尾）时返回 404，禁止目录浏览。
// 注意：StripPrefix 后根路径会变成空字符串 ""，需一并拦截。
func noDirListing(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "" || strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}
		h.ServeHTTP(w, r)
	})
}

// inlineUploadExts 允许浏览器内联展示（不强制下载）的扩展名白名单：
// 图片 / 视频 / 音频 / 字体 / PDF / 纯文本。其余一律附件下载。
// 注意：.svg 故意不在白名单内——SVG 可以内嵌 <script>，内联渲染等同于存储型 XSS。
// 用户上传的 .svg 一律走 attachment 下载，不交给浏览器渲染。
var inlineUploadExts = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true,
	".bmp": true, ".ico": true, ".tif": true, ".tiff": true, ".avif": true,
	".mp4": true, ".webm": true, ".mov": true, ".m4v": true, ".avi": true, ".mkv": true,
	".flv": true, ".wmv": true, ".ts": true, ".3gp": true, ".ogv": true,
	".mp3": true, ".wav": true, ".ogg": true, ".m4a": true, ".flac": true, ".aac": true,
	".woff": true, ".woff2": true, ".ttf": true, ".otf": true, ".eot": true,
	".pdf": true, ".txt": true, ".md": true,
}

// safeUploads 对非白名单扩展名强制 Content-Disposition: attachment（下载而非渲染）
func safeUploads(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ext := strings.ToLower(filepath.Ext(r.URL.Path))
		if !inlineUploadExts[ext] {
			w.Header().Set("Content-Disposition", "attachment")
		}
		h.ServeHTTP(w, r)
	})
}

// lanIP 获取本机局域网 IPv4（用于打印访问地址）
func lanIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "localhost"
	}
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ip4 := ipnet.IP.To4(); ip4 != nil {
				return ip4.String()
			}
		}
	}
	return "localhost"
}
