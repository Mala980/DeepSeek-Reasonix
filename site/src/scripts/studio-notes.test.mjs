import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { test } from 'node:test';
import { issueHref, parseStudioNotes, plainText } from '../lib/studio-notes.mjs';

const sample = '本版加入应用内反馈。\n\n## 新增\n\n- 终端支持 `/feedback` 并回复 (#11432, #11445)\n- 修复登录 #11421\n\n## 修复\n\n- 其他\n';

test('parses the fixed note shape into summary, sections, bullets and issue links', () => {
  const doc = parseStudioNotes(sample);
  assert.equal(doc.summary, '本版加入应用内反馈。');
  assert.deepEqual(doc.blocks.map((b) => b.type), ['paragraph', 'heading', 'list', 'heading', 'list']);
  const first = doc.blocks[2].items[0];
  assert.deepEqual(first.filter((t) => t.type === 'code').map((t) => t.value), ['/feedback']);
  assert.deepEqual(first.filter((t) => t.type === 'ref').map((t) => t.number), ['11432', '11445']);
  assert.deepEqual(doc.blocks[2].items[1].filter((t) => t.type === 'ref').map((t) => t.number), ['11421']);
  assert.equal(issueHref('11432'), 'https://github.com/esengine/DeepSeek-Reasonix/issues/11432');
  assert.throws(() => issueHref('1/../x'));
});

test('hostile input stays text and every source character is still present', () => {
  const long = '很长'.repeat(2000);
  const hostile = [
    '<script>alert(1)</script> 摘要',
    '',
    '## <img src=x onerror=alert(1)> 新增',
    '- [click](javascript:alert(1)) <b>bold</b> (#12, #34)',
    '- `<a href="javascript:x">` (#9x)',
    '### 未知标题',
    '1. 有序项',
    `- ${long}`,
    '- 悬空反引号 ` 与 (#7',
  ].join('\n');
  const doc = parseStudioNotes(hostile);
  const tokens = [];
  const visit = (inline) => tokens.push(...inline);
  for (const block of doc.blocks) {
    if (block.type === 'list') block.items.forEach(visit); else visit(block.inline);
  }
  for (const t of tokens) assert.ok(['text', 'code', 'ref'].includes(t.type));
  for (const t of tokens.filter((x) => x.type === 'ref')) assert.match(t.number, /^\d+$/);
  assert.ok(!tokens.some((t) => t.type === 'link' || t.href));
  const all = doc.blocks.flatMap((b) => (b.type === 'list' ? b.items.map(plainText) : [plainText(b.inline)])).join('\n');
  for (const needle of ['<script>alert(1)</script> 摘要', '<img src=x onerror=alert(1)> 新增', '[click](javascript:alert(1)) <b>bold</b>', '<a href="javascript:x">', '(#9x)', '未知标题', '1. 有序项', long, '(#7']) {
    assert.ok(all.includes(needle), needle.slice(0, 30));
  }
  assert.equal(doc.blocks.find((b) => plainText(b.inline ?? []).includes('未知标题')).type, 'paragraph');
});

test('the page renders notes as escaped text nodes only', async () => {
  const component = await readFile(new URL('../components/ChangelogPage.astro', import.meta.url), 'utf8');
  assert.doesNotMatch(component, /set:html|innerHTML|Fragment set/);
  assert.match(component, /issueHref\(/);
});
