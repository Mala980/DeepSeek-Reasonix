const REPO_ISSUES = 'https://github.com/esengine/DeepSeek-Reasonix/issues/';
const TRAILING_REFS = /(\(?)(#\d{1,9}(?:\s*,\s*#\d{1,9})*)(\)?)\s*$/;

export function issueHref(digits) {
  if (!/^\d{1,9}$/.test(digits)) throw new Error('issue number must be digits only');
  return `${REPO_ISSUES}${digits}`;
}

function codeSpans(text) {
  const out = [];
  const pattern = /`([^`\n]+)`/g;
  let last = 0;
  for (const match of text.matchAll(pattern)) {
    if (match.index > last) out.push({ type: 'text', value: text.slice(last, match.index) });
    out.push({ type: 'code', value: match[1] });
    last = match.index + match[0].length;
  }
  if (last < text.length) out.push({ type: 'text', value: text.slice(last) });
  return out;
}

export function parseInline(line) {
  const trailing = line.match(TRAILING_REFS);
  const hasBoundary = trailing && (trailing.index === 0 || /\s/.test(line[trailing.index - 1]) || trailing[1] === '(');
  const insideCode = trailing && (line.slice(0, trailing.index).split('`').length - 1) % 2 === 1;
  if (!trailing || !hasBoundary || insideCode || (trailing[1] === '(') !== (trailing[3] === ')')) return codeSpans(line);
  const body = line.slice(0, trailing.index);
  const tokens = codeSpans(body);
  tokens.push({ type: 'text', value: trailing[1] });
  trailing[2].split(/(\s*,\s*)/).forEach((part) => {
    const ref = part.match(/^#(\d+)$/);
    tokens.push(ref ? { type: 'ref', number: ref[1], value: part } : { type: 'text', value: part });
  });
  if (trailing[3]) tokens.push({ type: 'text', value: trailing[3] });
  return tokens;
}

export function parseStudioNotes(markdown) {
  const blocks = [];
  let list = null;
  let summary = '';
  for (const raw of String(markdown ?? '').split(/\r?\n/)) {
    const line = raw.trimEnd();
    if (!line.trim()) { list = null; continue; }
    const heading = line.match(/^##\s+(\S.*)$/);
    const bullet = line.match(/^[-*]\s+(\S.*)$/);
    if (heading) {
      list = null;
      blocks.push({ type: 'heading', inline: parseInline(heading[1].trim()) });
    } else if (bullet) {
      if (!list) { list = { type: 'list', items: [] }; blocks.push(list); }
      list.items.push(parseInline(bullet[1]));
    } else {
      list = null;
      if (!summary && blocks.length === 0) summary = line.trim();
      blocks.push({ type: 'paragraph', inline: parseInline(line.trim()) });
    }
  }
  return { summary, blocks };
}

export function plainText(tokens) {
  return tokens.map((t) => t.value).join('');
}
