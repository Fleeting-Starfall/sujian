package handler

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// makeNoisyPNG 带噪声 PNG，便于验证压缩收益
func makeNoisyPNG(size int, alpha uint8) []byte {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	seed := uint32(0x9e3779b9)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			seed = seed*1664525 + 1013904223
			v := byte(seed >> 16)
			img.Set(x, y, color.RGBA{v, v / 2, 255 - v, alpha})
		}
	}
	var buf bytes.Buffer
	png.Encode(&buf, img)
	return buf.Bytes()
}

// makeGradientPNG 可压缩渐变 PNG
func makeGradientPNG(size int, alpha uint8) []byte {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			img.Set(x, y, color.RGBA{
				R: uint8(x * 255 / size),
				G: uint8(y * 255 / size),
				B: uint8((x + y) * 255 / (2 * size)),
				A: alpha,
			})
		}
	}
	var buf bytes.Buffer
	png.Encode(&buf, img)
	return buf.Bytes()
}

func TestOptimizeImage_JPEG(t *testing.T) {
	raw := makeNoisyPNG(2000, 255) // 不透明
	out, ext, ok := optimizeImage(bytes.NewReader(raw))
	if !ok || ext != ".jpg" {
		t.Fatalf("期望 .jpg 且 ok=true，实际 ext=%q ok=%v", ext, ok)
	}
	if len(out) >= len(raw) {
		t.Fatalf("优化后未变小: %d >= %d", len(out), len(raw))
	}
	img, _, err := image.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("输出不是合法 JPEG: %v", err)
	}
	if d := img.Bounds().Dx(); d > optMaxDim {
		t.Fatalf("未缩放: 宽=%d > %d", d, optMaxDim)
	}
}

func TestOptimizeImage_TransparentPNG(t *testing.T) {
	raw := makeNoisyPNG(2000, 128) // 半透明
	out, ext, ok := optimizeImage(bytes.NewReader(raw))
	if !ok || ext != ".png" {
		t.Fatalf("期望 .png 且 ok=true，实际 ext=%q ok=%v", ext, ok)
	}
	if len(out) >= len(raw) {
		t.Fatalf("优化后未变小: %d >= %d", len(out), len(raw))
	}
}

func TestOptimizeImage_UnknownFormat(t *testing.T) {
	out, ext, ok := optimizeImage(bytes.NewReader([]byte("this is not an image")))
	if ok || len(out) != 0 || ext != "" {
		t.Fatalf("未知格式应 ok=false 且返回空，实际 ok=%v len=%d ext=%q", ok, len(out), ext)
	}
}

