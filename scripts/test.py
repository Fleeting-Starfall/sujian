#!/usr/bin/env python3
# 素见 全链路回归测试
# 用法：先启动服务（PORT=8099），再运行本脚本；需全新 data/（先清空再启动）
# 依赖：仅标准库。通过 HTTP 断言全部 API 行为。
import base64, json, sys, urllib.request, urllib.error, urllib.parse, http.cookiejar

BASE = "http://127.0.0.1:8099"
cj = http.cookiejar.CookieJar()
opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(cj))

def req(path, method="GET", body=None, multipart=None):
    path = urllib.parse.quote(path, safe="/?=&%")
    url = BASE + path
    data, headers = None, {}
    if multipart:
        boundary = "----sujianboundary"
        parts = []
        for name, (filename, content, ctype) in multipart.items():
            parts.append(("--%s\r\nContent-Disposition: form-data; name=\"%s\"; filename=\"%s\"\r\nContent-Type: %s\r\n\r\n" % (boundary, name, filename, ctype)).encode())
            parts.append(content)
            parts.append(b"\r\n")
        parts.append(("--%s--\r\n" % boundary).encode())
        data = b"".join(parts)
        headers["Content-Type"] = "multipart/form-data; boundary=" + boundary
    elif body is not None:
        data = json.dumps(body).encode()
        headers["Content-Type"] = "application/json"
    r = urllib.request.Request(url, data=data, headers=headers, method=method)
    try:
        with opener.open(r) as resp:
            return resp.status, json.loads(resp.read().decode() or "{}")
    except urllib.error.HTTPError as e:
        return e.code, json.loads(e.read().decode() or "{}")

def raw_get(path):
    r = urllib.request.Request(BASE + path, method="GET")
    try:
        with opener.open(r) as resp:
            return resp.status, resp.read()
    except urllib.error.HTTPError as e:
        return e.code, b""

def check(name, cond, extra=""):
    print(("PASS " if cond else "FAIL ") + name + ("" if cond else "  -> " + str(extra)))
    if not cond:
        sys.exit(1)

def login(u, p):
    return req("/api/login", "POST", {"username": u, "password": p})[0] == 200

def switch(u, p):
    return login(u, p)

# ============ 账号体系 ============
s, r = req("/api/register", "POST", {"username": "a", "nickname": "x", "password": "pass123"})
check("用户名过短被拒", s == 400, r)
s, r = req("/api/register", "POST", {"username": "alice", "nickname": "爱丽丝", "password": "pass123"})
check("注册提交=pending", s == 200 and r["user"]["status"] == "pending", r)
uid = r["user"]["id"]
check("用户号为8位字母数字", len(r["user"].get("uid", "")) == 8, r["user"].get("uid"))
check("待审核不能登录", req("/api/login", "POST", {"username": "alice", "password": "pass123"})[0] == 401)
check("管理员登录", login("admin", "admin123"))
s, r = req("/api/admin/pending")
check("待审核列表含 alice", any(u["id"] == uid for u in r), r)
check("通过审核", req("/api/admin/users/%s/approve" % uid, "POST", {})[0] == 200)
check("alice 登录成功", login("alice", "pass123"))

