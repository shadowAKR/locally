import './style.css';
import { marked } from 'marked';
import DOMPurify from 'dompurify';
import { backend, isMock } from './backend';

marked.setOptions({ gfm: true, breaks: true });
const md = (s) => DOMPurify.sanitize(marked.parse(s || ''));
const esc = (s) => String(s ?? '').replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));

const icons = {
  bolt: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M13 2 4 14h7l-1 8 9-12h-7l1-8z"/></svg>',
  send: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 19V5"/><path d="m5 12 7-7 7 7"/></svg>',
  stop: '<svg viewBox="0 0 24 24" fill="currentColor"><rect x="6" y="6" width="12" height="12" rx="2"/></svg>',
  plus: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round"><path d="M12 5v14M5 12h14"/></svg>',
  terminal: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="m4 17 6-6-6-6"/><path d="M12 19h8"/></svg>',
  file: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><path d="M14 2v6h6"/></svg>',
  folder: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v9a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/></svg>',
  search: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round"><circle cx="11" cy="11" r="7"/><path d="m20 20-3.5-3.5"/></svg>',
  info: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round"><circle cx="12" cy="12" r="9"/><path d="M12 11v5M12 8h.01"/></svg>',
  external: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M14 4h6v6"/><path d="M20 4 10 14"/><path d="M20 14v4a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h4"/></svg>',
  bell: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M6 8a6 6 0 0 1 12 0c0 7 3 9 3 9H3s3-2 3-9"/><path d="M10 21h4"/></svg>',
  trash: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 6h18"/><path d="M8 6V4h8v2"/><path d="M19 6l-1 14H6L5 6"/></svg>',
  check: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="m5 12 5 5L20 7"/></svg>',
  x: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="M6 6l12 12M18 6 6 18"/></svg>',
  warn: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M10.3 3.9 1.8 18a2 2 0 0 0 1.7 3h17a2 2 0 0 0 1.7-3L13.7 3.9a2 2 0 0 0-3.4 0z"/><path d="M12 9v4M12 17h.01"/></svg>',
  shield: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/></svg>',
  globe: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round"><circle cx="12" cy="12" r="9"/><path d="M3 12h18M12 3a14 14 0 0 1 0 18M12 3a14 14 0 0 0 0 18"/></svg>',
  clock: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round"><circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/></svg>',
  play: '<svg viewBox="0 0 24 24" fill="currentColor"><path d="M7 5v14l12-7z"/></svg>',
  mail: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="5" width="18" height="14" rx="2"/><path d="m3 7 9 6 9-6"/></svg>',
  gear: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.7 1.7 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-1.8-.3 1.7 1.7 0 0 0-1 1.5V21a2 2 0 1 1-4 0v-.1a1.7 1.7 0 0 0-1.1-1.5 1.7 1.7 0 0 0-1.8.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.7 1.7 0 0 0 .3-1.8 1.7 1.7 0 0 0-1.5-1H3a2 2 0 1 1 0-4h.1a1.7 1.7 0 0 0 1.5-1.1 1.7 1.7 0 0 0-.3-1.8l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.7 1.7 0 0 0 1.8.3H9a1.7 1.7 0 0 0 1-1.5V3a2 2 0 1 1 4 0v.1a1.7 1.7 0 0 0 1 1.5 1.7 1.7 0 0 0 1.8-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.7 1.7 0 0 0-.3 1.8V9a1.7 1.7 0 0 0 1.5 1H21a2 2 0 1 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1z"/></svg>',
  chat: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12a8 8 0 0 1-8 8H8l-5 3 1.5-4.5A8 8 0 1 1 21 12z"/></svg>',
  power: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round"><path d="M12 3v9"/><path d="M18.4 6.6a9 9 0 1 1-12.8 0"/></svg>',
};
const toolIcon = (name) => {
  if (name.startsWith('browser_')) return icons.globe;
  if (name.includes('schedul')) return icons.clock;
  return { run_shell: icons.terminal, read_file: icons.file, write_file: icons.file, list_directory: icons.folder, folder_sizes: icons.folder, find_files: icons.search, system_info: icons.info, open: icons.external, notify: icons.bell, trash: icons.trash }[name] || icons.bolt;
};

const suggestions = [
  { icon: icons.info, text: 'How is my laptop doing?', sub: 'Battery, disk, memory and load', prompt: 'Give me a quick health check of my laptop: battery, disk space, memory and CPU load.' },
  { icon: icons.mail, text: 'Summarize today’s emails', sub: 'Reads Gmail in the agent browser', prompt: 'Open my Gmail inbox and summarize the emails I received today.' },
  { icon: icons.folder, text: 'What is using my disk space?', sub: 'Drills into the biggest folders', prompt: 'Find out which folders are taking the most disk space in my home directory.' },
  { icon: icons.clock, text: 'Schedule a daily check', sub: 'Creates a scheduled task', prompt: 'Every day at 9:00 run a health check of my laptop and notify me if the disk is above 90% or the battery is below 20%.' },
];

