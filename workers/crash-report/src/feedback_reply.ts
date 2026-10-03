import type { Env } from "./env";
import { verifyInstall } from "./feedback_auth";
import { isBlocked, isTrusted } from "./feedback_blocks";
import { readCappedText } from "./feedback_body";
import { ipHash } from "./feedback_crypto";
import { jsonResponse, refuse, windowDetails } from "./feedback_http";
import { ipLimited, take } from "./feedback_quota";
import { ReplyBody } from "./feedback_schema";
import { MAX_REPLIES_PER_ITEM, MAX_REPLY_BYTES, REPLIES_PER_INSTALL_HOURLY, TRUSTED_REPLIES_PER_INSTALL_HOURLY } from "./feedback_types";
import { scrubSensitiveText } from "./scrub";

const REPLYABLE = ["needs_info", "answered", "recorded", "in_progress"];
const REPLYABLE_SQL = `(${REPLYABLE.map((s) => `'${s}'`).join(",")})`;

export async function handleUserReply(request: Request, env: Env, receipt: string): Promise<Response> {
  const who = await verifyInstall(request, env);
  if (who instanceof Response) return who;
  const secret = env.FEEDBACK_TOKEN_SECRET ?? "";
  const ip = request.headers.get("cf-connecting-ip") ?? "unknown";
  const now = new Date();
  const trusted = await isTrusted(env, who.installHash, now);
  const limit = trusted ? TRUSTED_REPLIES_PER_INSTALL_HOURLY : REPLIES_PER_INSTALL_HOURLY;
  const hour = now.toISOString().slice(0, 13);
  const bucket = `rh:${who.installHash}:${hour}`;
  const details = windowDetails("reply_hourly", now);
  const blocked = await isBlocked(env, [`install:${who.installHash}`, `ip:${await ipHash(secret, ip)}`], now);
  if (await ipLimited(env, ip, trusted)) return refuse("feedback.rate_limited", "reply limit reached", windowDetails("ip_hourly", now));
  if (blocked) {
    const row = await env.DB.prepare("SELECT n FROM feedback_quota WHERE bucket = ?").bind(bucket).first<{ n: number }>();
    return refuse("feedback.rate_limited", "reply limit reached", trusted || (row?.n ?? 0) >= limit ? details : windowDetails("ip_hourly", now));
  }
  const text = await readCappedText(request, MAX_REPLY_BYTES * 2 + 512);
  if (text === null) return refuse("feedback.too_large", "reply too large");
  let raw: unknown;
  try {
    raw = JSON.parse(text);
  } catch {
    return refuse("feedback.invalid", "body is not valid JSON");
  }
  const parsed = ReplyBody.safeParse(raw);
  if (!parsed.success) return refuse("feedback.invalid", "reply must be 1-4096 bytes of text");

  if (!(await take(env, bucket, now.toISOString().slice(0, 10), limit))) return refuse("feedback.rate_limited", "reply limit reached", details);
  // One transaction decides ownership, status and the per-item cap, stores the
  // reply and moves needs_info back to held, so none of it can land half-done.
  // A reply to a question is already in front of the maintainer through the
  // triage queue; one to an issue-less report waits in the maintainer's queue (2).
  const at = now.toISOString();
  const [res] = await env.DB.batch([
    env.DB.prepare(
      `INSERT INTO feedback_replies (receipt, author, body, handled, created_at)
       SELECT receipt, 'user', ?, CASE status WHEN 'needs_info' THEN 1 WHEN 'answered' THEN 2 ELSE 0 END, ? FROM feedback
       WHERE receipt = ? AND install_hash = ? AND status IN ${REPLYABLE_SQL}
         AND (SELECT COUNT(*) FROM feedback_replies WHERE receipt = ? AND author = 'user') < ?`,
    ).bind(scrubSensitiveText(parsed.data.body), at, receipt, who.installHash, receipt, MAX_REPLIES_PER_ITEM),
    env.DB.prepare(
      `UPDATE feedback SET status = CASE WHEN status = 'needs_info' THEN 'held' ELSE status END, updated_at = ?
       WHERE receipt = ? AND install_hash = ? AND changes() > 0`,
    ).bind(at, receipt, who.installHash),
  ]);
  if ((res.meta?.changes ?? 0) === 0) {
    await env.DB.prepare("UPDATE feedback_quota SET n = n - 1 WHERE bucket = ? AND n > 0").bind(bucket).run();
    const row = await env.DB.prepare(`SELECT status FROM feedback WHERE receipt = ? AND install_hash = ?`).bind(receipt, who.installHash).first<{ status: string }>();
    const replyable = row !== null && REPLYABLE.includes(row.status);
    return replyable ? refuse("feedback.reply_limit", "reply limit reached for this report", windowDetails("reply_item", now)) : refuse("feedback.not_replyable", "this report cannot take replies");
  }
  const replyId = res.meta?.last_row_id ?? 0;
  return jsonResponse({ replyId }, 201);
}
