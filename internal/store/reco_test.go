package store

import (
	"os"
	"path/filepath"
	"testing"

	"sujian/internal/model"
)

// TestColdStartInterests 注册兴趣标签作为弱画像进入推荐。
func TestColdStartInterests(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	s.SeedAdmin("admin", "admin123")

	// 注册时自选兴趣「摄影」
	u, err := s.Register("alice", "Alice", "pass123", []string{"摄影"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetUserStatus(u.ID, "active"); err != nil {
		t.Fatal(err)
	}
	// 由 admin 发布，避免作者标签污染画像
	var admin *model.User
	for _, uu := range s.Users {
		if uu.Role == "admin" {
			admin = uu
			break
		}
	}
	if admin == nil {
		t.Fatal("未找到 admin")
	}
	s.AddNote(admin, "摄1", "c", "image", nil, "", []string{"摄影"}, "推荐", "published", "green")
	s.AddNote(admin, "编1", "c", "image", nil, "", []string{"编程"}, "推荐", "published", "green")

	// 画像含「摄影」且权重 0.5
	prof := s.interestTags(u.ID)
	if prof["摄影"] != 0.5 {
		t.Fatalf("冷启动弱画像未生效：prof[摄影]=%v（期望 0.5）", prof["摄影"])
	}
	// 推荐里应包含摄影笔记（被弱画像命中）
	rec := s.RecommendForUser(u.ID, 10)
	foundPhoto := false
	for _, n := range rec {
		for _, tag := range n.Tags {
			if tag == "摄影" {
				foundPhoto = true
			}
		}
	}
	if !foundPhoto {
		t.Fatal("冷启动：注册兴趣「摄影」对应的笔记未进入推荐")
	}
	t.Logf("✓ 冷启动弱画像生效：prof[摄影]=0.5，摄影笔记进入推荐")
}

// TestRecoConfigLoad reco_config.json 读取与默认值兜底。
func TestRecoConfigLoad(t *testing.T) {
	// 1) 无配置文件时用默认值
	dir1 := t.TempDir()
	s1 := New(dir1)
	def := DefaultRecoConfig()
	if s1.RecoConfig != def {
		t.Fatalf("默认配置不匹配：%+v vs %+v", s1.RecoConfig, def)
	}

	// 2) 存在配置文件时读取覆盖项
	dir2 := t.TempDir()
	cfgJSON := `{"w_cf": 9.9, "cold_max_per_tag": 2}`
	if err := os.WriteFile(filepath.Join(dir2, "reco_config.json"), []byte(cfgJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	s2 := New(dir2)
	if s2.RecoConfig.WCf != 9.9 {
		t.Fatalf("w_cf 未读取：%v（期望 9.9）", s2.RecoConfig.WCf)
	}
	if s2.RecoConfig.ColdMaxPerTag != 2 {
		t.Fatalf("cold_max_per_tag 未读取：%v（期望 2）", s2.RecoConfig.ColdMaxPerTag)
	}
	// 未覆盖项保持默认
	if s2.RecoConfig.WTag != def.WTag {
		t.Fatalf("未覆盖项 w_tag 未保持默认：%v", s2.RecoConfig.WTag)
	}
	t.Logf("✓ 推荐配置读取生效：w_cf=9.9 cold_max_per_tag=2，其余保持默认")
}
