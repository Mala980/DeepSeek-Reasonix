import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { test } from 'node:test';

const workflow = await readFile(new URL('../../../.github/workflows/sync-site-changelog.yml', import.meta.url), 'utf8');
const validation = await readFile(new URL('../../../.github/workflows/release-notes.yml', import.meta.url), 'utf8');

test('sync executes trusted website content with pinned actions and no fork-triggered write job', () => {
  assert.doesNotMatch(workflow, /pull_request|pull_request_target/);
  assert.match(workflow, /ref: website/);
  assert.match(workflow, /^on:\n(?:\s+#.*\n)*\s+workflow_dispatch:\n/m);
  assert.doesNotMatch(workflow, /repository_dispatch|schedule:|cron:/);
  for (const line of workflow.split('\n').filter((line) => line.includes('uses:'))) {
    assert.match(line, /@[0-9a-f]{40}/);
  }
  assert.match(validation, /contents: read/);
  assert.doesNotMatch(validation, /contents: write|pull-requests: write/);
});

test('a pending review is preserved and generation cannot merge or deploy', () => {
  assert.match(workflow, /steps\.review\.outputs\.action == 'sync'/);
  assert.match(workflow, /steps\.review\.outputs\.action == 'open-pr'/);
  assert.match(workflow, /sync-batch\.mjs/);
  assert.match(workflow, /gh pr create --base website/);
  assert.doesNotMatch(workflow, /gh pr merge|deploy-pages|git push.*--force/);
  assert.match(workflow, /secrets\.DEEPSEEK_API_KEY/);
});

test('release-note review checks cover the synchronization tests and workflow changes', () => {
  assert.match(validation, /node --test site\/scripts\/releases\/\*\.test\.mjs/);
  assert.match(validation, /\.github\/workflows\/sync-site-changelog\.yml/);
});
