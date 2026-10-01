export const APP_CSS = String.raw`:root {
  color-scheme: light dark;
  --bg: #f6f7f9; --panel: #ffffff; --text: #1b1f24; --muted: #5d6673; --line: #d9dde3;
  --accent: #2457d6; --accent-text: #ffffff; --danger: #b42318; --on: #e8eefc;
  --ok: #17663a; --warn: #8a5a00; --bad: #b42318;
}
@media (prefers-color-scheme: dark) {
  :root {
    --bg: #14171c; --panel: #1c2027; --text: #e6e9ee; --muted: #98a2b0; --line: #333a45;
    --accent: #6b93f5; --accent-text: #0d1220; --danger: #f08a82; --on: #232c44;
    --ok: #62c98c; --warn: #e0b04a; --bad: #f08a82;
  }
}
* { box-sizing: border-box; }
body { margin: 0; background: var(--bg); color: var(--text); font: 14px/1.5 system-ui, -apple-system, "Segoe UI", "PingFang SC", "Microsoft YaHei", sans-serif; }
h1 { font-size: 18px; margin: 0; }
h2 { font-size: 16px; margin: 0; }
h3 { font-size: 13px; margin: 18px 0 6px; color: var(--muted); font-weight: 600; }
.mono { font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; font-size: 12.5px; overflow-wrap: anywhere; }
.muted { color: var(--muted); }
.error { color: var(--danger); }
.top { display: flex; align-items: center; justify-content: space-between; padding: 10px 16px; background: var(--panel); border-bottom: 1px solid var(--line); }
.layout { display: grid; grid-template-columns: minmax(320px, 5fr) minmax(360px, 6fr); gap: 16px; padding: 16px; align-items: start; }
.list, .detail, .login { background: var(--panel); border: 1px solid var(--line); border-radius: 8px; padding: 14px; min-width: 0; }
.detail { position: sticky; top: 12px; max-height: calc(100vh - 24px); overflow: auto; }
.center { display: grid; place-items: center; min-height: 100vh; padding: 16px; }
.login { width: min(380px, 100%); display: grid; gap: 12px; }
button, input, select, textarea { font: inherit; color: var(--text); background: var(--bg); border: 1px solid var(--line); border-radius: 6px; padding: 6px 10px; }
textarea { width: 100%; resize: vertical; }
button { cursor: pointer; }
button:focus-visible, input:focus-visible, select:focus-visible, textarea:focus-visible, .row:focus-visible { outline: 2px solid var(--accent); outline-offset: 1px; }
button.primary { background: var(--accent); border-color: var(--accent); color: var(--accent-text); }
button.danger { border-color: var(--danger); color: var(--danger); }
.filters { display: flex; flex-wrap: wrap; gap: 8px; align-items: end; margin-bottom: 10px; }
.f { display: grid; gap: 2px; font-size: 12px; color: var(--muted); }
.f input, .f select { width: 128px; }
.rows { display: grid; gap: 6px; }
.row { display: grid; gap: 3px; text-align: left; width: 100%; padding: 8px 10px; background: var(--panel); }
.row:hover { background: var(--on); }
.row.on { background: var(--on); border-color: var(--accent); }
.r1 { display: flex; flex-wrap: wrap; gap: 6px; align-items: center; }
.r2 { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.r3 { font-size: 12px; }
.more { margin-top: 10px; width: 100%; }
.tag, .badge { font-size: 12px; padding: 0 7px; border-radius: 10px; border: 1px solid var(--line); }
.badge.s-held, .badge.s-needs_info { color: var(--warn); border-color: var(--warn); }
.badge.s-rejected { color: var(--bad); border-color: var(--bad); }
.badge.s-received, .badge.s-recorded, .badge.s-fixed, .badge.s-answered { color: var(--ok); border-color: var(--ok); }
.dh { display: flex; flex-wrap: wrap; gap: 8px; align-items: center; }
.text { white-space: pre-wrap; overflow-wrap: anywhere; }
.body { background: var(--bg); border: 1px solid var(--line); border-radius: 6px; padding: 10px; max-height: 280px; overflow: auto; }
dl { display: grid; grid-template-columns: max-content 1fr; gap: 3px 14px; margin: 0; }
dt { color: var(--muted); }
dd { margin: 0; overflow-wrap: anywhere; }
.msg { border-left: 3px solid var(--line); padding: 2px 10px; margin: 8px 0; }
.msg.maint { border-left-color: var(--accent); }
.imgs { display: flex; flex-wrap: wrap; gap: 10px; }
figure { margin: 0; max-width: 220px; }
figure img { max-width: 100%; border: 1px solid var(--line); border-radius: 6px; display: block; }
.opts { display: flex; flex-wrap: wrap; gap: 10px; align-items: center; margin: 8px 0; }
.inline { display: inline-flex; gap: 6px; align-items: center; }
.actions { display: flex; flex-wrap: wrap; gap: 8px; }
@media (max-width: 820px) { .layout { grid-template-columns: 1fr; } .detail { position: static; max-height: none; } }
`;
