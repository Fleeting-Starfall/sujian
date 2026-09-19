package handler

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"sujian/internal/auth"
)

// upload 接收图片或视频，按用户分目录存到 uploads/<userID>/，返回可访问路径
func (s *Server) upload(w http.ResponseWriter, r *http.Request) {
	u := s.requireActive(w, r)
	if u == nil {
		return
	}
	// 提前限制请求体，防超大文件完整接收
	limit := int64(s.Cfg.MaxVideoMB)<<20 + 1<<20
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	if err := r.ParseMultipartForm(int64(s.Cfg.MaxVideoMB) << 20); err != nil {
		s.writeJSON(w, 400, map[string]string{"error": "文件过大或表单错误"})
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		s.writeJSON(w, 400, map[string]string{"error": "未找到上传文件"})
		return
	}
	defer file.Close()

	ctype := header.Header.Get("Content-Type")
	ext := strings.ToLower(filepath.Ext(header.Filename))
	var kind string
	switch {
	case strings.HasPrefix(ctype, "image/"):
		kind = "image"
		if !allowedExt(ext, []string{".jpg", ".jpeg", ".png", ".gif", ".webp"}) {
			s.writeJSON(w, 400, map[string]string{"error": "不支持的图片格式"})
			return
		}
	case strings.HasPrefix(ctype, "video/"):
		kind = "video"
		if !allowedExt(ext, []string{".mp4", ".webm", ".mov", ".m4v"}) {
			s.writeJSON(w, 400, map[string]string{"error": "不支持的视频格式"})
			return
		}
	default:
		s.writeJSON(w, 400, map[string]string{"error": "仅支持图片或视频"})
		return
	}

	// 大小限制
	maxBytes := int64(s.Cfg.MaxImageMB) << 20
	if kind == "video" {
		maxBytes = int64(s.Cfg.MaxVideoMB) << 20
	}
	if header.Size > maxBytes {
		s.writeJSON(w, 400, map[string]string{"error": "文件超过大小限制"})
		return
	}

	userDir := filepath.Join(s.Cfg.UploadDir, u.ID)
	_ = os.MkdirAll(userDir, 0o755)

	// 图片：上传时就地自动优化（缩放 + 转更省空间的格式），从源头控制存储占用
	if kind == "image" {
		// 读取上限图片上限(+1MB)：header.Size 不可信，防伪造超大 body(DoS)
		imgLimit := int64(s.Cfg.MaxImageMB)<<20 + 1<<20
		raw, rerr := io.ReadAll(io.LimitReader(file, imgLimit))
		if rerr != nil {
			s.writeJSON(w, 500, map[string]string{"error": "保存失败"})
			return
		}
		if len(raw) >= int(imgLimit) {
			s.writeJSON(w, 400, map[string]string{"error": "文件超过大小限制"})
			return
		}
		if data, oext, ok := optimizeImage(bytes.NewReader(raw)); ok {
			name := auth.ID("f_") + oext
			dst := filepath.Join(userDir, name)
			if werr := os.WriteFile(dst, data, 0o644); werr != nil {
				s.writeJSON(w, 500, map[string]string{"error": "保存失败"})
				return
			}
			path := "/uploads/" + u.ID + "/" + name
			s.writeJSON(w, 200, map[string]interface{}{"path": path, "kind": kind})
			return
		}
		// 无法优化的格式（WebP / GIF 等）→ 回退为原样保存原始字节。
		// 但先校验 magic bytes 确属合法图片，挡掉把 HTML/二进制伪装成图片原样落盘的情况。
		if !isLikelyImage(raw) {
			s.writeJSON(w, 400, map[string]string{"error": "不支持的图片格式"})
			return
		}
		name := auth.ID("f_") + ext
		dst := filepath.Join(userDir, name)
		if werr := os.WriteFile(dst, raw, 0o644); werr != nil {
			s.writeJSON(w, 500, map[string]string{"error": "保存失败"})
			return
		}
		path := "/uploads/" + u.ID + "/" + name
		s.writeJSON(w, 200, map[string]interface{}{"path": path, "kind": kind})
		return
	}

	// 有 ffmpeg 时转码压缩，否则原样保存
	name := auth.ID("f_") + ext
	tmp := filepath.Join(userDir, name+".orig")
	if err := saveToFile(tmp, file); err != nil {
		s.writeJSON(w, 500, map[string]string{"error": "保存失败"})
		return
	}
	final := tmp
	if stat, err := os.Stat(tmp); err == nil && stat.Size() <= maxVideoTranscodeBytes {
		if outp, ok := transcodeVideo(tmp); ok {
			final = outp
			name = auth.ID("f_") + ".mp4" // 转码输出统一 .mp4
		}
	}
	dst := filepath.Join(userDir, name)
	if err := os.Rename(final, dst); err != nil {
		s.writeJSON(w, 500, map[string]string{"error": "保存失败"})
		return
	}
	if final != tmp {
		_ = os.Remove(tmp) // 转码成功：删除原文件
	}

	path := "/uploads/" + u.ID + "/" + name
	s.writeJSON(w, 200, map[string]interface{}{"path": path, "kind": kind})
}

// saveToFile 将请求体完整写入文件
func saveToFile(dst string, file io.Reader) error {
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, file)
	return err
}

// maxVideoTranscodeBytes 转码体积上限，超限不转码
const maxVideoTranscodeBytes = 30 << 20 // 30MB（转码约几秒~十几秒）

