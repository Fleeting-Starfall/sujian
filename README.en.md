[**中文**](README.md) | **English**

# Sujian (素见)

A Xiaohongshu-style content community accessible over your local network. It supports image and video notes, publishing / likes / favorites / comments / follows / search. New registrations require admin approval before they can log in. The UI is built on the **Yuanli Design System** (`colors_and_type.css` + `components.css`), and the server is implemented with the Go standard library only — **a single binary, zero external dependencies, zero database**, just double-click to run.

> 📖 **Full user manual** (usage only; covers registration, publishing, social features, admin console and FAQ): [`docs/usage.md`](docs/usage.md) (legacy version kept: [`docs/user-guide.md`](docs/user-guide.md))

---

## Quick Start

After cloning from GitHub or downloading the source, enter the project root (`sujian`):

```bash
git clone <repo-url> sujian
cd sujian
```

The project files are placed directly in the current folder (sujian). Then run:

```bash
./start.sh              # Start: auto-build + run in the background
./start.sh stop         # Stop
./start.sh restart      # Restart
./start.sh status       # Check status
```

After startup, the console prints the access address:

```
Sujian started ✓
  Local:    http://localhost:8099
  LAN:      http://192.168.x.x:8099      ← open this on phones/PCs on the same WiFi/Ethernet
  Log:      data/sujian.log  (stop: ./start.sh stop)
```

- The service runs **in the background** and keeps running after the terminal is closed; logs are written to `data/sujian.log`
- If the port is taken: `PORT=9000 ./start.sh`
- The default admin password can be changed in `AdminPass` at the top of `main.go`.

### Want to see it in action? Generate demo data with one command

```bash
go run ./cmd/seed      # skipped if data already exists
go run ./cmd/seed -f   # wipe data/ and uploads/, then rebuild
```

Creates 3 demo users (password `demo123`: 苏小厨 / 阿梨 / 数码君) + 7 notes across channels (auto-generated gradient covers, including green/yellow/red level demos) + comments / likes / favorites / follows, so the feed has content immediately.

---

## File Structure (how it is organized)

```
sujian/                      # Project root (this folder, no nested subdirectory)
├── main.go                  # Entry point: config, startup, prints LAN address (port/admin/upload limits edited here)
├── start.sh                 # One-command start/stop script (start background / stop / restart / status)
├── cmd/seed/                # Seed data: go run ./cmd/seed [-f] generates demo content
├── scripts/test.py          # End-to-end regression test (180 assertions, pure stdlib HTTP script)
├── scripts/bench.js         # Load test script (zero dependencies, CONC/TOTAL/BASE adjustable)
├── go.mod                   # Module definition (standard library only, no third-party dependencies)
├── internal/
│   ├── model/model.go       # Data models: User (with 8-digit Uid) / Note / Comment / Notification / Draft / FriendRequest
│   ├── auth/auth.go         # Password hashing (salt+SHA256) and random ID/Token/Uid generation
│   ├── store/store.go       # Data layer: in-memory index + JSON file persistence + sessions + all read/write methods
│   └── handler/             # HTTP handlers
│       ├── server.go        # Server struct, auth middleware (requireLogin/Active/Admin), page gates
│       ├── routes.go        # All route registration (API + pages)
│       ├── auth.go          # Register / login / logout / current user
│       ├── content.go       # Feed / notes / likes / favorites / comments / follows / users
│       ├── friend.go        # Friends system (request/accept/reject/list/delete by Uid)
│       ├── upload.go        # Image/video upload (per-user directories)
│       ├── image_opt.go     # Image auto-optimization (resize + convert to JPEG/PNG, with magic bytes validation)
│       ├── notify.go        # Notifications (query/read)
│       ├── report.go        # Reports (notes/comments) + user search
│       ├── message.go       # In-site messages (conversations/chat/unread)
│       ├── draft.go         # Draft box
│       └── admin.go         # Admin console API (stats/review/users/notes/comments)
├── data/                    # ★ Runtime data (JSON persistence + PID/log, auto-generated)
│   ├── sujian.pid           #   Running PID (managed by start.sh)
│   ├── sujian.log           #   Runtime log (written when started via start.sh)
│   ├── users.json           #   Users (with review status pending/active/rejected)
│   ├── notes.json           #   Notes
│   ├── comments.json        #   Comments (with nested replies parentId/replyTo)
│   ├── likes.json           #   Likes (noteID -> list of user IDs)
│   ├── favorites.json       #   Favorites
│   ├── follows.json         #   Follow relationships
│   ├── notifications.json   #   Notifications
│   ├── drafts.json          #   Drafts
│   ├── friends.json         #   Friend relationships (two-way confirmed)
│   ├── friend_requests.json #   Friend requests
│   ├── reports.json         #   Reports (notes/comments)
│   ├── comment_likes.json   #   Comment likes (commentID -> list of user IDs)
│   ├── view_history.json    #   View history (userID -> noteID -> last viewed time)
│   └── messages.json        #   In-site messages
├── uploads/                 # ★ User-uploaded files, stored per user
│   └── <userID>/            #   uploads/userID/image-or-video
│       └── f_xxxx.jpg|mp4
└── public/                  # ★ Frontend pages (HTML + CSS + JS)
    ├── index.html           #   Home: channel tabs + masonry feed (featured/latest)
    ├── login.html           #   Login
    ├── register.html        #   Register (pending review after submission)
    ├── note.html            #   Note detail: image/video + like/favorite + nested comments + related notes + share
    ├── publish.html         #   Publish/edit note + draft recovery (image/video switch + multi-file upload)
    ├── profile.html         #   Profile page (edit profile + notes/favorites tabs + stats)
    ├── search.html          #   Search (title/tag/author)
    ├── notifications.html   #   Notification center (unread badge)
    ├── followers.html       #   Following/follower lists
    ├── tag.html             #   Tag aggregation page
    ├── drafts.html          #   Draft box
    ├── messages.html        #   Messages (conversation list + chat window)
    ├── admin/index.html     #   Admin console (Yuanli Sidenav skeleton)
    ├── css/                 #   Two CSS files from the Yuanli Design System (copied as-is) + app.css (layout)
    └── js/                  #   Frontend scripts
        ├── app.js           #   Common frontend scripts (nav/bell/cards/helpers)
        └── masonry.js       #   JS true masonry (shortest-column algorithm + responsive 4/2/1 columns + auto re-layout)
```