// ---------------- state ----------------
const state = {
  running: false,
  chatId: null,
  settings: null,
  models: [],
  status: null,
  tools: [],
  paths: {},
  launchAtLogin: false,
  turn: null, // { root, text: {el, raw}, thinking: {el, body, raw}, tools: Map }
  schedules: [],
  modal: null, // 'schedules' | 'settings' | null
};

// ---------------- layout ----------------
document.querySelector('#app').innerHTML = `
  <aside class="sidebar">
    <div class="brand">
      <div class="logo">${icons.bolt}</div>
      <div><h1>Local Agent</h1><p>Runs on your machine</p></div>
    </div>
    <button class="btn new-chat" id="newChat">${icons.plus} New chat</button>

    <div class="side-section grow">
      <h3>Recent chats</h3>
      <div class="chat-list" id="chatList"><div class="empty-note">No saved chats yet</div></div>
    </div>

    <nav class="side-nav">
      <button class="nav-item" id="navSched">${icons.clock}<span>Scheduled tasks</span><span class="nav-badge hidden" id="schedCount"></span></button>
      <button class="nav-item" id="navSettings">${icons.gear}<span>Settings</span></button>
      <div class="side-status" id="sideStatus" title="Open settings"><span class="dot" id="statusDot"></span><span class="grow" id="statusText">Checking Ollama…</span><span id="mockTag"></span></div>
    </nav>
  </aside>

  <section class="main">
    <header class="topbar">
      <div class="title"><span id="chatTitle">Chat</span> <button class="model-pill" id="modelPill" title="Change model">—</button></div>
      <div class="status-pill" id="statusPill"><span class="spinner"></span><span id="statusPillText">thinking</span></div>
    </header>
    <div class="messages" id="messages">
      <div class="banner hidden" id="banner"><span>${icons.warn}</span><span class="grow" id="bannerText"></span><button class="btn sm" id="startOllama">Start Ollama</button></div>
      <div class="welcome" id="welcome">
        <div class="big-logo">${icons.bolt}</div>
        <h2>What should I do on your laptop?</h2>
        <p>I can run commands, manage files, open apps, read your email and websites in my own browser, and run tasks on a schedule. Anything that changes the system asks for your approval first.</p>
        <div class="suggestions">${suggestions.map((s, i) => `
          <button class="suggestion" data-i="${i}"><span class="s-icon">${s.icon}</span><span><div class="s-text">${esc(s.text)}</div><div class="s-sub">${esc(s.sub)}</div></span></button>`).join('')}
        </div>
      </div>
      <div class="thread" id="thread"></div>
    </div>
    <div class="toasts" id="toasts"></div>
    <div class="composer-wrap">
      <div class="composer">
        <textarea id="input" rows="1" placeholder="Ask me to do something… e.g. “summarize my unread emails” or “open VS Code”"></textarea>
        <button class="send" id="send" title="Send">${icons.send}</button>
      </div>
      <div class="composer-hint"><kbd>Enter</kbd> to send · <kbd>Shift</kbd>+<kbd>Enter</kbd> for a new line · <kbd>Esc</kbd> to stop</div>
    </div>
  </section>

  <div class="modal-backdrop hidden" id="modal">
    <div class="modal" role="dialog" aria-modal="true" aria-labelledby="modalTitle">
      <div class="modal-head">
        <div class="modal-title"><span class="modal-icon" id="modalIcon"></span><h2 id="modalTitle"></h2></div>
        <div class="modal-actions" id="modalActions"></div>
        <button class="icon-btn lg" id="modalClose" title="Close (Esc)">${icons.x}</button>
      </div>
      <div class="modal-body" id="modalBody"></div>
    </div>
  </div>
`;

const $ = (id) => document.getElementById(id);
const els = {
  messages: $('messages'), thread: $('thread'), welcome: $('welcome'), input: $('input'), send: $('send'),
  statusPill: $('statusPill'), statusPillText: $('statusPillText'), modelPill: $('modelPill'), chatTitle: $('chatTitle'),
  statusDot: $('statusDot'), statusText: $('statusText'),
  banner: $('banner'), bannerText: $('bannerText'),
  chatList: $('chatList'), schedCount: $('schedCount'), toasts: $('toasts'),
  modal: $('modal'), modalTitle: $('modalTitle'), modalIcon: $('modalIcon'), modalActions: $('modalActions'), modalBody: $('modalBody'),
};
if (isMock) $('mockTag').textContent = 'preview';

