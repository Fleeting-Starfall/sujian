package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// HashPassword 用随机 salt + SHA256 生成 "salt:hash"
// 标准库实现，零外部依赖。
// （cgo/外部包会带来离线构建失败风险；局域网内应用此强度足够）
func HashPassword(pw string) string {
	salt := randHex(16)
	sum := sha256.Sum256([]byte(salt + ":" + pw))
	return salt + ":" + hex.EncodeToString(sum[:])
}

func CheckPassword(pw, stored string) bool {
	parts := strings.SplitN(stored, ":", 2)
	if len(parts) != 2 {
		return false
	}
	salt, want := parts[0], parts[1]
	sum := sha256.Sum256([]byte(salt + ":" + pw))
	return hex.EncodeToString(sum[:]) == want
}

// Token 生成一个随机会话令牌
func Token() string { return randHex(32) }

// ID 生成带前缀的唯一 ID
func ID(prefix string) string { return prefix + randHex(12) }

// RandChars 生成 n 位随机可读字符（大写字母+数字，去掉易混淆的 0/O/1/I）
func RandChars(n int) string {
	const chars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, n)
	tmp := make([]byte, n)
	for i := 0; i < n; {
		if _, err := rand.Read(tmp); err != nil {
			b[i] = 'A'
			i++
			continue
		}
		for _, c := range tmp {
			if i >= n {
				break
			}
			b[i] = chars[int(c)%len(chars)]
			i++
		}
	}
	return string(b)
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