// TestUploadOptimizesImage 上传大图后落盘为缩放后的更小 jpg。
func TestUploadOptimizesImage(t *testing.T) {
	app, cleanup := newTestApp(t)
	defer cleanup()
	token, adminID := app.login(t, "admin", "admin123")

	raw := makeGradientPNG(2000, 255)
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	hdr := make(textproto.MIMEHeader)
	hdr.Set("Content-Disposition", `form-data; name="file"; filename="big.png"`)
	hdr.Set("Content-Type", "image/png")
	fw, err := mw.CreatePart(hdr)
	if err != nil {
		t.Fatal(err)
	}
	fw.Write(raw)
	mw.Close()

	req := httptest.NewRequest("POST", "/api/upload", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.AddCookie(&http.Cookie{Name: "session", Value: token})
	rec := httptest.NewRecorder()
	app.mux.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("上传期望 200，实际 %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Path string `json:"path"`
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Kind != "image" {
		t.Fatalf("kind 应为 image，实际 %q", resp.Kind)
	}
	if filepath.Ext(resp.Path) != ".jpg" {
		t.Fatalf("优化后扩展名应为 .jpg，实际 %q", resp.Path)
	}

	// 落盘文件验证
	saved := filepath.Join(app.dir, resp.Path[1:]) // 去掉前缀 /
	data, err := os.ReadFile(saved)
	if err != nil {
		t.Fatalf("读取落盘文件失败: %v", err)
	}
	if len(data) >= len(raw) {
		t.Fatalf("落盘文件未变小: %d >= %d", len(data), len(raw))
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("落盘文件不是合法图片: %v", err)
	}
	if img.Bounds().Dx() > optMaxDim || img.Bounds().Dy() > optMaxDim {
		t.Fatalf("落盘文件未缩放: %dx%d", img.Bounds().Dx(), img.Bounds().Dy())
	}
	_ = adminID
}

// TestUploadRejectsOversizeImage 超大 body 被截断并按超限拒绝，不读进内存。
func TestUploadRejectsOversizeImage(t *testing.T) {
	app, cleanup := newTestApp(t)
	defer cleanup()
	token, _ := app.login(t, "admin", "admin123")

	// 12MB > 上限(10MB+1MB 余量)
	payload := bytes.Repeat([]byte("A"), 12<<20)
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	hdr := make(textproto.MIMEHeader)
	hdr.Set("Content-Disposition", `form-data; name="file"; filename="big.png"`)
	hdr.Set("Content-Type", "image/png")
	fw, err := mw.CreatePart(hdr)
	if err != nil {
		t.Fatal(err)
	}
	fw.Write(payload)
	mw.Close()

	req := httptest.NewRequest("POST", "/api/upload", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.AddCookie(&http.Cookie{Name: "session", Value: token})
	rec := httptest.NewRecorder()
	app.mux.ServeHTTP(rec, req)
	if rec.Code != 400 {
		t.Fatalf("超大伪图片应被拒(400)，实际 %d: %s", rec.Code, rec.Body.String())
	}
}

// TestUploadRejectsDisguisedFile 伪装图片被 magic bytes 拦截。
func TestUploadRejectsDisguisedFile(t *testing.T) {
	app, cleanup := newTestApp(t)
	defer cleanup()
	token, _ := app.login(t, "admin", "admin123")

	for _, tc := range []struct{ name, ctype, filename string }{
		{"html_as_png", "image/png", "evil.png"},
		{"html_as_webp", "image/webp", "evil.webp"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload := []byte(`<html><script>alert(1)</script></html>`)
			var body bytes.Buffer
			mw := multipart.NewWriter(&body)
			hdr := make(textproto.MIMEHeader)
			hdr.Set("Content-Disposition", `form-data; name="file"; filename="`+tc.filename+`"`)
			hdr.Set("Content-Type", tc.ctype)
			fw, err := mw.CreatePart(hdr)
			if err != nil {
				t.Fatal(err)
			}
			fw.Write(payload)
			mw.Close()

			req := httptest.NewRequest("POST", "/api/upload", &body)
			req.Header.Set("Content-Type", mw.FormDataContentType())
			req.AddCookie(&http.Cookie{Name: "session", Value: token})
			rec := httptest.NewRecorder()
			app.mux.ServeHTTP(rec, req)
			if rec.Code != 400 {
				t.Fatalf("伪装文件应被拒(400)，实际 %d: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

// TestUploadTranscodesVideo 有 ffmpeg 时视频自动转码压缩。
func TestUploadTranscodesVideo(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("环境无 ffmpeg，跳过视频转码测试")
	}
	app, cleanup := newTestApp(t)
	defer cleanup()
	token, _ := app.login(t, "admin", "admin123")

	// 用 ffmpeg 生成高码率测试视频
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "src.mp4")
	cmd := exec.Command("ffmpeg", "-y", "-f", "lavfi", "-i", "testsrc=duration=1:size=1280x720:rate=30",
		"-c:v", "libx264", "-b:v", "8M", "-pix_fmt", "yuv420p", src)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("生成测试视频失败: %v %s", err, out)
	}
	raw, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	hdr := make(textproto.MIMEHeader)
	hdr.Set("Content-Disposition", `form-data; name="file"; filename="test.mp4"`)
	hdr.Set("Content-Type", "video/mp4")
	fw, err := mw.CreatePart(hdr)
	if err != nil {
		t.Fatal(err)
	}
	fw.Write(raw)
	mw.Close()

	req := httptest.NewRequest("POST", "/api/upload", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.AddCookie(&http.Cookie{Name: "session", Value: token})
	rec := httptest.NewRecorder()
	app.mux.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("上传期望 200，实际 %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Path string `json:"path"`
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Kind != "video" {
		t.Fatalf("kind 应为 video，实际 %q", resp.Kind)
	}
	saved := filepath.Join(app.dir, resp.Path[1:])
	data, err := os.ReadFile(saved)
	if err != nil {
		t.Fatalf("读取落盘文件失败: %v", err)
	}
	if len(data) >= len(raw) {
		t.Fatalf("视频未转码变小: 落盘 %d >= 输入 %d", len(data), len(raw))
	}
}