// ---------------- helpers ----------------
function scrollToBottom(force = false) {
  const m = els.messages;
  const nearBottom = m.scrollHeight - m.scrollTop - m.clientHeight < 120;
  if (force || nearBottom) m.scrollTop = m.scrollHeight;
}
function setStatus(text) {
  if (!text) { els.statusPill.classList.remove('show'); return; }
  els.statusPillText.textContent = text;
  els.statusPill.classList.add('show');
}
function setRunning(v) {
  state.running = v;
  els.send.innerHTML = v ? icons.stop : icons.send;
  els.send.classList.toggle('stop', v);
  els.send.title = v ? 'Stop' : 'Send';
  if (!v) { setStatus(''); if (!state.modal) els.input.focus(); }
}
function autoResize() {
  els.input.style.height = 'auto';
  els.input.style.height = Math.min(els.input.scrollHeight, 180) + 'px';
}
function summarizeArgs(tool, argsJSON) {
  try {
    const a = typeof argsJSON === 'string' ? JSON.parse(argsJSON || '{}') : (argsJSON || {});
    const first = a.command ?? a.path ?? a.url ?? a.target ?? a.text ?? a.pattern ?? a.prompt ?? a.title ?? a.field ?? Object.values(a)[0];
    return first != null ? String(first) : '';
  } catch { return argsJSON || ''; }
}
function prettyArgs(argsJSON) {
  try { return JSON.stringify(typeof argsJSON === 'string' ? JSON.parse(argsJSON || '{}') : argsJSON, null, 2); } catch { return argsJSON; }
}
function timeAgo(iso) {
  const d = (Date.now() - new Date(iso).getTime()) / 1000;
  if (d < 60) return 'now';
  if (d < 3600) return Math.floor(d / 60) + 'm';
  if (d < 86400) return Math.floor(d / 3600) + 'h';
  return Math.floor(d / 86400) + 'd';
}
function fmtWhen(iso) {
  const t = new Date(iso);
  if (isNaN(t) || t.getFullYear() < 2000) return '';
  const sameDay = t.toDateString() === new Date().toDateString();
  return (sameDay ? 'today ' : t.toLocaleDateString(undefined, { weekday: 'short', day: 'numeric', month: 'short' }) + ' ') + t.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' });
}
function toast(text, actionLabel, onAction) {
  const el = document.createElement('div');
  el.className = 'toast';
  el.innerHTML = `<span>${icons.clock}</span><span class="grow">${esc(text)}</span>${actionLabel ? `<button class="btn sm">${esc(actionLabel)}</button>` : ''}<button class="icon-btn">${icons.x}</button>`;
  el.querySelector('.icon-btn').onclick = () => el.remove();
  if (actionLabel) el.querySelector('.btn').onclick = () => { onAction?.(); el.remove(); };
  els.toasts.appendChild(el);
  setTimeout(() => el.remove(), 12000);
}

// ---------------- modal ----------------
function openModal(kind, title, icon, actionsHTML = '') {
  state.modal = kind;
  els.modalTitle.textContent = title;
  els.modalIcon.innerHTML = icon;
  els.modalActions.innerHTML = actionsHTML;
  els.modalBody.innerHTML = '';
  els.modalBody.scrollTop = 0;
  els.modal.classList.remove('hidden');
  requestAnimationFrame(() => els.modal.classList.add('show'));
}
function closeModal() {
  if (!state.modal) return;
  state.modal = null;
  els.modal.classList.remove('show');
  setTimeout(() => { if (!state.modal) els.modal.classList.add('hidden'); }, 160);
  els.input.focus();
}
els.modal.addEventListener('click', (e) => { if (e.target === els.modal) closeModal(); });
$('modalClose').addEventListener('click', closeModal);
document.addEventListener('keydown', (e) => { if (e.key === 'Escape' && state.modal) { e.preventDefault(); closeModal(); } });

