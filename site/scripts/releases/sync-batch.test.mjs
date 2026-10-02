import assert from 'node:assert/strict';
import { test } from 'node:test';
import { batchBranch, decideBatch } from './sync-batch.mjs';

test('a missing batch branch starts a fresh sync', () => {
  assert.equal(decideBatch({ branchExists: false, openPullRequests: 0, commitsAhead: 0 }).action, 'sync');
});

test('an open review is left alone with a clear notice', () => {
  const d = decideBatch({ branchExists: true, openPullRequests: 1, commitsAhead: 3 });
  assert.equal(d.action, 'skip');
  assert.match(d.notice, /open review already exists/);
});

test('a pushed branch without a review gets one opened from the existing branch', () => {
  const d = decideBatch({ branchExists: true, openPullRequests: 0, commitsAhead: 1 });
  assert.equal(d.action, 'open-pr');
});

test('a stale branch with nothing ahead is skipped and named for deletion', () => {
  const d = decideBatch({ branchExists: true, openPullRequests: 0, commitsAhead: 0 });
  assert.equal(d.action, 'skip');
  assert.ok(d.notice.includes(`delete the branch ${batchBranch}`));
});