# ============ 资料 / 上传 ============
s, r = req("/api/me/update", "POST", {"nickname": "爱丽丝酱", "bio": "你好呀～"})
check("编辑资料", s == 200 and r["nickname"] == "爱丽丝酱", r)
s, r = req("/api/me/update", "POST", {"bio": ""})
check("清空简介", s == 200 and r["bio"] == "", r)
png = base64.b64decode("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==")
s, r = req("/api/upload", "POST", multipart={"file": ("a.png", png, "image/png")})
check("上传图片", s == 200, r)
media = [r["path"]]
check("切到 admin", login("admin", "admin123"))
s, r = req("/api/upload", "POST", multipart={"file": ("admin.png", png, "image/png")})
check("admin 上传图片", s == 200, r)
admin_media = [r["path"]]
check("切到 alice", login("alice", "pass123"))
s, r = req("/api/notes", "POST", {"title": "越权媒体", "content": "x", "mediaType": "image", "media": admin_media, "tags": [], "category": "生活"})
check("引用他人媒体被拒", s == 400, r)
s, r = req("/api/notes", "POST", {"title": "越权媒体", "content": "x", "mediaType": "image", "media": media, "cover": admin_media[0], "tags": [], "category": "生活"})
check("引用他人封面被拒", s == 400, r)
s, r = req("/api/notes", "POST", {"title": "第一篇笔记", "content": "你好素见", "mediaType": "image", "media": media, "tags": ["测试", "教程"], "category": "美食"})
check("发布笔记", s == 200, r)
nid = r["note"]["id"]
check("新笔记状态=pending(待审核)", r["note"].get("status") == "pending", r["note"].get("status"))
check("切到 alice", login("alice", "pass123"))
s, r = req("/api/notes/%s" % nid)
check("作者可见待审核详情(pending=true)", s == 200 and r.get("pending") is True, r)
check("切到 admin", login("admin", "admin123"))
s, r = req("/api/notes/%s" % nid)
check("管理员可预览待审核笔记(pending=true)", s == 200 and r.get("pending") is True, r)
s, r = req("/api/feed")
check("待审核笔记不进首页", s == 200 and all(n["id"] != nid for n in r["items"]), [n["id"] for n in r["items"]])
s, r = req("/api/admin/notes/%s/review" % nid, "POST", {"level": "green"})
check("审核通过(绿)", s == 200, r)
s, r = req("/api/feed")
check("绿标笔记进首页", s == 200 and any(n["id"] == nid for n in r["items"]), [n["id"] for n in r["items"]])
check("切到 alice", login("alice", "pass123"))
s, r = req("/api/notifications")
check("作者收到审核通过通知", s == 200 and any(n["type"] == "approve" for n in r["items"]), r)

# ============ 通知 ============
check("切到 admin", login("admin", "admin123"))
s, r = req("/api/notes/%s/like" % nid, "POST", {})
check("admin 点赞", s == 200 and r["liked"], r)
s, r = req("/api/notes/%s/comment" % nid, "POST", {"content": "管理员来顶一下"})
check("admin 评论", s == 200, r)
cmt_id = r["id"]
check("切到 alice", login("alice", "pass123"))
s, r = req("/api/notifications")
check("alice 收到通知(like+comment)", s == 200 and r["unread"] >= 2, r)
s, r = req("/api/notifications/read", "POST", {})
s, r = req("/api/notifications")
check("全部已读 unread==0", s == 200 and r["unread"] == 0, r)

# ============ 评论回复 / 排序 ============
s, r = req("/api/notes/%s/comment" % nid, "POST", {"content": "谢谢管理员", "parentId": cmt_id})
check("alice 回复 admin 评论", s == 200 and r.get("parentId") == cmt_id and r.get("replyTo") == "管理员", r)
s, r = req("/api/notes/%s?order=hot" % nid)
check("最热排序顶层为被回复评论", s == 200 and r["comments"][0]["id"] == cmt_id, r)
check("切到 admin", login("admin", "admin123"))
s, r = req("/api/notifications")
check("admin 收到 reply 通知", s == 200 and any(n["type"] == "reply" for n in r["items"]), r)

# ============ 关注 / 粉丝 ============
check("admin 关注 alice", req("/api/user/%s/follow" % uid, "POST", {})[1].get("following") is True)
s, r = req("/api/user/%s/followers" % uid)
check("alice 粉丝列表含 admin", s == 200 and any(u["user"]["username"] == "admin" for u in r), r)
s, r = req("/api/user/%s/following" % uid)
check("alice 关注列表返回", s == 200, r)