// ---------------- rendering ----------------
function clearThread() {
  els.thread.innerHTML = '';
  state.turn = null;
}
function addUserMessage(text) {
  els.welcome.classList.add('hidden');
  const el = document.createElement('div');
  el.className = 'msg user';
  el.innerHTML = `<div class="bubble">${esc(text)}</div>`;
  els.thread.appendChild(el);
  scrollToBottom(true);
}
function startTurn() {
  els.welcome.classList.add('hidden');
  const root = document.createElement('div');
  root.className = 'msg assistant';
  root.innerHTML = `<div class="avatar">${icons.bolt}</div>`;
  els.thread.appendChild(root);
  state.turn = { root, text: null, thinking: null, tools: new Map(), renderQueued: false };
  scrollToBottom(true);
}
function ensureTextBubble() {
  const t = state.turn;
  if (t.text) return t.text;
  const el = document.createElement('div');
  el.className = 'bubble md streaming';
  t.root.appendChild(el);
  t.text = { el, raw: '' };
  return t.text;
}
function appendText(delta) {
  const t = state.turn; if (!t) return;
  const tb = ensureTextBubble();
  tb.raw += delta;
  if (!t.renderQueued) {
    t.renderQueued = true;
    requestAnimationFrame(() => {
      t.renderQueued = false;
      tb.el.innerHTML = md(tb.raw);
      scrollToBottom();
    });
  }
}
function finishText() {
  const t = state.turn; if (!t?.text) return;
  t.text.el.innerHTML = md(t.text.raw);
  t.text.el.classList.remove('streaming');
  if (!t.text.raw.trim()) t.text.el.remove();
  t.text = null;
}
function appendThinking(delta) {
  const t = state.turn; if (!t) return;
  if (!t.thinking) {
    const el = document.createElement('details');
    el.className = 'thinking';
    el.innerHTML = `<summary>Thinking…</summary><div class="t-body"></div>`;
    t.root.appendChild(el);
    t.thinking = { el, body: el.querySelector('.t-body'), raw: '' };
  }
  t.thinking.raw += delta;
  t.thinking.body.textContent = t.thinking.raw;
  scrollToBottom();
}
function addToolCard(ev) {
  const t = state.turn; if (!t) return;
  finishText();
  if (t.thinking) t.thinking.el.querySelector('summary').textContent = 'Thought process';
  const card = document.createElement('div');
  card.className = 'tool' + (ev.risky ? ' risky' : '');
  card.dataset.id = ev.id;
  const iconCls = ev.tool.startsWith('browser_') ? ' globe' : '';
  card.innerHTML = `
    <div class="tool-head">
      <span class="tool-icon${iconCls}">${toolIcon(ev.tool)}</span>
      <span class="tool-name">${esc(ev.tool)}</span>
      <span class="tool-summary">${esc(summarizeArgs(ev.tool, ev.args))}</span>
      <span class="tool-meta">${ev.risky ? '<span class="badge amber">needs approval</span>' : ''}<span class="t-status"><span class="spinner"></span></span></span>
    </div>
    <div class="tool-body">
      <div class="tool-section"><h4>Arguments</h4><pre class="tool-args">${esc(prettyArgs(ev.args))}</pre></div>
      <div class="tool-section out hidden"><h4>Output</h4><pre class="tool-output"></pre></div>
    </div>`;
  card.querySelector('.tool-head').addEventListener('click', () => card.classList.toggle('open'));
  t.root.appendChild(card);
  t.tools.set(ev.id, card);
  scrollToBottom();
  return card;
}
function showApproval(ev) {
  const card = state.turn?.tools.get(ev.id); if (!card) return;
  card.classList.add('open');
  const ap = document.createElement('div');
  ap.className = 'approval';
  ap.innerHTML = `
    <div class="ap-title">${icons.shield} Approve this action?</div>
    <div class="ap-reason">${esc(ev.tool)} · ${esc(ev.reason || 'changes the system')}</div>
    <div class="ap-cmd">${esc(summarizeArgs(ev.tool, ev.args))}</div>
    <div class="ap-actions">
      <button class="btn success" data-ok="1">${icons.check} Approve</button>
      <button class="btn danger" data-ok="0">${icons.x} Deny</button>
    </div>`;
  ap.querySelectorAll('button').forEach((b) => b.addEventListener('click', () => {
    backend.resolveApproval(ev.id, b.dataset.ok === '1');
    ap.classList.add('resolved');
  }));
  card.querySelector('.tool-body').appendChild(ap);
  scrollToBottom(true);
}
function resolveApproval(ev) {
  const card = state.turn?.tools.get(ev.id); if (!card) return;
  const ap = card.querySelector('.approval');
  if (ap) {
    ap.classList.add('resolved');
    ap.querySelector('.ap-title').innerHTML = ev.approved ? `${icons.check} Approved` : `${icons.x} Denied`;
    ap.querySelector('.ap-title').style.color = ev.approved ? '#b8f3cc' : '#ffb3b3';
  }
  const badge = card.querySelector('.badge');
  if (badge) { badge.className = 'badge ' + (ev.approved ? 'green' : 'red'); badge.textContent = ev.approved ? 'approved' : 'denied'; }
  if (!ev.approved) card.classList.remove('open');
}
function fillToolResult(card, output, isError, durationMs) {
  card.classList.remove('risky');
  card.classList.add(isError ? 'error' : 'ok');
  const status = card.querySelector('.t-status');
  status.innerHTML = isError ? '<span class="badge red">error</span>' : (durationMs ? `${durationMs} ms` : '');
  if (!isError && !card.querySelector('.badge.red, .badge.green')) card.querySelector('.tool-icon').innerHTML = icons.check;
  const out = card.querySelector('.tool-section.out');
  out.classList.remove('hidden');
  out.querySelector('.tool-output').textContent = output || '(no output)';
}
function showToolResult(ev) {
  const card = state.turn?.tools.get(ev.id); if (!card) return;
  fillToolResult(card, ev.output, ev.isError, ev.durationMs);
  scrollToBottom();
}
function showError(text) {
  finishText();
  const el = document.createElement('div');
  el.className = 'msg error';
  el.innerHTML = `<div class="bubble">${esc(text)}</div>`;
  (state.turn?.root || els.thread).appendChild(el);
  scrollToBottom(true);
}

// Rebuild the transcript of a saved chat from its stored messages.
function renderHistory(messages) {
  clearThread();
  if (!messages?.length) { els.welcome.classList.remove('hidden'); return; }
  els.welcome.classList.add('hidden');
  let pendingCards = [];
  let n = 0;
  for (const m of messages) {
    if (m.role === 'user') {
      if (m.content.startsWith('[Scheduled task')) {
        startTurn();
        const b = document.createElement('div');
        b.className = 'bubble sched-banner';
        b.textContent = m.content.split('\n')[0].replace(/^\[|\]$/g, '');
        state.turn.root.appendChild(b);
        const p = document.createElement('div'); p.className = 'bubble md'; p.innerHTML = md(m.content.split('\n').slice(1).join('\n')); state.turn.root.appendChild(p);
      } else addUserMessage(m.content);
      state.turn = null;
    } else if (m.role === 'assistant') {
      if (!state.turn) startTurn();
      if (m.thinking) { appendThinking(m.thinking); state.turn.thinking.el.querySelector('summary').textContent = 'Thought process'; }
      if (m.content?.trim()) { appendText(m.content); finishText(); }
      pendingCards = [];
      for (const tc of m.tool_calls || []) {
        const id = 'hist-' + (n++);
        const card = addToolCard({ id, tool: tc.function.name, args: JSON.stringify(tc.function.arguments || {}), risky: false });
        pendingCards.push(card);
      }
    } else if (m.role === 'tool') {
      const card = pendingCards.shift();
      if (card) fillToolResult(card, m.content, /^Error:|^The user declined|^Tool call cancelled/.test(m.content || ''), 0);
    }
  }
  state.turn = null;
  requestAnimationFrame(() => scrollToBottom(true));
}

