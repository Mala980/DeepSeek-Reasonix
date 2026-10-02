#!/usr/bin/env node
import { execFileSync } from 'node:child_process';
import { appendFile } from 'node:fs/promises';
import { resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

export const batchBranch = 'website-release-notes/sync';

export function decideBatch({ branchExists, openPullRequests, commitsAhead }) {
  if (!branchExists) return { action: 'sync', notice: '' };
  if (openPullRequests > 0) {
    return { action: 'skip', notice: `An open review already exists for ${batchBranch}; merge or close it before the next batch.` };
  }
  if (commitsAhead > 0) {
    return { action: 'open-pr', notice: `${batchBranch} has no open review but is ahead of website; opening one from the existing branch without changing it.` };
  }
  return { action: 'skip', notice: `${batchBranch} exists with no open review and nothing ahead of website; delete the branch ${batchBranch} so the next run can start a fresh batch.` };
}

const run = (cmd, ...args) => execFileSync(cmd, args, { encoding: 'utf8' }).trim();

function inspect() {
  const exists = run('git', 'ls-remote', '--heads', 'origin', batchBranch) !== '';
  if (!exists) return { branchExists: false, openPullRequests: 0, commitsAhead: 0 };
  run('git', 'fetch', 'origin', `+refs/heads/${batchBranch}:refs/remotes/origin/${batchBranch}`, '+refs/heads/website:refs/remotes/origin/website');
  const openPullRequests = Number(run('gh', 'pr', 'list', '--base', 'website', '--head', batchBranch, '--state', 'open', '--json', 'number', '--jq', 'length'));
  const commitsAhead = Number(run('git', 'rev-list', '--count', `origin/website..origin/${batchBranch}`));
  return { branchExists: true, openPullRequests, commitsAhead };
}

async function main() {
  const decision = decideBatch(inspect());
  if (decision.notice) console.log(`::notice::${decision.notice}`);
  if (process.env.GITHUB_OUTPUT) await appendFile(process.env.GITHUB_OUTPUT, `action=${decision.action}\n`);
  if (process.env.GITHUB_STEP_SUMMARY && decision.notice) await appendFile(process.env.GITHUB_STEP_SUMMARY, `${decision.notice}\n`);
  console.log(`action=${decision.action}`);
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  main().catch((error) => { console.error(error.message); process.exitCode = 1; });
}