# ============ 标签 / 相关推荐 / 编辑 / 置顶 ============
s, r = req("/api/tag/%s/notes" % "测试")
check("标签聚合含笔记", s == 200 and any(n["id"] == nid for n in r["items"]), r)
s, r = req("/api/notes/%s" % nid)
check("详情含 related 字段", s == 200 and "related" in r and "following" in r, r)
check("切到 alice", login("alice", "pass123"))
# 评论需在"编辑"前进行：编辑后笔记回到待审核（pending），待审核笔记不可评论
s, r = req("/api/notes/%s/comment" % nid, "POST", {"content": "临时评论"})
check("alice 再发评论", s == 200, r)
tmp_cid = r["id"]
s, r = req("/api/notes/%s" % nid, "PUT", {"title": "第一篇笔记(已改)", "content": "内容更新", "tags": ["测试"], "category": "美食"})
check("编辑笔记", s == 200 and r["note"]["title"] == "第一篇笔记(已改)", r)
s, r = req("/api/notes/%s/pin" % nid, "POST", {"pinned": True})
check("作者置顶笔记", s == 200 and r["pinned"] is True, r)
s, r = req("/api/user/%s/notes" % uid)
check("个人主页置顶优先", s == 200 and len(r) > 0 and r[0]["id"] == nid and r[0]["pinned"] is True, r)
check("切到 admin", login("admin", "admin123"))
s, r = req("/api/notes/%s/pin" % nid, "POST", {"pinned": True})
check("非作者置顶被拒", s == 400, r)
s, r = req("/api/notes/%s" % nid, "PUT", {"title": "越权修改"})
check("越权编辑被拒", s != 200, r)

# ============ 浏览量 / 评论删除权限 ============
check("切到 alice", login("alice", "pass123"))
s, r = req("/api/notes/%s" % nid)
# 注：作者本人浏览不计入浏览量；此处全流程仅管理员在详情页浏览过一次（admin 于"标签聚合"段 GET 详情）。
# 故期望 views >= 1 即视为"浏览量递增"生效。
check("浏览量递增", s == 200 and r["note"]["views"] >= 1, r)
s, r = req("/api/comments/%s" % cmt_id, "DELETE")
check("非作者删除他人评论被拒", s != 200, r)
s, r = req("/api/comments/%s" % tmp_cid, "DELETE")
check("作者删除自己评论", s == 200, r)
check("切到 admin", login("admin", "admin123"))
s, r = req("/api/comments/%s" % cmt_id, "DELETE")
check("管理员删除任意评论", s == 200, r)

# ============ 草稿 ============
check("切到 alice", login("alice", "pass123"))
s, r = req("/api/drafts", "POST", {"title": "未写完的笔记", "content": "明天继续…", "mediaType": "image", "media": media, "tags": ["草稿"], "category": "生活"})
check("保存草稿", s == 200 and r.get("id"), r)
did = r["id"]
s, r = req("/api/drafts", "POST", {"title": "越权媒体草稿", "content": "x", "mediaType": "image", "media": admin_media, "tags": [], "category": "生活"})
check("草稿引用他人媒体被拒", s == 400, r)
check("切到 admin", login("admin", "admin123"))
s, r = req("/api/drafts", "POST", {"id": did, "title": "越权覆盖草稿", "content": "x", "mediaType": "image", "media": media, "tags": [], "category": "生活"})
check("越权覆盖他人草稿被拒", s == 400, r)
check("切到 alice", login("alice", "pass123"))
s, r = req("/api/drafts")
check("草稿列表含", s == 200 and any(d["id"] == did for d in r), r)
s, r = req("/api/drafts", "POST", {"id": did, "title": "未写完的笔记v2", "content": "改一下", "mediaType": "image", "media": media, "tags": [], "category": "生活"})
check("更新草稿", s == 200 and r["title"] == "未写完的笔记v2", r)
check("删除草稿", req("/api/drafts/%s" % did, "DELETE")[0] == 200)