// ---------------- chats ----------------
async function refreshChats() {
  try {
    const list = await backend.listChats();
    if (!list?.length) { els.chatList.innerHTML = '<div class="empty-note">No saved chats yet</div>'; return; }
    els.chatList.innerHTML = list.slice(0, 60).map((c) => `
      <div class="chat-item ${c.id === state.chatId ? 'active' : ''}" data-id="${esc(c.id)}" title="${esc(c.title)}">
        <span class="c-icon">${c.source === 'schedule' || (c.title || '').startsWith('⏰') ? icons.clock : icons.chat}</span>
        <span class="c-title">${esc((c.title || 'Untitled').replace(/^⏰\s*/, ''))}</span>
        <span class="c-time">${timeAgo(c.updatedAt)}</span>
        <button class="c-del" title="Delete chat">${icons.trash}</button>
      </div>`).join('');
    els.chatList.querySelectorAll('.chat-item').forEach((item) => {
      item.addEventListener('click', () => openChat(item.dataset.id));
      item.querySelector('.c-del').addEventListener('click', async (e) => {
        e.stopPropagation();
        try { await backend.deleteChat(item.dataset.id); } catch (err) { toast(String(err)); return; }
        if (item.dataset.id === state.chatId) await newChat();
        refreshChats();
      });
    });
  } catch (e) { console.error(e); }
}
async function openChat(id) {
  closeModal();
  if (id === state.chatId) return;
  try {
    const view = await backend.loadChat(id);
    state.chatId = view.id;
    els.chatTitle.textContent = view.title || 'Chat';
    renderHistory(view.messages);
    setRunning(await backend.isRunning());
    refreshChats();
  } catch (e) { toast(String(e)); }
}
async function newChat() {
  const id = await backend.newChat();
  state.chatId = id;
  els.chatTitle.textContent = 'Chat';
  clearThread();
  els.welcome.classList.remove('hidden');
  setRunning(false);
  refreshChats();
}