**Core conventions**

- **Different users, different folders**: uploaded images/videos go to `uploads/<userID>/`; the database stores only relative paths, and `/uploads/` is served statically by the server.
- **Data persistence**: every write is an atomic JSON save (write `.tmp` first, then rename); no data loss on restart.
- **Sessions**: after login a random Token is issued and stored in a Cookie (HttpOnly); sessions live in server memory and you must log in again after a restart.

---

## Feature List

| Feature                | Description                                                                                                                                                                                                                                                        |
| ---------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Login/Register         | New registrations are set to "pending review"; the user can only log in after admin approval (username 2–20 characters)                                                                                                                                             |
| Home feed              | Featured/Following tabs + category channels + **JS true masonry cards** (shortest-column algorithm, auto re-layout on image load/window resize, responsive 4/2/1 columns) + Featured (heat)/Latest sorting + video hover mute preview + **"For You" personalized ranking** |
| **Smart recommendations** | Inspired by Xiaohongshu's recommendation approach: the home "For You" feed builds an **interest-tag profile** from your likes/favorites/comments/posts/follows, ranking by tag overlap + interaction heat (CES = like×1 + favorite×1 + comment×4) + time decay + diversity spread (**cold start** falls back to heat + freshness when there is no history); the note detail "related notes" finds similar notes by **tag overlap** (same-channel weighting + spread, to avoid filter bubbles) |
| Publish notes          | Image (multi-image) or video; the first frame is auto-captured as the cover for videos; supports a **draft box** (save/restore/delete drafts); published notes enter **content review** and only become visible to others after being graded                                                    |
| Note detail            | Large image (click to zoom)/video playback, **view count**, likes, favorites, comments (**nested replies**, **comment likes**, deletable by author/admin, **latest/hottest sorting**), follow author, **related notes** (matched by tags), **share copy link**, author can **pin/edit/delete**                                                      |
| **Content levels (red/yellow/green)** | When a note passes review, the admin assigns a level: 🟢green = pushed to home + free browsing · 🟡yellow = normal content (shown on home) · 🔴red = **sensitive content** (shown on home/search/profile with a blurred cover + confirmation before viewing, invisible when logged out); all three levels can appear on the home feed; new notes default to pending review, admins can change the level or reject |
| **Sensitive content confirmation** | Red-level content is shown normally on home/search/profile (blurred cover + "Sensitive" marker); opening the detail page requires clicking "I understand, continue viewing" to unblur; not visible to logged-out users                                                                           |
| Notifications          | Top-bar bell + unread badge; you are notified when you are liked/favorited/commented/replied/followed, and **authors are also notified when their notes are approved or rejected**; supports mark-one-read and mark-all-read; related notifications are cleaned up when notes/comments are deleted     |
| Profile page           | Profile (editable avatar/nickname/bio), **8-digit user ID** (shown/copyable), note count/likes/following/followers stats, "Notes/Favorites/**History**" tabs, following/follower list pages, **pinned notes shown first**, **official verification badge**, **self-service password change** (all sessions invalidated after change), **view your own login password** (show/hide) |
| **View history**       | Logged-in users are recorded in "My History" when opening a note detail (one entry per note, timestamp updated, max 200 entries per user with oldest auto-evicted); the profile "History" tab lists entries by most recent first, with **single removal** and **clear all**; deleted/taken-down notes no longer appear                                                                 |
| **Friends system**     | Each user has a random 8-digit user ID (uppercase letters + digits, unique); **add friends by user ID** (request → accept/reject → friend list → delete); friend requests/acceptances send notifications                                                                   |
| **Messages/in-site mail** | Top-bar envelope icon + unread badge; one-on-one conversations (conversation list/chat window/5s polling/Enter to send/auto read); **send any file** (images/videos/documents/archives etc., ≤300MB per file: images shown as thumbnails, videos playable inline, other files as downloadable cards); "Message" button on profiles; text messages up to 500 characters |
| Tag aggregation        | Click any #tag to view all notes under that tag                                                                                                                                                                                                                    |
| Search                 | By title / tag / author nickname; **user search** (nickname/username/8-digit user ID, results show user cards)                                                                                                                                                  |
| **Report system**      | Both notes and comments can be **reported** (with a reason); the admin console "Report Management" shows details, with **delete reported content** or **dismiss**; duplicate reports on the same content are flagged                                                                          |
| Admin console `/admin` | Overview dashboard, registration review (approve/reject), **content review (green/yellow/red grading + reject + re-grade)**, **bulk user creation (with optional official verification)**, user management (**reset password**/**one-click reset to 000000**/**view plaintext password**/official verification toggle/disable), note take-down (with **auto-cleanup after 30 days** and **permanent delete**), comment deletion, **report management** |
| **Content governance** | When an admin **disables a user**, their published notes disappear from home/search/tags/profile/direct links automatically (admins can still view them) — "banned account means content taken down". Taken-down notes keep a 30-day grace period during which admins can restore or permanently delete them; after 30 days they are physically cleaned up automatically (including likes/favs/comments/notifs and media files). |
| **Scheduled account lock** | Admins can **temporarily lock** any regular user (enter hours, default 24, shortcuts 1/24/168/720): while locked the user **cannot log in** (login shows "account locked, retry after xx"), existing sessions are invalidated immediately, and all logged-in operations are rejected; **published content stays visible and is not taken down**. Unlock is **automatic on expiry** (judged in real time, no scheduled tasks). Admin accounts cannot be locked. |