# ============ 内容审核：黄/红/驳回 + 年龄认证 ============
check("切到 alice", login("alice", "pass123"))
s, r = req("/api/notes", "POST", {"title": "黄色内容笔记", "content": "x", "mediaType": "image", "media": media, "tags": ["黄标测试"], "category": "影视"})
nid2 = r["note"]["id"]
s, r = req("/api/notes", "POST", {"title": "红色内容笔记", "content": "x", "mediaType": "image", "media": media, "tags": ["红标测试"], "category": "影视"})
nid3 = r["note"]["id"]
s, r = req("/api/notes", "POST", {"title": "将被驳回的笔记", "content": "x", "mediaType": "image", "media": media, "tags": [], "category": "生活"})
nid4 = r["note"]["id"]
check("切到 admin", login("admin", "admin123"))
check("审核通过(黄)", req("/api/admin/notes/%s/review" % nid2, "POST", {"level": "yellow"})[0] == 200)
check("审核通过(红)", req("/api/admin/notes/%s/review" % nid3, "POST", {"level": "red"})[0] == 200)
check("驳回笔记", req("/api/admin/notes/%s/reject" % nid4, "POST", {})[0] == 200)
s, r = req("/api/admin/notes/%s/review" % nid4, "POST", {"level": "bad"})
check("非法等级被拒", s == 400, r)
check("切到 alice", login("alice", "pass123"))
s, r = req("/api/notifications")
check("作者收到驳回通知", s == 200 and any(n["type"] == "reject" for n in r["items"]), r)
s, r = req("/api/feed")
# 红黄绿三色均进首页：黄标正常展示、红标登录可见（前端模糊+确认），驳回不展示
check("黄/红进首页，驳回不展示", s == 200 and all(n["id"] != nid4 for n in r["items"]) and any(n["id"] == nid2 for n in r["items"]) and any(n["id"] == nid3 for n in r["items"]), [n["id"] for n in r["items"]])
s, r = req("/api/notes/%s" % nid4)
check("作者可见自己被驳回的笔记(rejected=true)", s == 200 and r.get("rejected") is True, r)
s, r = req("/api/notes/%s" % nid2)
check("黄标可自由浏览", s == 200 and r["note"]["level"] == "yellow", r)
s, r = req("/api/notes/%s" % nid3)
check("红标登录可浏览(带敏感标记)", s == 200 and r["note"]["level"] == "red" and r.get("sensitive") is True, r)
s, r = req("/api/feed?q=%s" % "黄标测试")
check("搜索可见黄标", s == 200 and any(n["id"] == nid2 for n in r["items"]), r)
s, r = req("/api/feed?q=%s" % "红标测试")
check("搜索显示红标(登录用户)", s == 200 and any(n["id"] == nid3 for n in r["items"]), r)
s, r = req("/api/user/%s/notes" % uid)
check("主页显示红标(登录用户)", s == 200 and any(n["id"] == nid3 for n in r), r)
# 年龄认证标记仍可设置（兼容旧字段），不影响红标浏览门槛
check("切到 admin", login("admin", "admin123"))
check("设置年龄认证", req("/api/admin/users/%s/ageverify" % uid, "POST", {"ageVerified": True})[0] == 200)
check("切到 alice", login("alice", "pass123"))
s, r = req("/api/notes/%s" % nid3)
check("红标可浏览(level=red)", s == 200 and r["note"]["level"] == "red", r)
s, r = req("/api/feed?q=%s" % "红标测试")
check("搜索显示红标(已认证)", s == 200 and any(n["id"] == nid3 for n in r["items"]), r)
s, r = req("/api/me")
check("alice ageVerified=true", s == 200 and r["ageVerified"] is True, r)