// ---------------- schedules ----------------
const kindLabel = { startup: 'At app start', daily: 'Every day', once: 'Once' };
function schedWhen(t) {
  if (t.kind === 'daily') return `${kindLabel.daily} at ${t.time}`;
  if (t.kind === 'once') return `${kindLabel.once} · ${fmtWhen(t.at) || t.at || ''}`;
  return kindLabel.startup;
}
async function refreshSchedules() {
  try {
    state.schedules = (await backend.listSchedules()) || [];
    const active = state.schedules.filter((t) => t.enabled).length;
    els.schedCount.textContent = active;
    els.schedCount.classList.toggle('hidden', !active);
    if (state.modal === 'schedules') renderSchedulesModal();
  } catch (e) { console.error(e); }
}
function openSchedules(showForm = false) {
  openModal('schedules', 'Scheduled tasks', icons.clock, `<button class="btn sm primary" id="addSched">${icons.plus} New task</button>`);
  $('addSched').addEventListener('click', () => renderSchedulesModal(true));
  renderSchedulesModal(showForm);
}
function renderSchedulesModal(showForm = false) {
  const body = els.modalBody;
  const formOpen = showForm || !!body.querySelector('.sched-form');
  const draft = formOpen && body.querySelector('.sched-form') ? { prompt: $('sfPrompt')?.value, kind: $('sfKind')?.value, time: $('sfTime')?.value, when: $('sfWhen')?.value } : null;
  const list = state.schedules;
  body.innerHTML = `
    <div id="schedForm"></div>
    ${list.length ? `<div class="sched-list">${list.map((t) => `
      <div class="sched-item ${t.enabled ? '' : 'off'}" data-id="${esc(t.id)}">
        <span class="s-kind">${t.kind === 'startup' ? icons.power : icons.clock}</span>
        <div class="s-main">
          <div class="s-name">${esc(t.name)}</div>
          <div class="s-prompt">${esc(t.prompt)}</div>
          <div class="s-when">
            <span class="pill">${esc(schedWhen(t))}</span>
            ${t.enabled && t.nextRun && t.kind !== 'startup' ? `<span>next ${esc(fmtWhen(t.nextRun))}</span>` : ''}
            ${t.lastRun && new Date(t.lastRun).getFullYear() > 2000 ? `<span>· last ran ${esc(fmtWhen(t.lastRun))}</span>` : ''}
          </div>
        </div>
        <div class="s-actions">
          ${t.lastChat ? `<button class="icon-btn s-open" title="Open last run">${icons.external}</button>` : ''}
          <button class="icon-btn s-run" title="Run now">${icons.play}</button>
          <button class="icon-btn danger s-del" title="Delete">${icons.trash}</button>
          <label class="switch sm" title="${t.enabled ? 'Enabled' : 'Disabled'}"><input type="checkbox" class="s-toggle" ${t.enabled ? 'checked' : ''}><span></span></label>
        </div>
      </div>`).join('')}</div>`
    : `<div class="empty-state ${formOpen ? 'hidden' : ''}">
        <div class="es-icon">${icons.clock}</div>
        <h3>Nothing scheduled yet</h3>
        <p>Run a prompt when the app starts, every day at a set time, or once at a date. You can also just ask the chat: “every day at 9 summarize my unread emails”.</p>
        <button class="btn primary" id="esAdd">${icons.plus} Create a task</button>
      </div>`}
    <div class="modal-foot">
      <label class="toggle-row">
        <span class="t-text"><span class="t-title">Launch at login</span><span class="t-sub">The app must be running for tasks to fire. Startup tasks run a few seconds after it opens.</span></span>
        <span class="switch"><input type="checkbox" id="launchAtLogin" ${state.launchAtLogin ? 'checked' : ''}><span></span></span>
      </label>
      ${state.paths.schedules ? `<div class="path-note">Stored in <code>${esc(state.paths.schedules)}</code></div>` : ''}
    </div>`;
  body.querySelectorAll('.sched-item').forEach((item) => {
    const id = item.dataset.id;
    const t = state.schedules.find((s) => s.id === id);
    item.querySelector('.s-toggle').addEventListener('change', (e) => backend.setScheduleEnabled(id, e.target.checked).then(refreshSchedules));
    item.querySelector('.s-run').addEventListener('click', () => backend.runScheduleNow(id).then(() => toast(`Running “${t?.name}” now`)).catch((e) => toast(String(e))));
    item.querySelector('.s-del').addEventListener('click', () => backend.deleteSchedule(id).then(refreshSchedules));
    item.querySelector('.s-open')?.addEventListener('click', () => openChat(t.lastChat));
  });
  $('esAdd')?.addEventListener('click', () => renderSchedulesModal(true));
  $('launchAtLogin').addEventListener('change', (e) => {
    state.launchAtLogin = e.target.checked;
    backend.setLaunchAtLogin(e.target.checked).catch((err) => { toast(String(err)); e.target.checked = !e.target.checked; });
  });
  if (formOpen) showSchedForm(draft);
}
function showSchedForm(draft) {
  const host = $('schedForm'); if (!host) return;
  host.innerHTML = `
    <div class="sched-form">
      <label class="f-label" for="sfPrompt">What should the agent do?</label>
      <textarea class="input" id="sfPrompt" rows="3" placeholder="e.g. Open Gmail and summarize unread emails, then notify me.">${esc(draft?.prompt || '')}</textarea>
      <div class="row">
        <div class="field grow"><label class="f-label" for="sfKind">When</label>
          <select class="input" id="sfKind"><option value="daily">Every day at</option><option value="once">Once at</option><option value="startup">When the app starts</option></select></div>
        <div class="field" id="sfTimeField"><label class="f-label" for="sfTime">Time</label><input class="input" id="sfTime" type="time" value="${esc(draft?.time || '09:00')}"></div>
        <div class="field grow hidden" id="sfWhenField"><label class="f-label" for="sfWhen">Date and time</label><input class="input" id="sfWhen" placeholder="tomorrow 9am · 2026-11-01 18:00 · in 2 hours" value="${esc(draft?.when || '')}"></div>
      </div>
      <div class="form-error hidden" id="sfErr"></div>
      <div class="actions"><button class="btn sm ghost" id="sfCancel">Cancel</button><button class="btn sm primary" id="sfSave">${icons.check} Save task</button></div>
    </div>`;
  const kind = $('sfKind'), err = $('sfErr');
  if (draft?.kind) kind.value = draft.kind;
  const sync = () => { $('sfTimeField').classList.toggle('hidden', kind.value !== 'daily'); $('sfWhenField').classList.toggle('hidden', kind.value !== 'once'); };
  kind.onchange = sync; sync();
  $('sfCancel').onclick = () => { host.innerHTML = ''; els.modalBody.querySelector('.empty-state')?.classList.remove('hidden'); };
  $('sfSave').onclick = async () => {
    try {
      await backend.addSchedule('', $('sfPrompt').value, kind.value, $('sfTime').value, $('sfWhen').value);
      host.innerHTML = '';
      await refreshSchedules();
      toast('Task scheduled');
    } catch (e) { err.textContent = String(e); err.classList.remove('hidden'); }
  };
  els.modalBody.querySelector('.empty-state')?.classList.add('hidden');
  $('sfPrompt').focus();
}

