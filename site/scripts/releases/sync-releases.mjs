#!/usr/bin/env node
import { execFileSync } from 'node:child_process';
import { appendFile } from 'node:fs/promises';
import { resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { parseStudioNotes } from '../../src/lib/studio-notes.mjs';
import { compareVersionsDesc, loadCatalog, upsertRelease } from './release-notes.mjs';

const repository = 'esengine/DeepSeek-Reasonix';
const root = resolve(import.meta.dirname, '../../..');
const git = (...args) => execFileSync('git', args, { cwd: root, encoding: 'utf8' }).trim();

export function missingReleases(published, catalog) {
  const existing = new Set(catalog.releases.map((r) => `${r.product || 'cli'}:${r.version}`));
  const missing = new Map();
  for (const release of published) {
    if (release.draft || !release.published_at) continue;
    const studio = release.tag_name.match(/^studio-v(2\.\d+\.\d+)$/);
    const cli = release.tag_name.match(/^(?:desktop-|npm-)?v(1\.\d+\.\d+)$/);
    if (!studio && (!cli || release.prerelease)) continue;
    const product = studio ? 'studio' : 'cli';
    const version = (studio || cli)[1];
    const key = `${product}:${version}`;
    if (existing.has(key)) continue;
    // Prefer the canonical CLI release over duplicate desktop/package tags.
    if (!missing.has(key) || release.tag_name === `v${version}`) {
      missing.set(key, { ...release, product, version });
    }
  }
  return [...missing.values()].sort((a, b) => -compareVersionsDesc(a.version, b.version));
}

export function studioRecord(release, markdown, sha) {
  if (!markdown?.trim()) throw new Error(`Missing Studio source for ${release.tag_name}`);
  const version = release.version || release.tag_name.replace(/^studio-v/, '');
  const title = { en: `Reasonix Studio v${version}`, zh: `Reasonix Studio v${version}` };
  const line = parseStudioNotes(markdown).summary || `Reasonix Studio v${version}`;
  const summary = { en: line, zh: line };
  return {
    product: 'studio', version, date: release.published_at.slice(0, 10),
    channel: release.prerelease ? 'prerelease' : 'stable',
    sourceNotes: { language: 'zh', markdown, sha, path: `release-notes/studio/${version}.md` },
    title, summary, surfaces: ['desktop'], guides: [],
    highlights: [{ kind: 'improved', title, body: summary }],
    changes: { new: [], improved: [], fixed: [] }, upgrade: [], risks: [], contributors: [],
    links: {
      github: `https://github.com/${repository}/releases/tag/studio-v${version}`,
      compare: `https://github.com/${repository}/blob/${sha}/release-notes/studio/${version}.md`,
      download: 'https://reasonix.io/#start',
    },
  };
}

export function describeFailure(error, secrets = [process.env.DEEPSEEK_API_KEY, process.env.GH_TOKEN]) {
  let text = String(error?.message ?? error).split('\n')[0];
  for (const secret of secrets) if (secret && secret.length > 3) text = text.split(secret).join('[redacted]');
  text = text.replace(/(Bearer\s+|sk-|ghp_|gho_|ghs_|github_pat_)\S+/g, '$1[redacted]').slice(0, 200);
  return `${error?.name || 'Error'}: ${text}`;
}

export async function syncCatalog({ catalog, published, studioSHA, readStudio, save, generateCLI, hasModelKey }) {
  const added = [], pending = [], failures = [];
  for (const release of missingReleases(published, catalog)) {
    const key = `${release.product}:${release.version}`;
    try {
      if (release.product === 'studio') {
        await save(studioRecord(release, await readStudio(release.version), studioSHA));
      } else {
        if (!hasModelKey) { pending.push(key); failures.push({ key, reason: 'no model key configured' }); continue; }
        const generated = await generateCLI(release);
        await save({ ...generated, status: 'published' });
      }
      added.push(key);
    } catch (error) {
      pending.push(key);
      failures.push({ key, reason: describeFailure(error) });
    }
  }
  return { added, pending, failures };
}

async function main() {
  const published = JSON.parse(execFileSync('gh', ['api', '--paginate', '--slurp', `repos/${repository}/releases?per_page=100`], { encoding: 'utf8', maxBuffer: 64 * 1024 * 1024 })).flat();
  const studioSHA = git('rev-parse', 'origin/studio');
  const result = await syncCatalog({
    catalog: await loadCatalog(), published, studioSHA,
    readStudio: async (version) => execFileSync('git', ['show', `${studioSHA}:release-notes/studio/${version}.md`], { cwd: root, encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] }),
    save: (release) => upsertRelease(release), hasModelKey: Boolean(process.env.DEEPSEEK_API_KEY),
    generateCLI: async (release) => {
      const releaseTag = git('rev-parse', '--verify', `refs/tags/${release.tag_name}^{commit}`);
      // Each range ends at the published tag, never today's product branch tip.
      execFileSync(process.execPath, ['site/scripts/releases/generate-release-notes.mjs', '--version', release.version, '--to', releaseTag, '--tag', release.tag_name, '--date', release.published_at.slice(0, 10)], { cwd: root, stdio: 'inherit' });
      return (await loadCatalog()).releases.find((r) => r.version === release.version && r.product !== 'studio');
    },
  });
  const summary = `## Website changelog sync\n\nAdded: ${result.added.join(', ') || 'none'}\n\nPending (retried next run): ${result.pending.join(', ') || 'none'}\n${result.failures.map((f) => `\n- ${f.key}: ${f.reason}`).join('')}\n`;
  console.log(summary);
  if (process.env.GITHUB_STEP_SUMMARY) await appendFile(process.env.GITHUB_STEP_SUMMARY, summary);
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  main().catch((error) => { console.error(error.message); process.exitCode = 1; });
}
