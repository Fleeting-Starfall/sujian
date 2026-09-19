package store

import (
	"sync"
	"testing"
	"time"

	"sujian/internal/auth"
	"sujian/internal/model"
)

// TestConcurrentAccess 高并发读写，验证无 data race。
func TestConcurrentAccess(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	s.SeedAdmin("admin", "admin123")

	var uids []string
	for i := 0; i < 5; i++ {
		u, err := s.Register("user"+string(rune('a'+i)), "昵称"+string(rune('a'+i)), "pw", nil)
		if err != nil {
			t.Fatal(err)
		}
		uids = append(uids, u.ID)
	}
	for _, id := range uids {
		if err := s.SetUserStatus(id, "active"); err != nil {
			t.Fatal(err)
		}
	}

	var nids []string
	for i := 0; i < 5; i++ {
		u := s.UserByID(uids[i%len(uids)])
		n := s.AddNote(u, "标题", "内容", "image", nil, "", []string{"tag"}, "推荐", "published", "green")
		nids = append(nids, n.ID)
	}

	done := make(chan struct{})
	var wg sync.WaitGroup

	reader := func() {
		defer wg.Done()
		for {
			select {
			case <-done:
				return
			default:
			}
			uid := uids[len(uids)-1]
			u := s.UserByID(uid)
			_ = u.Nickname
			_ = u.AgeVerified
			for _, n := range s.NotesList("", "", nil, "hot") {
				_ = n.Title
				_ = n.Views
			}
			if n := s.NoteByID(nids[0]); n != nil {
				_ = n.Content
			}
			for _, c := range s.CommentsByNote(nids[0], "hot") {
				_ = c.Content
			}
			_ = s.ConversationsOf(uid)
			_ = s.NotificationsOf(uid)
			_ = s.AllReports()
			_ = s.FriendRequestsTo(uid)
			_ = s.FriendRequestsFrom(uid)
			_ = s.FavNotesOf(uid)
			_ = s.NotesByTag("tag")
			_ = s.RelatedNotes(nids[0], "推荐", 10)
		}
	}

	writer := func(seed int) {
		defer wg.Done()
		i := 0
		for {
			select {
			case <-done:
				return
			default:
			}
			uid := uids[len(uids)-1]
			_ = s.UpdateProfile(uid, "昵称X", nil, "")
			u := s.UserByID(uids[(i+seed)%len(uids)])
			s.AddNote(u, "t", "c", "image", nil, "", nil, "推荐", "published", "green")
			s.AddComment(nids[0], uid, "name", "comment", "", "")
			s.ToggleLike(nids[0], uid)
			s.AddMessage(uid, "from", uids[0], "hi", "", "", "", 0)
			s.AddNotification(&model.Notification{
				ID: auth.ID("nt_"), UserID: uid, ActorID: uids[0],
				Type: "like", CreatedAt: time.Now().Unix(),
			})
			i++
		}
	}

	for i := 0; i < 8; i++ {
		wg.Add(1)
		go reader()
	}
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go writer(i)
	}

	time.Sleep(2 * time.Second)
	close(done)
	wg.Wait()
}