// ---------------- settings ----------------
function openSettings() {
  openModal('settings', 'Settings', icons.gear);
  els.modalBody.innerHTML = `
    <section class="set-section">
      <h3>Connection</h3>
      <div class="status-card" id="stStatus">
        <span class="dot" id="stDot"></span>
        <div class="grow"><div class="st-title" id="stText">Checking…</div><div class="st-sub" id="stSub"></div></div>
        <button class="btn sm hidden" id="stStart">Start Ollama</button>
      </div>
      <div class="field"><label class="f-label" for="stUrl">Ollama URL</label>
        <div class="row"><input class="input grow" id="stUrl" value="${esc(state.settings?.ollamaUrl || 'http://localhost:11434')}" spellcheck="false"><button class="btn sm" id="stUrlSave">Apply</button></div></div>
    </section>
    <section class="set-section">
      <h3>Model</h3>
      <div class="field"><select class="input" id="stModel"><option>Loading…</option></select>
        <div class="f-hint">Only models with tool support can run actions. Pull more with <code>ollama pull &lt;name&gt;</code>.</div></div>
    </section>
    <section class="set-section">
      <h3>Behaviour</h3>
      <label class="toggle-row">
        <span class="t-text"><span class="t-title">Auto-approve actions</span><span class="t-sub">Run risky commands (writes, deletes, installs, browser clicks) without asking first</span></span>
        <span class="switch warn"><input type="checkbox" id="stAuto" ${state.settings?.autoApprove ? 'checked' : ''}><span></span></span>
      </label>
      <label class="toggle-row">
        <span class="t-text"><span class="t-title">Thinking mode</span><span class="t-sub">Lets the model reason before answering. Slower, more careful.</span></span>
        <span class="switch"><input type="checkbox" id="stThink" ${state.settings?.think ? 'checked' : ''}><span></span></span>
      </label>
    </section>
    <section class="set-section">
      <h3>Tools available to the agent</h3>
      <div class="tool-chips">${state.tools.map((n) => `<span class="chip">${esc(n)}</span>`).join('') || '<span class="empty-note">Loading…</span>'}</div>
    </section>
    <section class="set-section">
      <h3>Storage</h3>
      <div class="kv">${Object.entries(state.paths).map(([k, v]) => `<div class="kv-row"><span>${esc(k)}</span><code>${esc(v)}</code></div>`).join('') || '<div class="empty-note">—</div>'}</div>
    </section>`;
  $('stModel').addEventListener('change', (e) => saveSettings({ model: e.target.value }));
  $('stAuto').addEventListener('change', (e) => saveSettings({ autoApprove: e.target.checked }));
  $('stThink').addEventListener('change', (e) => saveSettings({ think: e.target.checked }));
  const applyUrl = async () => {
    const url = $('stUrl').value.trim();
    if (!url || url === state.settings.ollamaUrl) return;
    await saveSettings({ ollamaUrl: url });
    state.models = [];
    await refreshStatus();
    await refreshModels();
  };
  $('stUrlSave').addEventListener('click', applyUrl);
  $('stUrl').addEventListener('keydown', (e) => { if (e.key === 'Enter') applyUrl(); });
  $('stStart').addEventListener('click', async () => {
    try { await backend.startOllama(); $('stSub').textContent = 'Starting…'; setTimeout(refreshStatus, 2500); } catch (e) { $('stSub').textContent = String(e); }
  });
  syncSettingsUI();
}
// Push current status/models into whichever settings controls are mounted.
function syncSettingsUI() {
  if (state.modal !== 'settings') return;
  const s = state.status;
  const dot = $('stDot'); if (!dot) return;
  dot.className = 'dot ' + (s ? (s.connected ? 'ok' : 'bad') : '');
  $('stText').textContent = !s ? 'Checking…' : s.connected ? `Ollama ${s.ollamaVersion} connected` : 'Ollama is not running';
  $('stSub').textContent = !s ? '' : s.connected ? `${state.models.length} model${state.models.length === 1 ? '' : 's'} installed` : (s.error || 'Start it to begin chatting.');
  $('stStart').classList.toggle('hidden', !s || s.connected);
  const sel = $('stModel');
  sel.innerHTML = state.models.map((m) =>
    `<option value="${esc(m.name)}" ${m.supportsTools ? '' : 'disabled'}>${esc(m.name)} · ${esc(m.parameterSize || m.size)}${m.supportsTools ? '' : ' (no tool support)'}</option>`).join('')
    || `<option value="">${s?.connected ? 'No models installed' : 'Ollama unavailable'}</option>`;
  if (state.settings?.model) sel.value = state.settings.model;
}