> When a note is deleted/taken down, its uploaded images and videos are cleaned up as well.  
> All icons come from the **Yuanli Design System** `assets/icons/` inline SVGs (no emoji, no third-party icon library).

---

## Admin & Review Flow

1. On first start, the admin account `admin / admin123` is created automatically (changeable in `main.go`).
2. A new user registers → status `pending` → "waiting for admin review".
3. The admin logs in and opens `/admin` → "User Review" → approve/reject.
4. After approval the user can log in and publish notes.
5. Newly published notes enter **pending review**; the admin grades them in `/admin` → "Content Review" using the red/yellow/green levels:
   - **Green**: pushed to home + free browsing
   - **Yellow**: normal content, shown on home
   - **Red**: sensitive content, shown on home/search/profile with a blurred cover; opening the detail requires confirmation; invisible when logged out
   - Can also **reject** directly (the author gets a notification); already-approved notes can be **re-graded** or **taken down** at any time
6. **Bulk user creation**: `/admin` → "User Management" → one `username password nickname` per line; check "Official Verification" to set the official badge uniformly (created as active/usable immediately).

---

## Upload Limits (changeable in main.go)

- Images: jpg / jpeg / png / gif / webp, ≤10MB each; **auto-optimized on upload**: downscaled proportionally when the long edge > 1600px, opaque images converted to **JPEG (q82)** (transparent images keep PNG), WebP/GIF (animated) kept as-is — storage usage is controlled at the source
- Videos: mp4 / webm / mov / m4v, ≤200MB each; **auto-transcoded and compressed when ffmpeg is available** (unified to 720p mp4 H.264+AAC, crf28, measured ~80%+ size reduction; only files ≤30MB are transcoded to avoid blocking publishing; saved as-is without ffmpeg or on transcode failure)
- **Security hardening**: image reads have a memory cap (prevents oversized request bodies from exhausting memory); before falling back to saving as-is, the file header magic bytes are validated (blocks HTML/scripts disguised as images)