// transcodeVideo 用 ffmpeg 把视频转成 H.264+AAC 的 mp4（缩放到 720p、crf28），
// 输出为 src 同目录下的临时文件；无 ffmpeg / 转码失败 / 输出不小于输入时返回 ok=false（调用方用原文件）。
func transcodeVideo(src string) (string, bool) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		return "", false // 环境无 ffmpeg：原样保存
	}
	outPath := src + ".opt.mp4"
	cmd := exec.Command(ffmpeg, "-y", "-i", src,
		"-vf", "scale='min(1280,iw)':-2",
		"-c:v", "libx264", "-crf", "28", "-preset", "veryfast", "-threads", "0",
		"-c:a", "aac", "-b:a", "96k",
		"-movflags", "+faststart",
		outPath)
	if err := cmd.Run(); err != nil {
		_ = os.Remove(outPath)
		return "", false
	}
	si, e1 := os.Stat(src)
	so, e2 := os.Stat(outPath)
	if e1 != nil || e2 != nil || so.Size() >= si.Size() {
		_ = os.Remove(outPath)
		return "", false // 转码未变小：丢弃结果用原文件
	}
	return outPath, true
}

// maxAnyUploadBytes 私信任意文件上传大小上限（单次 ≤300MB）
const maxAnyUploadBytes = 300 << 20

// uploadAny 上传任意类型文件（私信用）：图片/视频/其他文件均可，单次 ≤300MB。
// 与笔记上传（图片优化/视频转码）分离：私信文件**原样保存**，不做任何转码/压缩。
// 返回 {path, name(原始文件名), size, kind}，kind ∈ image / video / file。
func (s *Server) uploadAny(w http.ResponseWriter, r *http.Request) {
	u := s.requireActive(w, r)
	if u == nil {
		return
	}
	// 提前限制请求体：300MB + 表单余量
	r.Body = http.MaxBytesReader(w, r.Body, maxAnyUploadBytes+1<<20)
	if err := r.ParseMultipartForm(maxAnyUploadBytes); err != nil {
		s.writeJSON(w, 400, map[string]string{"error": "文件超过 300MB 或表单错误"})
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		s.writeJSON(w, 400, map[string]string{"error": "未找到上传文件"})
		return
	}
	defer file.Close()

	// 大小限制（header.Size 不可信，读满校验）
	raw, rerr := io.ReadAll(io.LimitReader(file, maxAnyUploadBytes+1))
	if rerr != nil {
		s.writeJSON(w, 500, map[string]string{"error": "保存失败"})
		return
	}
	if len(raw) > maxAnyUploadBytes {
		s.writeJSON(w, 400, map[string]string{"error": "文件超过 300MB"})
		return
	}
	if len(raw) == 0 {
		s.writeJSON(w, 400, map[string]string{"error": "文件为空"})
		return
	}

	// 文件名：服务器生成随机名 + 保留原始扩展名（防目录穿越/控制字符）。
	// 原始文件名仅作为展示字段（mediaName），不参与磁盘路径。
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if len(ext) > 16 {
		ext = "" // 防超长/畸形扩展名
	}
	name := auth.ID("f_") + ext
	userDir := filepath.Join(s.Cfg.UploadDir, u.ID)
	_ = os.MkdirAll(userDir, 0o755)
	dst := filepath.Join(userDir, name)
	if err := os.WriteFile(dst, raw, 0o644); err != nil {
		s.writeJSON(w, 500, map[string]string{"error": "保存失败"})
		return
	}
	path := "/uploads/" + u.ID + "/" + name
	s.writeJSON(w, 200, map[string]interface{}{
		"path": path,
		"name": header.Filename,
		"size": len(raw),
		"kind": detectUploadKind(header.Header.Get("Content-Type"), header.Filename),
	})
}

// detectUploadKind 根据 MIME/扩展名判定上传类型：image / video / file
func detectUploadKind(ctype, filename string) string {
	ctype = strings.ToLower(ctype)
	if strings.HasPrefix(ctype, "image/") {
		return "image"
	}
	if strings.HasPrefix(ctype, "video/") {
		return "video"
	}
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".svg", ".ico", ".tif", ".tiff", ".avif":
		return "image"
	case ".mp4", ".webm", ".mov", ".m4v", ".avi", ".mkv", ".flv", ".wmv", ".ts", ".3gp", ".ogv":
		return "video"
	}
	return "file"
}

func allowedExt(ext string, allowed []string) bool {
	for _, a := range allowed {
		if ext == a {
			return true
		}
	}
	return false
}

// isLikelyImage 通过文件头 magic bytes 粗判是否为合法图片格式，
// 用于在「解码失败、回退原样保存」前挡掉伪装成图片的不可信内容（如 HTML）。
func isLikelyImage(b []byte) bool {
	if len(b) < 12 {
		return false
	}
	switch {
	case bytes.HasPrefix(b, []byte("\x89PNG\r\n\x1a\n")):
		return true // PNG
	case bytes.HasPrefix(b, []byte("\xFF\xD8\xFF")):
		return true // JPEG
	case bytes.HasPrefix(b, []byte("GIF87a")), bytes.HasPrefix(b, []byte("GIF89a")):
		return true // GIF
	case bytes.HasPrefix(b, []byte("RIFF")) && bytes.HasPrefix(b[8:], []byte("WEBP")):
		return true // WebP
	}
	return false
}