# ============ 批量创建用户（带官方认证） ============
check("切到 admin", login("admin", "admin123"))
s, r = req("/api/admin/users/batch", "POST", {"users": [
    {"username": "u1", "password": "pass123", "nickname": "用户一"},
    {"username": "u2", "password": "pass123", "nickname": "用户二"},
    {"username": "u1", "password": "pass123", "nickname": "重复"}
], "officialVerified": True})
check("批量创建(2成功1失败)", s == 200 and len(r["created"]) == 2 and len(r["failed"]) == 1, r)
check("批量创建直接 active", all(u["status"] == "active" for u in r["created"]), r)
check("批量创建带官方认证", all(u["officialVerified"] for u in r["created"]), r)
check("批量创建用户可登录", login("u1", "pass123"))
s, r = req("/api/notes/%s" % nid3)
check("批量创建用户可看红标(登录即可)", s == 200, r)
s, r = req("/api/notes/%s" % nid4)
check("他人看驳回笔记 404", s == 404, r)
check("切到 admin", login("admin", "admin123"))
s, r = req("/api/admin/users/batch", "POST", {"users": [], "officialVerified": False})
check("空列表被拒(admin)", s == 400, r)
check("切到 alice", login("alice", "pass123"))

# ============ 举报系统 ============
# 注：nid 已在前文被编辑为 pending（待审核），举报接口只接受已发布笔记，故此处用黄标已发布的 nid2（作者同为 alice）
check("切到 admin", login("admin", "admin123"))
s, r = req("/api/report/note/%s" % nid2, "POST", {"reason": "测试举报理由"})
check("举报笔记", s == 200 and r["status"] == "pending", r)
s, r = req("/api/report/note/%s" % nid2, "POST", {"reason": "重复举报"})
check("重复举报被拒", s == 400, r)
s, r = req("/api/report/note/%s" % nid2, "POST", {"reason": ""})
check("空理由被拒", s == 400, r)
s, r = req("/api/admin/reports")
check("举报列表含该举报", s == 200 and any(x["target"] == "note" and x["targetId"] == nid2 for x in r), r)
rp_id = [x for x in r if x["targetId"] == nid2][0]["id"]
check("举报处理(忽略)", req("/api/admin/reports/%s/resolve" % rp_id, "POST", {"status": "ignored"})[0] == 200)
s, r = req("/api/admin/reports")
check("举报状态=ignored", s == 200 and [x for x in r if x["id"] == rp_id][0]["status"] == "ignored", r)
check("切到 alice", login("alice", "pass123"))
s, r = req("/api/notes/%s/comment" % nid2, "POST", {"content": "这条评论将被举报"})
check("alice 发评论(供举报)", s == 200, r)
rep_cid = r["id"]
check("切到 admin", login("admin", "admin123"))
s, r = req("/api/report/comment/%s" % rep_cid, "POST", {"reason": "评论不当"})
check("举报评论", s == 200 and r["target"] == "comment", r)
check("切到 alice", login("alice", "pass123"))
s, r = req("/api/report/note/%s" % nid2, "POST", {"reason": "自己举报自己"})
check("不能举报自己的笔记", s == 400, r)

# ============ 用户搜索 ============
check("切到 admin", login("admin", "admin123"))
s, r = req("/api/me")
admin_uid, admin_id = r["uid"], r["id"]
check("切到 alice", login("alice", "pass123"))
s, r = req("/api/users/search?q=%s" % "alice")
check("按用户名搜到", s == 200 and any(u["username"] == "alice" for u in r), r)
s, r = req("/api/users/search?q=%s" % "爱丽丝")
check("按昵称搜到", s == 200 and any(u["nickname"] == "爱丽丝酱" for u in r), r)
s, r = req("/api/users/search?q=%s" % admin_uid)
check("按用户号搜到 admin", s == 200 and any(u["id"] == admin_id for u in r), r)
s, r = req("/api/users/search?q=%s" % "不存在的用户xyz")
check("搜不到返回空数组", s == 200 and r == [], r)

# ============ 管理 + 清理 ============
check("切到 admin", login("admin", "admin123"))
s, r = req("/api/admin/stats")
check("统计 users>=2 notes>=1", s == 200 and r["users"] >= 2 and r["notes"] >= 1, r)