---

## Notes

- Passwords are stored with salt+SHA256 (standard library implementation, compiles offline in any environment; sufficient for LAN usage).
- Pure Go standard library (`net/http` + `encoding/json`), no cgo, no third-party packages; `go build` produces a portable binary.
- **Automated tests**: `scripts/test.py` (built-in regression script, no longer relies on /tmp) covers accounts/review/content/notifications/friends/reports/messages/content governance end-to-end, currently **ALL PASS**. Run: clear `data/`, start the server, then `python3 scripts/test.py`.
- **Unit/concurrency tests**: `go test ./...` (in-process `httptest`, drives the real handler + real store, no network); `go test -race ./...` additionally checks concurrency safety. Key cases: `TestFunctionalFlows` (register pending → admin approves → post note → review → comment/like → DM without password leak → report → banned content auto-removed), `TestConcurrentHandlerLoad` (no races under high-concurrency reads/writes with `-race`), store-level `TestConcurrentAccess`, `image_opt_test.go` (image optimization + upload security: `TestUploadOptimizesImage` large image auto-compressed, `TestUploadRejectsOversizeImage` oversized fake image rejected, `TestUploadRejectsDisguisedFile` HTML disguised as image rejected). All tests ALL PASS; gofmt/go vet clean.
- **Data reliability**: view counts are persisted with throttling (every 20 changes or every 30 seconds); on `SIGINT/SIGTERM` (Ctrl+C / `./start.sh stop`) everything is persisted before exit, no data loss.
- **Taken-down note lifecycle**: after take-down (soft delete) there is a 30-day grace period during which the note can be restored or permanently deleted in the admin console; after expiry the admin can clean it up with one click (`Clear Old Taken-down` button, default 30 days, customizable). Permanent deletion also cleans up likes/favs/comments/notifications and media files; it cannot be undone.
- **Masonry**: home/search/tag/profile use **JS true masonry** (`public/js/masonry.js`, design in [`docs/masonry-plan.md`](docs/masonry-plan.md), implemented): cards fall into the current **shortest column** in reverse chronological order; image `onload`/`error` and window `resize` trigger auto re-layout; responsive **4/2/1 columns** (≥900px / 480–900px / <480px); "load more" also targets the shortest column, solving the gaps, uncontrollable landing positions and deletion holes of the original CSS multi-column layout; zero third-party dependencies.
- **Storage optimization**: uploaded images are auto-compressed (see "Upload Limits"); **videos are auto-transcoded and compressed when ffmpeg is available** (720p H.264/AAC, measured 80%+ smaller); `data/*.json` is persisted as **compact JSON** (no indentation), ~16–30% smaller than pretty-printed, while staying atomic (temp file + rename); **on-demand persistence** — a write only serializes the changed data files (e.g. a like writes only likes.json), avoiding full rewrites of 13 files per small operation and keeping writes low-latency as data grows (full fallback save on process exit, no data loss).
- **Performance reference** (measured: 1000 notes, 10-core Mac, 500 concurrency): read endpoints 0.6–1.5ms per request; feed ~6.6k QPS, note detail ~10k QPS, search ~10k QPS, static pages ~37.5k QPS, posting a comment ~17.9k QPS; simulated 500 devices opening the app at once (1500 requests) completed in 0.2s with zero failures; memory peaked at ~120MB under 500 concurrency (20–30MB idle). Built-in load test script: `scripts/bench.js` (`CONC=500 node scripts/bench.js`; write paths need `NOTE_ID=n_xxx`).

## Concurrency Safety (implementation notes)

- The data layer `Store` is protected by `sync.RWMutex`; **every read query that returns internal objects/slices makes a copy while holding the lock**, so handlers hold independent copies outside the lock, eliminating read-write data races.
- Write operations (new notes/comments/messages/notifications etc.) complete **in-memory mutation + `saveLocked()` persistence under the write lock** before releasing it, preventing old snapshots from overwriting newer data when requests interleave.
- Persistence uses a temp file + atomic `rename`, so a crash never leaves a half-written data file.
