/* 素见 前端公共脚本 */
'use strict';

/* 源力设计系统图标（assets/icons/ 内联，单色 currentColor 风格） */
const ICON = {
  inbox: '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 48 48" fill="none" width="46" height="46"><path fill-rule="evenodd" clip-rule="evenodd" d="M42 4a2 2 0 012 2v36a2 2 0 01-2 2H6a2 2 0 01-2-2V6a2 2 0 012-2h36zM8 40h32V30h-4v4a2 2 0 01-2 2H14a2 2 0 01-2-2v-4H8v10zM40 8H8v18h6a2 2 0 012 2v4h16v-4a2 2 0 012-2h6V8z" fill="currentColor"/></svg>',
  clock: '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 48 48" fill="none" width="46" height="46"><path fill-rule="evenodd" clip-rule="evenodd" d="M24 2c12.15 0 22 9.85 22 22s-9.85 22-22 22S2 36.15 2 24 11.85 2 24 2zm0 4C14.059 6 6 14.059 6 24s8.059 18 18 18 18-8.059 18-18S33.941 6 24 6zm1 8a1 1 0 011 1v7h7a1 1 0 011 1v2a1 1 0 01-1 1H23a1 1 0 01-1-1V15a1 1 0 011-1h2z" fill="currentColor"/></svg>',
  alert: '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 48 48" fill="none" width="46" height="46"><path fill-rule="evenodd" clip-rule="evenodd" d="M27.452 5.985a4 4 0 00-6.91 0L2.545 36.015A4 4 0 006 42.032h35.993a4 4 0 003.455-6.015L27.452 5.986zM6 38.031l17.997-30.03 17.996 30.03H6zm18.747-6c.76 0 1.375.616 1.375 1.375v1.375c0 .76-.616 1.375-1.375 1.375h-1.375c-.76 0-1.375-.615-1.375-1.375v-1.375c0-.759.616-1.375 1.375-1.375h1.375zm1.258-15.002a1 1 0 00-1-1h-2a1 1 0 00-1 1V29.03a1 1 0 001 1h2a1 1 0 001-1v-12z" fill="currentColor"/></svg>',
  notification: '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 48 48" fill="none" width="46" height="46"><path fill-rule="evenodd" clip-rule="evenodd" d="M36 20a8 8 0 110-16 8 8 0 010 16zm2 2.5a.5.5 0 01.5-.5h3a.5.5 0 01.5.5V42a2 2 0 01-2 2H6a2 2 0 01-2-2V8a2 2 0 012-2h19.5a.5.5 0 01.5.5v3a.5.5 0 01-.5.5H8v30h29a1 1 0 001-1V22.5zM40 12a4 4 0 11-8 0 4 4 0 018 0z" fill="currentColor"/></svg>',
  eye: '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 48 48" fill="none" width="46" height="46"><path fill-rule="evenodd" clip-rule="evenodd" d="M23.97 37c6.476 0 12.47-4.12 18.03-13.026C36.584 15.099 30.602 11 23.97 11 17.34 11 11.378 15.097 6 23.974 11.521 32.883 17.496 37 23.97 37zm-.003-30c8.043 0 14.965 4.916 20.767 14.748l.355.61.373.664a2 2 0 01-.01 1.957l-.4.694C39.037 35.891 32.01 41 23.966 41c-7.886 0-14.78-4.913-20.677-14.74l-.285-.48a49.45 49.45 0 01-.354-.615l-.108-.19a2 2 0 01-.01-1.948l.337-.607.18-.316C8.856 12.034 15.829 7 23.967 7zM24 16a8 8 0 100 16 8 8 0 000-16zm0 4a4 4 0 110 8 4 4 0 010-8z" fill="currentColor"/></svg>',
  edit: '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 48 48" fill="none" width="46" height="46"><path fill-rule="evenodd" clip-rule="evenodd" d="M43 40a1 1 0 011 1v2a1 1 0 01-1 1H5a1 1 0 01-1-1v-2a1 1 0 011-1h38zM28.87 3.886l.132.12 6.868 6.905a2 2 0 01-.004 2.825l-.739.738.013.014L13.628 36H6a2 2 0 01-1.995-1.85L4 34v-7.628l20.778-20.94-.019-.019 1.41-1.41a2 2 0 012.702-.117zm-7.107 10.206L8.065 28.059v.005l3.87 3.87h.006l13.825-13.839-4.003-4.003zm5.774-5.887l-2.972 3.031 4.029 4.029 2.989-2.992-4.046-4.068z" fill="currentColor"/></svg>',
  camera: '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 12 12" fill="none" width="15" height="15"><path fill-rule="evenodd" clip-rule="evenodd" d="M3.777 1.812c.163-.252.26-.313.463-.313h3.5c.2 0 .298.055.46.302l.3.449h2.45c.304 0 .55.224.55.5v7.5c0 .276-.246.5-.55.5h-9.9c-.304 0-.55-.224-.55-.5v-7.5c0-.276.246-.5.55-.5H3.5l.277-.438zm6.586 1.607H1.57v6.324h8.793V3.419zm-4.373.712a2.437 2.437 0 110 4.875 2.437 2.437 0 010-4.875zM6 5.25a1.313 1.313 0 100 2.625A1.313 1.313 0 006 5.25z" fill="currentColor"/></svg>',
  video: '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 48 48" fill="none" width="15" height="15"><path fill-rule="evenodd" clip-rule="evenodd" d="M31 8a2 2 0 011.995 1.85L33 10v28a2 2 0 01-1.85 1.995L31 40H20v-4h9V12H5v15H1V10a2 2 0 011.85-1.995L3 8h28zm16 7.532v16.936c0 1.554-1.865 2.514-3.332 1.715L36 30V18l7.668-4.183c1.467-.8 3.332.16 3.332 1.715zm-4 3.533l-3 1.2v7.47l3 1.2v-9.87zM10 16h4a1 1 0 01.993.883L15 17v4a1 1 0 01-.883.993L14 22h-4a1 1 0 01-.993-.883L9 21v-4a1 1 0 01.883-.993L10 16h4-4z" fill="currentColor"/></svg>',
  image: '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 48 48" fill="none" width="26" height="26"><path fill-rule="evenodd" clip-rule="evenodd" d="M34 12a2 2 0 012 2v28a2 2 0 01-2 2H6a2 2 0 01-2-2V14a2 2 0 012-2h28zm-2 4H8v24h24V16zm-4.119 10.366a.8.8 0 01.235.565V35.4a.6.6 0 01-.6.6H23.51l.012.011H12.689a.6.6 0 01-.424-1.024l4.442-4.443a.8.8 0 011.12-.01l2.423 2.331 6.5-6.5a.8.8 0 011.131 0zM42 4a2 2 0 012 2v25a1 1 0 01-1 1h-2a1 1 0 01-1-1V8H17a1 1 0 01-1-1V5a1 1 0 011-1h25zM18 20v6h-6v-6h6z" fill="currentColor"/></svg>',
  plus: '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="none" width="30" height="30"><path d="M8 3v10M3 8h10" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/></svg>',
  sun: '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="none" width="19" height="19"><circle cx="8" cy="8" r="3.2" stroke="currentColor" stroke-width="1.4"/><path d="M8 1.2v1.8M8 13v1.8M1.2 8h1.8M13 8h1.8M3 3l1.3 1.3M11.7 11.7 13 13M13 3l-1.3 1.3M4.3 11.7 3 13" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/></svg>',
  moon: '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="none" width="19" height="19"><path d="M13.5 9.5A5.5 5.5 0 0 1 6.5 2.5a5.5 5.5 0 1 0 7 7z" stroke="currentColor" stroke-width="1.4" stroke-linejoin="round"/></svg>'
};

