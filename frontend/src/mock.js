// Scripted fake backend used only when the UI is opened outside Wails.
export function createMock() {
  let listeners = [];
  let settings = { ollamaUrl: 'http://localhost:11434', model: 'qwen3.5:2b', autoApprove: false, think: false, numCtx: 16384, temperature: 0.2 };
  let pending = {};
  let cancelled = false;
  let launchAtLogin = false;
  const now = Date.now();
  let chats = {
    'c-old': { id: 'c-old', title: 'Find my large downloads', source: 'user', updatedAt: new Date(now - 3600e3).toISOString(), messages: [
      { role: 'user', content: 'Find files over 100 MB in Downloads' },
      { role: 'assistant', content: '', tool_calls: [{ function: { name: 'run_shell', arguments: { command: 'find ~/Downloads -size +100M' } } }] },
      { role: 'tool', tool_name: 'run_shell', content: '/home/me/Downloads/ubuntu.iso\n/home/me/Downloads/video.mp4' },
      { role: 'assistant', content: 'Two files are over 100 MB:\n\n| File | Size |\n|---|---|\n| `ubuntu.iso` | 4.7 GB |\n| `video.mp4` | 820 MB |' },
    ] },
    'c-sched': { id: 'c-sched', title: '⏰ Morning email summary', source: 'schedule', updatedAt: new Date(now - 7200e3).toISOString(), messages: [
      { role: 'user', content: '[Scheduled task "Morning email summary"]\nOpen Gmail and summarize unread emails.' },
      { role: 'assistant', content: 'You have 3 unread emails: an invoice from AWS, a calendar invite from Priya for 3 pm, and a newsletter.' },
    ] },
  };
  let current = { id: 'c-new', title: '', source: 'user', messages: [] };
  let schedules = [
    { id: 't1', name: 'Morning email summary', prompt: 'Open Gmail and summarize unread emails, then notify me.', kind: 'daily', time: '08:30', enabled: true, nextRun: new Date(now + 36e5 * 14).toISOString() },
    { id: 't2', name: 'Boot health check', prompt: 'Run a system health check and notify me if disk is above 90%.', kind: 'startup', enabled: true },
  ];
  const emit = (ev) => listeners.forEach((l) => l(ev));
  const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
  const stream = async (text) => {
    for (const w of text.split(/(?<=\s)/)) {
      if (cancelled) return;
      emit({ type: 'delta', chatId: current.id, text: w });
      await sleep(28);
    }
  };
  const summaries = () => Object.values(chats).sort((a, b) => b.updatedAt.localeCompare(a.updatedAt)).map((c) => ({ id: c.id, title: c.title, source: c.source, updatedAt: c.updatedAt, count: c.messages.length }));

  async function run(text) {
    cancelled = false;
    const id = current.id;
    current.messages.push({ role: 'user', content: text });
    emit({ type: 'status', chatId: id, text: 'thinking' });
    await sleep(500);
    emit({ type: 'thinking', chatId: id, text: 'The user wants disk usage. I should call system_info first, then summarise. ' });
    await sleep(400);
    const id1 = 'call-1-' + Date.now();
    emit({ type: 'tool_call', chatId: id, id: id1, tool: 'system_info', args: '{}', risky: false });
    await sleep(900);
    emit({ type: 'tool_result', chatId: id, id: id1, tool: 'system_info', durationMs: 842, output: 'MEMORY: 22.1 GB used of 30.0 GB total (74% used)\nDISK /: 78.2 GB used of 93.3 GB total (84% used), 15.1 GB free' });
    await stream('Your root disk is **84% full** with 15 GB free. I can clear the apt cache to recover some space. ');
    const id2 = 'call-2-' + Date.now();
    const args = JSON.stringify({ command: 'sudo apt-get clean' });
    emit({ type: 'tool_call', chatId: id, id: id2, tool: 'run_shell', args, risky: true, reason: 'requires elevated privileges' });
    emit({ type: 'approval', chatId: id, id: id2, tool: 'run_shell', args, reason: 'requires elevated privileges' });
    emit({ type: 'status', chatId: id, text: 'waiting for approval' });
    const ok = await new Promise((resolve) => (pending[id2] = resolve));
    emit({ type: 'approval_resolved', chatId: id, id: id2, approved: ok });
    if (ok) {
      await sleep(800);
      emit({ type: 'tool_result', chatId: id, id: id2, tool: 'run_shell', durationMs: 731, output: '(command succeeded with no output)' });
      await stream('\n\nDone. The apt cache is cleared.');
    } else {
      emit({ type: 'tool_result', chatId: id, id: id2, tool: 'run_shell', isError: true, output: 'The user declined this action.' });
      await stream('\n\nOkay, I left it alone.');
    }
    current.messages.push({ role: 'assistant', content: 'done' });
    current.title = current.title || text.slice(0, 50);
    current.updatedAt = new Date().toISOString();
    chats[current.id] = current;
    emit({ type: 'done', chatId: id });
    emit({ type: 'chats_changed' });
  }

  return {
    sendMessage: async (text) => { run(text); },
    cancel: async () => { cancelled = true; Object.values(pending).forEach((r) => r(false)); pending = {}; emit({ type: 'error', chatId: current.id, text: 'cancelled' }); },
    isRunning: async () => false,
    resolveApproval: async (id, ok) => { pending[id]?.(ok); delete pending[id]; },
    newChat: async () => { if (!current.messages.length) return current.id; current = { id: 'c-' + Date.now(), title: '', source: 'user', messages: [] }; return current.id; },
    currentChat: async () => current,
    listChats: async () => summaries(),
    loadChat: async (id) => { current = chats[id]; return current; },
    deleteChat: async (id) => { delete chats[id]; if (current.id === id) current = { id: 'c-' + Date.now(), title: '', source: 'user', messages: [] }; },
    listSchedules: async () => schedules,
    addSchedule: async (name, prompt, kind, time, when) => { const t = { id: 't' + Date.now(), name: name || prompt.slice(0, 30), prompt, kind, time, enabled: true, nextRun: kind === 'startup' ? null : new Date(now + 864e5).toISOString() }; schedules.push(t); emit({ type: 'schedules_changed' }); return t; },
    setScheduleEnabled: async (id, en) => { const t = schedules.find((s) => s.id === id); if (t) t.enabled = en; emit({ type: 'schedules_changed' }); },
    deleteSchedule: async (id) => { schedules = schedules.filter((s) => s.id !== id); emit({ type: 'schedules_changed' }); },
    runScheduleNow: async (id) => { const t = schedules.find((s) => s.id === id); emit({ type: 'chat_started', chatId: 'c-run-' + Date.now(), text: t?.name, tool: 'schedule' }); },
    getLaunchAtLogin: async () => launchAtLogin,
    setLaunchAtLogin: async (v) => { launchAtLogin = v; },
    listModels: async () => [
      { name: 'qwen3.5:2b', size: '2.7 GB', parameterSize: '1.9B', supportsTools: true },
      { name: 'llama3.2:3b', size: '2.0 GB', parameterSize: '3.2B', supportsTools: true },
      { name: 'gemma3:4b', size: '3.3 GB', parameterSize: '4.3B', supportsTools: false },
    ],
    getStatus: async () => ({ connected: true, ollamaVersion: '0.40 (mock)', model: settings.model }),
    getSettings: async () => settings,
    saveSettings: async (s) => { settings = s; },
    startOllama: async () => {},
    toolNames: async () => ['browser_click', 'browser_open', 'browser_read', 'browser_type', 'delete_scheduled_task', 'find_files', 'folder_sizes', 'list_directory', 'list_scheduled_tasks', 'notify', 'open', 'read_file', 'run_shell', 'schedule_task', 'system_info', 'trash', 'write_file'],
    paths: async () => ({ chats: '~/.local/share/localagent/chats', schedules: '~/.config/localagent/schedules.json' }),
    onEvent: (cb) => { listeners.push(cb); },
  };
}