# ============ 评论点赞 ============
# 注：同样使用已发布的黄标笔记 nid2（nid 已被编辑为 pending，不可评论）
check("切到 admin", login("admin", "admin123"))
s, r = req("/api/notes/%s/comment" % nid2, "POST", {"content": "要被赞的评论"})
check("admin 发评论(供点赞)", s == 200, r)
cl_id = r["id"]
check("切到 alice", login("alice", "pass123"))
s, r = req("/api/comments/%s/like" % cl_id, "POST", {})
check("评论点赞", s == 200 and r["liked"] is True and r["likeCount"] == 1, r)
s, r = req("/api/comments/%s/like" % cl_id, "POST", {})
check("评论取消赞", s == 200 and r["liked"] is False and r["likeCount"] == 0, r)
s, r = req("/api/comments/%s/like" % cl_id, "POST", {})
check("再次点赞", s == 200 and r["liked"] is True, r)
s, r = req("/api/notes/%s" % nid2)
check("详情评论带 liked/likeCount", s == 200 and any(c["id"] == cl_id and c["liked"] and c["likeCount"] == 1 for c in r["comments"]), r)
check("切到 admin", login("admin", "admin123"))
s, r = req("/api/notifications")
check("评论作者收到点赞通知", s == 200 and any(n["type"] == "commentLike" for n in r["items"]), r)

# ============ 私信 ============
check("切到 alice", login("alice", "pass123"))
s, r = req("/api/me")
alice_uid, alice_id = r["uid"], r["id"]
s, r = req("/api/messages/send", "POST", {"to": admin_id, "content": "你好管理员，测试私信"})
check("发送私信", s == 200 and r["toId"] == admin_id, r)
s, r = req("/api/messages/send", "POST", {"to": alice_id, "content": "给自己"})
check("不能给自己发私信", s == 400, r)
s, r = req("/api/messages/send", "POST", {"to": admin_id, "content": ""})
check("空内容被拒", s == 400, r)
s, r = req("/api/messages")
check("alice 会话列表含 admin", s == 200 and any(c["peer"]["id"] == admin_id for c in r), r)
s, r = req("/api/messages/with/%s" % admin_id)
check("对话记录含消息", s == 200 and any(m["content"] == "你好管理员，测试私信" for m in r["messages"]), r)
check("私信接口不泄露密码哈希", s == 200 and "passwordHash" not in r["peer"], r)
check("切到 admin", login("admin", "admin123"))
s, r = req("/api/messages/unread")
check("admin 未读私信>=1", s == 200 and r["unread"] >= 1, r)
s, r = req("/api/messages/with/%s" % uid)
check("admin 打开会话后消息已读", s == 200, r)
s, r = req("/api/messages/unread")
check("已读后未读=0", s == 200 and r["unread"] == 0, r)
s, r = req("/api/messages")
check("admin 会话列表含 alice 且未读=0", s == 200 and any(c["peer"]["id"] == uid and c["unread"] == 0 for c in r), r)
check("切到 alice", login("alice", "pass123"))

# ============ 好友系统 ============
check("切到 admin", login("admin", "admin123"))
s, r = req("/api/me")
admin_uid, admin_id = r["uid"], r["id"]
check("admin 用户号补齐为8位", len(admin_uid or "") == 8, admin_uid)
check("切到 alice", login("alice", "pass123"))
s, r = req("/api/uid/%s" % admin_uid)
check("按用户号查找", s == 200 and r["user"]["id"] == admin_id and r["relation"] == "none", r)
s, r = req("/api/friends/request", "POST", {"uid": admin_uid})
check("发送好友请求", s == 200, r)
s, r = req("/api/friends/request", "POST", {"uid": admin_uid})
check("重复请求被拒", s == 400, r)
check("切到 admin", login("admin", "admin123"))
s, r = req("/api/friends/requests")
check("admin 收到好友请求", s == 200 and any(x["fromId"] == uid for x in r["received"]), r)
req_id = [x for x in r["received"] if x["fromId"] == uid][0]["id"]
s, r = req("/api/friends/requests/%s/accept" % req_id, "POST", {})
check("接受好友请求", s == 200, r)
s, r = req("/api/friends")
check("admin 好友列表含 alice", s == 200 and any(u["id"] == uid for u in r), r)
check("切到 alice", login("alice", "pass123"))
s, r = req("/api/notifications")
check("alice 收到好友通知", s == 200 and any(n["type"] in ("friend", "friendAccepted") for n in r["items"]), r)
s, r = req("/api/friends/%s" % admin_id, "DELETE")
check("删除好友", s == 200, r)
s, r = req("/api/friends")
check("好友列表已清空", s == 200 and len(r) == 0, r)