/* 搜索历史（localStorage，全局；顶栏搜索与搜索页共用） */
function pushSearchHistory(q) {
  q = (q || '').trim();
  if (!q) return;
  let h = [];
  try { h = JSON.parse(localStorage.getItem('sujian-search-history')) || []; } catch (e) {}
  h = h.filter(x => x !== q);
  h.unshift(q);
  if (h.length > 10) h = h.slice(0, 10);
  try { localStorage.setItem('sujian-search-history', JSON.stringify(h)); } catch (e) {}
}

/* ---------- 主题（深色 / 浅色 / 跟随系统） ---------- */
const THEME_KEY = 'sujian-theme';
function applyTheme(t) {
  const root = document.documentElement;
  if (t === 'dark') root.dataset.theme = 'dark';
  else if (t === 'light') delete root.dataset.theme;
  else { // 未设置：跟随系统
    const sysDark = window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches;
    if (sysDark) root.dataset.theme = 'dark'; else delete root.dataset.theme;
  }
}
function getStoredTheme() { try { return localStorage.getItem(THEME_KEY); } catch (e) { return null; } }
(function initTheme() { applyTheme(getStoredTheme()); })();
function toggleTheme() {
  const next = document.documentElement.dataset.theme === 'dark' ? 'light' : 'dark';
  try { localStorage.setItem(THEME_KEY, next); } catch (e) {}
  applyTheme(next);
  const btn = document.getElementById('nav-theme');
  if (btn) btn.innerHTML = next === 'dark' ? ICON.sun : ICON.moon;
}

