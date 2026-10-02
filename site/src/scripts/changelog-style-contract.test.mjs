import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { test } from "node:test";

const css = await readFile(new URL("../styles/changelog.css", import.meta.url), "utf8");
const component = await readFile(new URL("../components/ChangelogPage.astro", import.meta.url), "utf8");
const versionPage = await readFile(new URL("../pages/changelog/[version].astro", import.meta.url), "utf8");

test("historical changelog styling remains readable", () => {
  assert.doesNotMatch(css, /var\(--(?:muted|paper)\)/);
  assert.match(css, /\.release-channel\s*\{/);
});

test("every release uses the version-ledger hero", () => {
  assert.doesNotMatch(component, /isThematicTitle|isThematic/);
  assert.doesNotMatch(component, /release-hero--compact|release-hero__actions|<blockquote>/);
  assert.match(component, /<header class="release-hero">/);
  assert.match(component, /<h1>Reasonix\{isStudio \? ' Studio' : ''\} v\{release\.version\}<\/h1>/);
  assert.match(component, /<p class="release-lede">/);
  assert.doesNotMatch(css, /\.release-hero--compact|\.release-hero__actions|\.release-hero blockquote/);
});

test("targeted releases split Desktop and CLI updates without removing legacy sections", () => {
  assert.match(component, /release\.targetingVersion === 1/);
  assert.match(component, /id=\{target\.id\}/);
  assert.match(component, /Desktop updates/);
  assert.match(component, /CLI updates/);
  assert.match(component, /CLI users may skip this version if preferred/);
  assert.match(component, /<section id="highlights"/);
  assert.match(css, /\.release-target-section__summary/);
  assert.match(css, /\.release-item-targets/);
});

test("changelog has one official navigation and marks archives noindex", () => {
  assert.doesNotMatch(component, /release-channel-tabs/);
  assert.match(component, /Historical archive/);
  assert.match(component, /noindex=\{isPreview\}/);
});

test("reviewed exact-version routes redirect safely until their publication marker exists", () => {
  assert.match(versionPage, /publishedReleases/);
  assert.match(versionPage, /publishedVersions\.has\(release\.version\)/);
  assert.match(versionPage, /if \(!published\)/);
  assert.match(versionPage, /return Astro\.redirect/);
  assert.match(versionPage, /release\.product === 'studio' \? '\/changelog\/' : '\/changelog\/cli\/'/);
});

test('product navigation exposes both lines and labels original Chinese Studio notes', () => {
  assert.match(component, /aria-label="Products"/);
  assert.match(component, /\/changelog\/cli\//);
  assert.match(component, /lang="zh"/);
  assert.match(component, /Chinese source notes/);
});

test('/changelog/ is the Studio default and the CLI tab has its own stable URL', async () => {
  const read = (name) => readFile(new URL(`../pages/changelog/${name}`, import.meta.url), 'utf8');
  const studio = await readFile(new URL('../components/StudioChangelog.astro', import.meta.url), 'utf8');
  assert.match(studio, /canonical=\{new URL\(`\$\{base\}\$\{releasePath\(latest\.version\)\}`, Astro\.site\)\.href\}/);
  for (const name of ['index.astro', 'studio.astro']) assert.match(await read(name), /<StudioChangelog \/>/);
  const cli = await read('cli.astro');
  assert.match(cli, /latestStableRelease/);
  assert.match(cli, /canonical=\{canonical\}/);
  for (const name of ['stable.astro', 'preview.astro']) assert.match(await read(name), /Astro\.redirect\('\/changelog\/cli\/', 301\)/);
  assert.match(component, /href=\{`\$\{base\}\/changelog\/cli\/`\} aria-current=\{!isStudio/);
  assert.match(component, /href=\{`\$\{base\}\/changelog\/`\} aria-current=\{isStudio/);
});