# ============ 重置密码 ============
check("切到 admin", login("admin", "admin123"))
s, r = req("/api/admin/users/%s/reset-password" % uid, "POST", {"newPassword": "newpass123"})
check("管理员重置密码", s == 200, r)
check("旧密码登录失败", req("/api/login", "POST", {"username": "alice", "password": "pass123"})[0] == 401)
check("新密码登录成功", login("alice", "newpass123"))
s, r = req("/api/notes/%s" % nid)
check("重置密码后功能正常", s == 200, r)

# ============ 被禁用户内容隐藏 ============
check("切到 admin", login("admin", "admin123"))
# u2 发一篇绿标笔记（先审核通过），然后禁用 u2，验证其内容从首页/搜索/标签消失
check("切到 u2", login("u2", "pass123"))
s, r = req("/api/upload", "POST", multipart={"file": ("u2.png", png, "image/png")})
u2_media = [r["path"]]
s, r = req("/api/notes", "POST", {"title": "u2 的笔记", "content": "将被隐藏", "mediaType": "image", "media": u2_media, "tags": ["隐藏测试"], "category": "生活"})
u2nid = r["note"]["id"]
u2_id = r["note"]["authorId"]
check("切到 admin", login("admin", "admin123"))
check("审核通过 u2 笔记", req("/api/admin/notes/%s/review" % u2nid, "POST", {"level": "green"})[0] == 200)
s, r = req("/api/feed")
check("u2 绿标笔记进首页", s == 200 and any(n["id"] == u2nid for n in r["items"]), r)
# 禁用 u2
check("切到 admin", login("admin", "admin123"))
check("禁用 u2", req("/api/admin/users/%s/reject" % u2_id, "POST", {})[0] == 200)
s, r = req("/api/feed")
check("被禁用户笔记从首页消失", s == 200 and all(n["id"] != u2nid for n in r["items"]), r)
s, r = req("/api/feed?q=%s" % "隐藏测试")
check("被禁用户笔记从搜索消失", s == 200 and all(n["id"] != u2nid for n in r["items"]), r)
s, r = req("/api/tag/%s/notes" % "隐藏测试")
check("被禁用户笔记从标签消失", s == 200 and all(n["id"] != u2nid for n in r["items"]), r)
s, r = req("/api/user/%s/notes" % u2_id)
check("被禁用户主页笔记为空", s == 200 and all(n["id"] != u2nid for n in r), r)
# 非管理员（alice）直链访问被禁用户笔记应 404
check("切到 alice", login("alice", "newpass123"))
s, r = req("/api/notes/%s" % u2nid)
check("被禁用户笔记直链 404(非管理员)", s == 404, r)
# 管理员仍可看（管理豁免）
check("切到 admin", login("admin", "admin123"))
s, r = req("/api/notes/%s" % u2nid)
check("管理员仍可查看被禁用户笔记", s == 200, r)

# ============ 删除 / 清理 ============
check("切到 alice", login("alice", "newpass123"))
check("作者删除笔记", req("/api/notes/%s" % nid, "DELETE")[0] == 200)
code, _ = raw_get(media[0])
check("删除后文件 404", code == 404, code)
s, r = req("/api/notes/%s" % nid, "PUT", {"title": "编辑已下架笔记"})
check("编辑已下架笔记被拒", s == 404, r)
s, r = req("/api/notes/%s/like" % nid, "POST", {})
check("下架后点赞被拒", s == 404, r)
check("切到 admin", login("admin", "admin123"))
s, r = req("/api/notifications")
check("删除笔记后相关通知被清理", s == 200 and all(n.get("noteId") != nid for n in r["items"]), r)

print("ALL PASS")