/* ---------- 基础工具 ---------- */
async function api(path, opts) {
  opts = opts || {};
  const init = { method: opts.method || 'GET', credentials: 'same-origin', headers: {} };
  if (opts.body !== undefined) {
    init.headers['Content-Type'] = 'application/json';
    init.body = JSON.stringify(opts.body);
  }
  if (opts.form !== undefined) {
    init.body = opts.form; // FormData
  }
  const res = await fetch(path, init);
  let data = null;
  try { data = await res.json(); } catch (e) { /* ignore */ }
  if (!res.ok) {
    const msg = (data && data.error) || ('请求失败 (' + res.status + ')');
    const err = new Error(msg);
    err.status = res.status;
    throw err;
  }
  return data;
}

function esc(s) {
  return String(s == null ? '' : s)
    .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;').replace(/'/g, '&#39;');
}

// escJS 用于把用户内容安全塞进「双引号 HTML 属性内的单引号 JS 字符串」，
// 例如 onclick="foo('ID','"+escJS(name)+"')"。
// 关键：把 ' " \ 转成 JS 转义序列（而非 HTML 实体），否则 HTML 属性会把 &#39; 解码回 '，
// 使攻击者昵称里的 ' 逃出 JS 字符串造成存储型 XSS。
function escJS(s) {
  return String(s == null ? '' : s)
    .replace(/\\/g, '\\\\')
    .replace(/'/g, "\\'")
    .replace(/"/g, '\\"')
    .replace(/\n/g, '\\n').replace(/\r/g, '\\r');
}

function fmtTime(ts) {
  if (!ts) return '';
  const d = new Date(ts * 1000);
  const now = Date.now();
  const diff = (now - d.getTime()) / 1000;
  if (diff < 60) return '刚刚';
  if (diff < 3600) return Math.floor(diff / 60) + ' 分钟前';
  if (diff < 86400) return Math.floor(diff / 3600) + ' 小时前';
  if (diff < 86400 * 7) return Math.floor(diff / 86400) + ' 天前';
  const p = n => (n < 10 ? '0' + n : '' + n);
  return d.getFullYear() + '-' + p(d.getMonth() + 1) + '-' + p(d.getDate());
}

const AV_COLORS = ['red', 'beige', 'green', 'teal', 'purple', 'primary', 'grey'];
function avatarColor(str) {
  let h = 0;
  if (str) for (let i = 0; i < str.length; i++) h = (h * 31 + str.charCodeAt(i)) >>> 0;
  return AV_COLORS[h % AV_COLORS.length];
}

/* 头像：有图显示图，否则源力 AvatarIcon 首字母 */
function avatarHTML(name, avatarPath, size) {
  size = size || 'default';
  const px = size === 'large' ? 64 : size === 'medium' ? 28 : size === 'mini' ? 20 : 32;
  if (avatarPath) {
    return '<img class="user-avatar" src="' + esc(avatarPath) + '" alt="" style="width:' + px + 'px;height:' + px + 'px;border-radius:50%;object-fit:cover;">';
  }
  const ch = (name || '?').charAt(0).toUpperCase();
  return '<span class="avatar-icon avatar-icon-' + size + ' avatar-icon-' + avatarColor(name) + '">' + esc(ch) + '</span>';
}

let toastTimer = null;
let navDocClick = null;
function toast(msg, isErr) {
  let el = document.getElementById('toast');
  if (!el) {
    el = document.createElement('div');
    el.id = 'toast';
    el.className = 'toast';
    document.body.appendChild(el);
  }
  el.textContent = msg;
  el.classList.toggle('err', !!isErr);
  el.classList.add('show');
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => el.classList.remove('show'), 2400);
}

function getQuery(name) {
  return new URLSearchParams(location.search).get(name) || '';
}

/* ---------- 通用弹窗（替代原生 confirm / prompt，避免突兀的浏览器默认弹窗） ---------- */
function openModal(html, onMount) {
  let mask = document.getElementById('__modal');
  if (!mask) {
    mask = document.createElement('div');
    mask.id = '__modal';
    mask.className = 'modal-mask';
    mask.innerHTML = '<div class="modal-card"></div>';
    document.body.appendChild(mask);
    mask.addEventListener('click', e => { if (e.target === mask) closeModal(); });
  }
  const card = mask.querySelector('.modal-card');
  card.innerHTML = html;
  mask.classList.add('open');
  if (onMount) onMount(card, closeModal);
  return mask;
}
function closeModal() {
  const mask = document.getElementById('__modal');
  if (mask) mask.classList.remove('open');
}

/* 确认弹窗：返回 Promise<boolean> */
function confirmDialog(title, message, okText) {
  return new Promise(resolve => {
    const done = (v) => { closeModal(); resolve(v); };
    openModal(
      '<h3>' + esc(title) + '</h3>' +
      '<div class="modal-msg">' + esc(message) + '</div>' +
      '<div class="modal-actions">' +
      '<button class="btn btn-outline btn-md" id="__m_cancel">取消</button>' +
      '<button class="btn btn-primary btn-md" id="__m_ok">' + esc(okText || '确定') + '</button>' +
      '</div>',
      (card) => {
        card.querySelector('#__m_cancel').addEventListener('click', () => done(false));
        card.querySelector('#__m_ok').addEventListener('click', () => done(true));
      }
    );
  });
}

/* 输入弹窗：返回 Promise<string|null>（取消/关闭为 null） */
function promptDialog(title, placeholder, defaultValue) {
  return new Promise(resolve => {
    const done = (v) => { closeModal(); resolve(v); };
    openModal(
      '<h3>' + esc(title) + '</h3>' +
      '<input class="modal-input" id="__m_input" placeholder="' + esc(placeholder || '') + '">' +
      '<div class="modal-actions">' +
      '<button class="btn btn-outline btn-md" id="__m_cancel">取消</button>' +
      '<button class="btn btn-primary btn-md" id="__m_ok">确定</button>' +
      '</div>',
      (card) => {
        const input = card.querySelector('#__m_input');
        input.value = defaultValue || '';
        input.focus();
        input.addEventListener('keydown', e => {
          if (e.key === 'Enter') { e.preventDefault(); done(input.value.trim()); }
          if (e.key === 'Escape') { e.preventDefault(); done(null); }
        });
        card.querySelector('#__m_cancel').addEventListener('click', () => done(null));
        card.querySelector('#__m_ok').addEventListener('click', () => done(input.value.trim()));
      }
    );
  });
}

/* ---------- 顶部导航 ---------- */
async function renderTopNav(active) {
  const me = await api('/api/me');
  const nav = document.getElementById('topnav');
  if (!nav) return me;
  const menuId = 'avatar-menu';
  const adminLink = me.role === 'admin' ? '<a href="/admin"><svg width="15" height="15" viewBox="0 0 16 16" fill="none"><path d="M8 1.5 15 4.8v3.4c0 3.6-3 6.4-7 6.8-4-.4-7-3.2-7-6.8V4.8L8 1.5z" stroke="currentColor" stroke-width="1.4" stroke-linejoin="round"/></svg>管理后台</a>' : '';
  let unread = 0;
  try { unread = (await api('/api/notifications')).unread || 0; } catch (e) {}
  let msgUnread = 0;
  try { msgUnread = (await api('/api/messages/unread')).unread || 0; } catch (e) {}
  const bell = '<a class="nav-icon-btn" id="nav-bell" href="/notifications" title="消息">' +
    '<svg width="19" height="19" viewBox="0 0 16 16" fill="none"><path d="M8 1.8c-2.6 0-4.2 2-4.2 4.4v2.2l-1.3 2.1c-.3.6.1 1.2.8 1.2h9.4c.7 0 1.1-.6.8-1.2L12.2 8.4V6.2c0-2.4-1.6-4.4-4.2-4.4z" stroke="currentColor" stroke-width="1.4" stroke-linejoin="round"/><path d="M6.4 12.6a1.7 1.7 0 0 0 3.2 0" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/></svg>' +
    (unread > 0 ? '<span class="bell-badge">' + (unread > 99 ? '99+' : unread) + '</span>' : '') +
    '</a>';
  const mail = '<a class="nav-icon-btn" id="nav-mail" href="/messages" title="私信">' +
    '<svg width="19" height="19" viewBox="0 0 16 16" fill="none"><rect x="2" y="3.4" width="12" height="9.2" rx="1.6" stroke="currentColor" stroke-width="1.4"/><path d="m2.8 4.6 5.2 4 5.2-4" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/></svg>' +
    (msgUnread > 0 ? '<span class="bell-badge">' + (msgUnread > 99 ? '99+' : msgUnread) + '</span>' : '') +
    '</a>';
  nav.innerHTML =
    '<a class="logo" href="/"><span class="logo-dot"></span>素见</a>' +
    '<div class="search-box">' +
    '  <input id="nav-search" type="search" placeholder="搜索笔记 / 标签 / 作者" value="' + esc(getQuery('q')) + '">' +
    '</div>' +
    '<div class="topnav-right">' +
    mail +
    bell +
    '  <button class="nav-icon-btn" id="nav-theme" title="切换深色 / 浅色" onclick="toggleTheme()">' + (document.documentElement.dataset.theme === 'dark' ? ICON.sun : ICON.moon) + '</button>' +
    '  <button class="btn btn-primary btn-md" onclick="location.href=\'/publish\'">' +
    '    <svg width="14" height="14" viewBox="0 0 16 16" fill="none"><path d="M8 3v10M3 8h10" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/></svg><span class="pub-label">发布</span>' +
    '  </button>' +
    '  <div class="avatar-wrap">' +
    '    <div class="avatar-trigger" onclick="toggleMenu(\'' + menuId + '\')">' + avatarHTML(me.nickname, me.avatar) +
    '    </div>' +
    '    <div class="avatar-menu" id="' + menuId + '">' +
    '      <a href="/profile?u=' + esc(me.id) + '">个人主页</a>' +
    '      <a href="/friends">好友</a>' +
    '      <a href="/messages">私信</a>' +
    '      <a href="/drafts">草稿箱</a>' +
    '      <a href="/search">搜索</a>' +
    adminLink +
    '      <a class="danger" href="#" onclick="doLogout();return false;">退出登录</a>' +
    '    </div>' +
    '  </div>' +
    '</div>';
  const input = document.getElementById('nav-search');
  if (input) {
    input.addEventListener('keydown', e => {
      if (e.key === 'Enter') {
        const v = input.value.trim();
        if (v) pushSearchHistory(v);
        location.href = '/search?q=' + encodeURIComponent(v);
      }
    });
  }
  if (navDocClick) document.removeEventListener('click', navDocClick);
  navDocClick = e => {
    if (!e.target.closest('.avatar-wrap')) {
      const m = document.getElementById(menuId);
      if (m) m.classList.remove('open');
    }
  };
  document.addEventListener('click', navDocClick);
  startUnreadPoll(); // 启动顶栏未读红点轮询（全局只启动一次）
  return me;
}

/* 顶栏未读红点实时刷新：不重建导航，只更新铃铛/私信徽标；页面隐藏时暂停 */
let unreadPollTimer = null;
function startUnreadPoll() {
  if (unreadPollTimer) return;
  unreadPollTimer = setInterval(async () => {
    if (document.hidden) return;
    try {
      const n = await api('/api/notifications');
      const m = await api('/api/messages/unread');
      setBadge('nav-bell', (n && n.unread) || 0);
      setBadge('nav-mail', (m && m.unread) || 0);
    } catch (e) { /* 静默：网络抖动时保留当前徽标 */ }
  }, 30000);
}
function setBadge(anchorId, count) {
  const a = document.getElementById(anchorId);
  if (!a) return;
  let b = a.querySelector('.bell-badge');
  if (count > 0) {
    if (!b) {
      b = document.createElement('span');
      b.className = 'bell-badge';
      a.appendChild(b);
    }
    b.textContent = count > 99 ? '99+' : count;
  } else if (b) {
    b.remove();
  }
}

function toggleMenu(id) {
  const m = document.getElementById(id);
  if (m) m.classList.toggle('open');
}

async function doLogout() {
  try { await api('/api/logout', { method: 'POST' }); } catch (e) {}
  location.href = '/login';
}

/* ---------- 笔记卡片 ---------- */
const LV_DOT_C = { green: '#2a9d5f', yellow: '#f0b429', red: '#e5484d', black: '#111418' };
function lvDotHTML(level) {
  if (!level) return '';
  const c = LV_DOT_C[level];
  return '<span class="nc-lv" style="background:' + c + ';"></span>';
}
function noteCardHTML(n) {
  const cover = n.media && n.media[0] ? n.media[0] : '';
  const sensitive = n.level === 'red' || n.level === 'black'; // 红/黑标=敏感内容，封面模糊
  // 媒体类型判定：旧数据可能 mediaType='image' 但 src 是 mp4（如 data/notes.json 里早期发布的视频笔记），
  // Chrome 会拒绝用 <img> 渲染 mp4 显示破图标；Safari 宽容能渲染首帧 —— 行为不一致。
  // 统一以「文件后缀」为准：视频扩展名 → <video>，其余 → <img>，两端浏览器行为一致。
  const isVideo = (n.mediaType === 'video') || /\.(mp4|mov|webm|m4v)(\?|$)/i.test(cover);
  let mediaHtml;
  if (isVideo) {
    // 视频卡片：优先用系统生成的封面图(n.cover, 发布时自动截首帧)作为 poster，
    // 避免 preload="metadata" 在部分编码下无法显示首帧导致的"无封面"黑块。
    mediaHtml = '<video class="nc-media' + (sensitive ? ' nc-blur' : '') + '" src="' + esc(cover) + '" preload="metadata" muted playsinline' +
      (n.cover ? ' poster="' + esc(n.cover) + '"' : '') + '></video>';
  } else {
    mediaHtml = '<img class="nc-media' + (sensitive ? ' nc-blur' : '') + '" src="' + esc(cover) + '" alt="' + esc(n.title) + '" loading="lazy" onerror="this.onerror=null;this.removeAttribute(\'src\');this.classList.add(\'img-broken\');">';
  }
  const tags = (n.tags || []).slice(0, 2).map(t => '<span class="tag">#' + esc(t) + '</span>').join('');
  const pin = n.pinned ? '<span class="nc-pin">置顶</span>' : '';
  const sensBadge = n.level === 'black' ? '<span class="nc-badge nc-badge-black">黑标·敏感</span>' :
                    sensitive ? '<span class="nc-badge nc-badge-sens">敏感</span>' : '';
  const statusBadge = n.status === 'pending' ? '<span class="nc-badge" style="background:#fff7e6; color:#ad6800; border:1px solid #ffd591;">审核中</span>' :
                      n.status === 'rejected' ? '<span class="nc-badge" style="background:#fff1f0; color:#cf1322; border:1px solid #ffa39e;">未通过</span>' : '';
  return (
    '<div class="note-card" data-note-id="' + esc(n.id) + '" onclick="location.href=\'/note?id=' + esc(n.id) + '\'">' +
    // 右上角「…」菜单：不感兴趣（静默生效，无文案）
    '<button class="nc-more" type="button" aria-label="更多" onclick="event.stopPropagation();toggleNoteMenu(event,\'' + esc(n.id) + '\')">⋯</button>' +
    '<div class="nc-menu" id="note-menu-' + esc(n.id) + '" style="display:none;" onclick="event.stopPropagation()">' +
    '<div class="nc-menu-item" onclick="dislikeNote(\'' + esc(n.id) + '\')">不感兴趣</div>' +
    '</div>' +
    pin +
    statusBadge +
    sensBadge +
    mediaHtml +
    '<div class="nc-body">' +
    '<div class="nc-title">' + lvDotHTML(n.level) + esc(n.title) + '</div>' +
    (tags ? '<div class="nc-tags">' + tags + '</div>' : '') +
    '</div>' +
    '<div class="nc-footer">' +
    avatarHTML(n.authorName, '', 'mini') +
    '<span class="nc-name">' + esc(n.authorName) + '</span>' +
    '<span class="nc-stats">' +
    // 心形 = 点赞数
    '<span><svg width="13" height="13" viewBox="0 0 16 16" fill="none"><path d="M8 14s-5.5-3.2-5.5-7a3 3 0 0 1 5.5-1.7A3 3 0 0 1 13.5 7c0 3.8-5.5 7-5.5 7z" stroke="currentColor" stroke-width="1.4" stroke-linejoin="round"/></svg>' + (n.likeCount || 0) + '</span>' +
    // 星形 = 收藏数
    '<span><svg width="13" height="13" viewBox="0 0 16 16" fill="none"><path d="M8 2.5 9.8 6l4 .6-2.9 2.8.7 4L8 11.8 4.4 13.4l.7-4L2.2 6.6 6.2 6 8 2.5z" stroke="currentColor" stroke-width="1.3" stroke-linejoin="round"/></svg>' + (n.favoriteCount || 0) + '</span>' +
    // 聊天气泡 = 评论数
    '<span><svg width="13" height="13" viewBox="0 0 16 16" fill="none"><path d="M2.5 4.2A1.7 1.7 0 0 1 4.2 2.5h7.6a1.7 1.7 0 0 1 1.7 1.7v5.6a1.7 1.7 0 0 1-1.7 1.7H7.2l-2.7 2.3a.6.6 0 0 1-1-.4V4.2z" stroke="currentColor" stroke-width="1.3" stroke-linejoin="round"/></svg>' + (n.commentCount || 0) + '</span>' +
    '</span>' +
    '</div>' +
    '</div>'
  );
}

/* 笔记卡片「…」菜单：展开/收起（同屏只开一个） */
function toggleNoteMenu(ev, noteID) {
  ev.stopPropagation();
  const menu = document.getElementById('note-menu-' + noteID);
  if (!menu) return;
  const open = menu.style.display !== 'none';
  // 先收起所有已开的菜单
  document.querySelectorAll('.nc-menu').forEach(m => { m.style.display = 'none'; });
  menu.style.display = open ? 'none' : 'block';
}
// 点击空白处关闭菜单
document.addEventListener('click', () => {
  document.querySelectorAll('.nc-menu').forEach(m => { m.style.display = 'none'; });
});

/* 不感兴趣：记负反馈 → 卡片淡出移除 → 提示「已减少此类内容」并支持 5 秒内撤销 */
function dislikeNote(noteID) {
  const menu = document.getElementById('note-menu-' + noteID);
  if (menu) menu.style.display = 'none';
  const card = document.querySelector('.note-card[data-note-id="' + CSS.escape(noteID) + '"]');
  if (!card) return;
  const feed = document.getElementById('feed');
  const m = feed && feed._masonry;

  // 备份一份用于撤销时放回（复位可能残留的动画样式）
  const backup = card.cloneNode(true);
  backup.style.opacity = ''; backup.style.transform = ''; backup.style.transition = '';

  // 后端记负反馈（fire-and-forget）
  fetch('/api/me/dislike', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ noteID: noteID })
  });

  // 淡出移除；同步从瀑布流内部卡片列表摘除，避免后续 reflow 重新插入
  card.style.transition = 'opacity .25s, transform .25s';
  card.style.opacity = '0';
  card.style.transform = 'scale(.96)';
  setTimeout(() => {
    if (m && m.cards) m.cards = m.cards.filter(c => c !== card);
    card.remove();
  }, 250);

  // 轻提示 + 撤销
  toastWithUndo('已减少此类内容', () => {
    // 撤销：后端删负反馈 + 卡片放回信息流
    fetch('/api/me/dislike', {
      method: 'DELETE',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ noteID: noteID })
    });
    if (m) m.append([backup]); else (feed || document.body).appendChild(backup);
  }, 5000);
}