// ---------------- event wiring ----------------
backend.onEvent((ev) => {
  switch (ev.type) {
    case 'chats_changed': refreshChats(); return;
    case 'schedules_changed': refreshSchedules(); return;
    case 'chat_started': {
      const emptyCurrent = !els.thread.children.length && !state.running;
      if (emptyCurrent) { state.chatId = ev.chatId; els.chatTitle.textContent = '⏰ ' + ev.text; clearThread(); els.welcome.classList.add('hidden'); setRunning(true); setStatus('running scheduled task'); }
      else toast(`Scheduled task running: ${ev.text}`, 'View', () => openChat(ev.chatId));
      return;
    }
  }
  if (ev.chatId && ev.chatId !== state.chatId) return; // belongs to another conversation
  if (!state.turn && ev.type !== 'done') startTurn();
  switch (ev.type) {
    case 'status': setStatus(ev.text); break;
    case 'delta': appendText(ev.text); break;
    case 'thinking': appendThinking(ev.text); break;
    case 'tool_call': addToolCard(ev); break;
    case 'approval': showApproval(ev); break;
    case 'approval_resolved': resolveApproval(ev); break;
    case 'tool_result': showToolResult(ev); break;
    case 'error':
      if (ev.text === 'cancelled') { finishText(); setStatus(''); setRunning(false); state.turn = null; }
      else showError(ev.text);
      break;
    case 'done':
      finishText();
      if (state.turn?.thinking) state.turn.thinking.el.querySelector('summary').textContent = 'Thought process';
      setRunning(false);
      state.turn = null;
      break;
  }
});

async function send() {
  if (state.running) { backend.cancel(); return; }
  const text = els.input.value.trim();
  if (!text) return;
  if (state.status && !state.status.connected) { flashBanner(); return; }
  els.input.value = ''; autoResize();
  addUserMessage(text);
  if (els.chatTitle.textContent === 'Chat') els.chatTitle.textContent = text.length > 60 ? text.slice(0, 57) + '...' : text;
  setRunning(true);
  setStatus('thinking');
  try {
    await backend.sendMessage(text);
  } catch (err) {
    showError(String(err));
    setRunning(false);
  }
}

els.send.addEventListener('click', send);
els.input.addEventListener('keydown', (e) => {
  if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); send(); }
  if (e.key === 'Escape' && state.running) backend.cancel();
});
els.input.addEventListener('input', autoResize);
document.querySelectorAll('.suggestion').forEach((b) => b.addEventListener('click', () => {
  els.input.value = suggestions[+b.dataset.i].prompt; autoResize(); send();
}));
$('newChat').addEventListener('click', newChat);
$('navSched').addEventListener('click', () => openSchedules());
$('navSettings').addEventListener('click', openSettings);
$('sideStatus').addEventListener('click', openSettings);
els.modelPill.addEventListener('click', openSettings);
$('startOllama').addEventListener('click', async () => {
  try { await backend.startOllama(); setTimeout(refreshStatus, 2500); } catch (e) { els.bannerText.textContent = String(e); }
});
document.addEventListener('keydown', (e) => {
  if ((e.ctrlKey || e.metaKey) && e.key === ',') { e.preventDefault(); openSettings(); }
  if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'n') { e.preventDefault(); newChat(); }
});

// ---------------- settings & status ----------------
function flashBanner() {
  els.banner.classList.remove('hidden');
  els.banner.animate([{ transform: 'translateX(0)' }, { transform: 'translateX(-5px)' }, { transform: 'translateX(5px)' }, { transform: 'translateX(0)' }], { duration: 250 });
}
async function saveSettings(patch) {
  state.settings = { ...state.settings, ...patch };
  els.modelPill.textContent = state.settings.model || 'no model';
  try { await backend.saveSettings(state.settings); } catch (e) { console.error(e); toast(String(e)); }
}
async function refreshModels() {
  try {
    state.models = (await backend.listModels()) || [];
    if (!state.settings?.model || !state.models.some((m) => m.name === state.settings.model)) {
      const first = state.models.find((m) => m.supportsTools) || state.models[0];
      if (first) saveSettings({ model: first.name });
    }
  } catch (e) { state.models = []; }
  syncSettingsUI();
}
async function refreshStatus() {
  try {
    const s = await backend.getStatus();
    state.status = s;
    els.statusDot.className = 'dot ' + (s.connected ? 'ok' : 'bad');
    els.statusText.textContent = s.connected ? `Ollama ${s.ollamaVersion}` : 'Ollama not running';
    els.statusText.title = s.error || '';
    if (s.connected) {
      els.banner.classList.add('hidden');
      if (!state.models.length) await refreshModels();
    } else {
      els.bannerText.textContent = 'Cannot reach Ollama at ' + (state.settings?.ollamaUrl || 'localhost:11434') + '. Start it to begin chatting.';
      els.banner.classList.remove('hidden');
    }
    syncSettingsUI();
  } catch (e) { console.error(e); }
}

(async function init() {
  state.settings = await backend.getSettings();
  els.modelPill.textContent = state.settings.model || 'no model';
  try { state.tools = await backend.toolNames(); } catch {}
  try { state.paths = (await backend.paths()) || {}; } catch {}
  try { const cur = await backend.currentChat(); state.chatId = cur.id; if (cur.messages?.length) { els.chatTitle.textContent = cur.title || 'Chat'; renderHistory(cur.messages); } } catch {}
  try { state.launchAtLogin = await backend.getLaunchAtLogin(); } catch {}
  await Promise.all([refreshChats(), refreshSchedules(), refreshStatus()]);
  if (!state.models.length) await refreshModels();
  setInterval(refreshStatus, 15000);
  setInterval(refreshSchedules, 60000);
  els.input.focus();
})();
