package handler

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
)

const (
	optMaxDim      = 1600 // 上传图片长边最大像素，超出则等比缩小
	optJPEGQuality = 82   // JPEG 质量（照片足够清晰且体积小）
)

// optimizeImage 把上传的图片在内存中缩放并转成更省空间的格式，从源头控制存储占用。
//
// 规则：
//   - 不透明图片（绝大多数照片/截图）→ 转 JPEG(q82)，扩展名 .jpg
//   - 含透明通道（贴纸、带透明底的 PNG）→ 保留 PNG，扩展名 .png
//   - GIF（保留动图）、WebP（标准库无法解码）等 → 返回 ok=false，由调用方原样保存原始字节
//
// 返回 ok=false 时 out 为空，调用方应回退为保存原始上传字节，绝不丢数据。
func optimizeImage(r io.Reader) (out []byte, ext string, ok bool) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, "", false
	}
	img, format, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, "", false // 标准库不支持的格式（如 WebP）
	}
	if format == "gif" {
		return nil, "", false // 保留动画 GIF，不优化
	}

	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w > optMaxDim || h > optMaxDim {
		scale := float64(optMaxDim) / float64(maxInt(w, h))
		nw := int(float64(w)*scale + 0.5)
		nh := int(float64(h)*scale + 0.5)
		if nw < 1 {
			nw = 1
		}
		if nh < 1 {
			nh = 1
		}
		img = downscale(img, nw, nh)
	}

	var buf bytes.Buffer
	if hasTransparency(img) {
		if err := png.Encode(&buf, img); err != nil {
			return nil, "", false
		}
		return buf.Bytes(), ".png", true
	}
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: optJPEGQuality}); err != nil {
		return nil, "", false
	}
	return buf.Bytes(), ".jpg", true
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// hasTransparency 采样扫描像素，判断图像是否含透明（alpha < 255）区域。
func hasTransparency(img image.Image) bool {
	b := img.Bounds()
	stepX, stepY := 1, 1
	if b.Dx() > 256 {
		stepX = b.Dx() / 256
	}
	if b.Dy() > 256 {
		stepY = b.Dy() / 256
	}
	for y := b.Min.Y; y < b.Max.Y; y += stepY {
		for x := b.Min.X; x < b.Max.X; x += stepX {
			_, _, _, a := img.At(x, y).RGBA()
			if a < 0xffff {
				return true
			}
		}
	}
	return false
}

// downscale 面积平均（box）下采样，纯 Go 无依赖，适合缩小尺寸。
// 对 dst 每个像素，取其在 src 中对应矩形框内像素的平均值。
func downscale(src image.Image, dw, dh int) *image.RGBA {
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < dh; y++ {
		y0 := int(float64(y) * float64(sh) / float64(dh))
		y1 := int(float64(y+1) * float64(sh) / float64(dh))
		if y1 <= y0 {
			y1 = y0 + 1
		}
		for x := 0; x < dw; x++ {
			x0 := int(float64(x) * float64(sw) / float64(dw))
			x1 := int(float64(x+1) * float64(sw) / float64(dw))
			if x1 <= x0 {
				x1 = x0 + 1
			}
			var sr, sg, sb, sa uint64
			n := 0
			for sy := y0; sy < y1; sy++ {
				for sx := x0; sx < x1; sx++ {
					pr, pg, pb, pa := src.At(b.Min.X+sx, b.Min.Y+sy).RGBA()
					sr += uint64(pr)
					sg += uint64(pg)
					sb += uint64(pb)
					sa += uint64(pa)
					n++
				}
			}
			inv := uint64(n)
			dst.SetRGBA(x, y, color.RGBA{
				R: uint8(sr / inv >> 8),
				G: uint8(sg / inv >> 8),
				B: uint8(sb / inv >> 8),
				A: uint8(sa / inv >> 8),
			})
		}
	}
	return dst
}