/* 带「撤销」按钮的轻提示（与 #toast 互不干扰，独立元素） */
let undoToastTimer = null;
function toastWithUndo(msg, onUndo, ms) {
  ms = ms || 5000;
  let el = document.getElementById('toast-undo');
  if (!el) {
    el = document.createElement('div');
    el.id = 'toast-undo';
    el.className = 'toast toast-undo-box';
    document.body.appendChild(el);
  }
  el.innerHTML = '<span class="tu-msg">' + esc(msg) + '</span><button type="button" class="tu-undo">撤销</button>';
  el.classList.add('show');
  let done = false;
  const fire = (undo) => {
    if (done) return; done = true;
    clearTimeout(undoToastTimer);
    el.classList.remove('show');
    if (undo && typeof onUndo === 'function') onUndo();
  };
  el.querySelector('.tu-undo').onclick = () => fire(true);
  if (undoToastTimer) clearTimeout(undoToastTimer);
  undoToastTimer = setTimeout(() => fire(false), ms);
}

/* 把笔记对象(或 HTML 字符串 / 已构建元素)转成卡片 DOM 节点 */
function htmlToEl(html) {
  const tmp = document.createElement('div');
  tmp.innerHTML = String(html == null ? '' : html).trim();
  return tmp.firstElementChild;
}
function buildCardEls(items) {
  const list = Array.isArray(items) ? items : [items];
  return list.map(it => {
    if (it instanceof HTMLElement) return it;
    if (typeof it === 'string') return htmlToEl(it);
    return htmlToEl(noteCardHTML(it)); // 笔记对象
  }).filter(Boolean);
}
function _getMasonry(container) {
  if (!container._masonry || !(container._masonry instanceof Masonry)) {
    container._masonry = new Masonry(container, { gap: 16 });
  }
  return container._masonry;
}

/* 全量渲染信息流(切换频道/标签/分类时): 走 Masonry.reset, 空态仍显示 empty-state */
function renderFeed(container, items, emptyText) {
  if (!items || items.length === 0) {
    if (container._masonry) { container._masonry.destroy(); container._masonry = null; }
    container.classList.remove('masonry');
    container.innerHTML = '<div class="empty-state"><div class="big">' + ICON.inbox + '</div>' + (emptyText || '还没有内容，快去发布第一篇笔记吧') + '</div>';
    return;
  }
  _getMasonry(container).reset(buildCardEls(items));
}

/* 追加更多(加载更多): 新卡片自动落入当前最短列 */
function appendFeed(container, items) {
  if (!items || items.length === 0) return;
  _getMasonry(container).append(buildCardEls(items));
}

/* 统计数字缩写 */
function fmtNum(n) {
  if (n >= 10000) return (n / 10000).toFixed(1).replace(/\.0$/, '') + 'w';
  return n;
}
