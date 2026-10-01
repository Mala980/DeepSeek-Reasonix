import type { Env } from "./env";
import { listLimit } from "./feedback_admin_store";
import { installHash } from "./feedback_crypto";
import { jsonResponse, refuse } from "./feedback_http";
import { replyCounts } from "./feedback_read";
import { CATEGORIES, STATUSES, type FeedbackRow, type StoredAttachment } from "./feedback_types";

const SNIPPET_CHARS = 120;
const HASH_PREFIX = /^[0-9a-f]{8,64}$/;
const INSTALL_ID = /^[A-Za-z0-9_-]{16,64}$/;
const MAX_FILTER_CHARS = 80;

type ListRow = { id: number } & Pick<FeedbackRow, "receipt" | "install_hash" | "category" | "display_name" | "env_json" | "attachments_json" | "status" | "created_at" | "updated_at"> & { snippet: string };

// `%`, `_` and the escape itself are literal in a user-typed substring.
function likeEscape(s: string): string {
  return s.replace(/[\\%_]/g, (c) => `\\${c}`);
}

function device(env: Record<string, string>): string {
  return [env.os, env.osVersion, env.arch].filter(Boolean).join(" ");
}

function parseOr<T>(json: string, fallback: T): T {
  try {
    return JSON.parse(json) as T;
  } catch {
    return fallback;
  }
}

// Newest first, paged by the row id of the last item seen (`before`). The contact
// and the full text stay out of the list; the detail endpoint carries them.
export async function listFeedback(env: Env, url: URL): Promise<Response> {
  const q = url.searchParams;
  const where: string[] = [];
  const binds: unknown[] = [];
  const status = q.get("status");
  if (status) {
    if (!(STATUSES as readonly string[]).includes(status)) return refuse("feedback.invalid", "unknown status");
    where.push("status = ?");
    binds.push(status);
  }
  const category = q.get("category");
  if (category) {
    if (!(CATEGORIES as readonly string[]).includes(category)) return refuse("feedback.invalid", "unknown category");
    where.push("category = ?");
    binds.push(category);
  }
  const version = (q.get("version") ?? "").trim();
  if (version) {
    if (version.length > MAX_FILTER_CHARS) return refuse("feedback.invalid", "version filter too long");
    where.push("json_extract(env_json, '$.version') = ?");
    binds.push(version);
  }
  const nickname = (q.get("nickname") ?? "").trim();
  if (nickname) {
    if (nickname.length > MAX_FILTER_CHARS) return refuse("feedback.invalid", "nickname filter too long");
    where.push("display_name LIKE ? ESCAPE '\\'");
    binds.push(`%${likeEscape(nickname)}%`);
  }
  const install = (q.get("install") ?? "").trim();
  if (install) {
    const clauses: string[] = [];
    if (HASH_PREFIX.test(install.toLowerCase())) {
      clauses.push("install_hash LIKE ? ESCAPE '\\'");
      binds.push(`${likeEscape(install.toLowerCase())}%`);
    }
    if (INSTALL_ID.test(install) && env.FEEDBACK_TOKEN_SECRET) {
      clauses.push("install_hash = ?");
      binds.push(await installHash(env.FEEDBACK_TOKEN_SECRET, install));
    }
    if (clauses.length === 0) return refuse("feedback.invalid", "install must be a hash prefix (8+ hex) or an install id");
    where.push(`(${clauses.join(" OR ")})`);
  }
  const beforeRaw = q.get("before");
  if (beforeRaw) {
    const before = Number(beforeRaw);
    if (!Number.isInteger(before) || before < 1) return refuse("feedback.invalid", "before must be a positive integer");
    where.push("id < ?");
    binds.push(before);
  }
  const limit = listLimit(url, 25, 100);
  const { results } = await env.DB.prepare(
    `SELECT id, receipt, install_hash, category, display_name, env_json, attachments_json, status, created_at, updated_at, substr(body, 1, ${SNIPPET_CHARS}) AS snippet FROM feedback${where.length ? ` WHERE ${where.join(" AND ")}` : ""} ORDER BY id DESC LIMIT ?`,
  )
    .bind(...binds, limit + 1)
    .all<ListRow>();
  const page = results.slice(0, limit);
  const counts = await replyCounts(env, page.map((r) => r.receipt));
  return jsonResponse({
    items: page.map((r) => {
      const e = parseOr<Record<string, string>>(r.env_json, {});
      const images = parseOr<StoredAttachment[]>(r.attachments_json, []).length;
      return {
        id: r.id,
        receipt: r.receipt,
        status: r.status,
        category: r.category,
        displayName: r.display_name,
        snippet: r.snippet,
        version: e.version ?? "",
        surface: e.surface ?? "",
        device: device(e),
        installHash: r.install_hash,
        hasImages: images > 0,
        imageCount: images,
        replyCount: counts.get(r.receipt) ?? 0,
        createdAt: r.created_at,
        updatedAt: r.updated_at,
      };
    }),
    nextBefore: results.length > limit ? page[page.length - 1].id : null,
  });
}
