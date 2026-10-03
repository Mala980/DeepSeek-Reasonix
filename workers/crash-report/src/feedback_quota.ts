import type { Env } from "./env";
import { RESERVED_SHARE } from "./feedback_types";
import { windowDetails, type FeedbackLimit, type LimitDetails } from "./feedback_http";
import { ipPrefix } from "./feedback_crypto";

export async function ipLimited(env: Env, ip: string, trusted: boolean): Promise<boolean> {
  return !trusted && !!env.FEEDBACK_LIMITER && !(await env.FEEDBACK_LIMITER.limit({ key: ipPrefix(ip) })).success;
}

// Atomically takes one unit from a fixed-window counter; false once `limit` is reached.
export async function take(env: Env, bucket: string, day: string, limit: number): Promise<boolean> {
  if (limit <= 0) return false;
  const row = await env.DB.prepare(
    "INSERT INTO feedback_quota (bucket, n, day) VALUES (?, 1, ?) ON CONFLICT (bucket) DO UPDATE SET n = n + 1 WHERE n < ? RETURNING n",
  )
    .bind(bucket, day, limit)
    .first<{ n: number }>();
  return row !== null;
}

export interface QuotaKeys {
  ipKey: string;
  installHash: string;
}

export interface QuotaLimits {
  globalDaily: number;
  ipHourly: number;
  installHourly: number;
  installDaily: number;
  trusted: boolean;
}

export type Admission = { kind: "ok" } | { kind: "limited" | "busy"; details: LimitDetails };

function callerSteps(keys: QuotaKeys, now: Date, limits: QuotaLimits): [string, number, FeedbackLimit][] {
  const day = now.toISOString().slice(0, 10);
  const hour = now.toISOString().slice(0, 13);
  return [
    ...(!limits.trusted ? [[`ip:${keys.ipKey}:${hour}`, limits.ipHourly, "ip_hourly"] as [string, number, FeedbackLimit]] : []),
    [`ih:${keys.installHash}:${hour}`, limits.installHourly, "install_hourly"],
    [`id:${keys.installHash}:${day}`, limits.installDaily, "install_daily"],
  ];
}

// Concealed refusals use ordinary exhausted counters, or the first caller window;
// block expiry never enters admission metadata.
export async function concealedLimit(env: Env, keys: QuotaKeys, now: Date, limits: QuotaLimits): Promise<LimitDetails> {
  const steps = callerSteps(keys, now, limits);
  for (const [bucket, limit, code] of steps) {
    const row = await env.DB.prepare("SELECT n FROM feedback_quota WHERE bucket = ?").bind(bucket).first<{ n: number }>();
    if ((row?.n ?? 0) >= limit) return windowDetails(code, now);
  }
  return windowDetails(steps[0][2], now);
}

// Untrusted installs stop short of the global cap so the last tenth stays
// available to installs whose earlier reports were accepted.
export function untrustedShare(globalDaily: number): number {
  return Math.floor(globalDaily * (1 - RESERVED_SHARE));
}

// The caller's own limits are spent before the shared daily budget, and a refusal
// hands back every unit already taken, so a rejected request costs nothing.
export async function admit(env: Env, keys: QuotaKeys, now: Date, limits: QuotaLimits): Promise<Admission> {
  const day = now.toISOString().slice(0, 10);
  const steps: [string, number, FeedbackLimit][] = [
    ...callerSteps(keys, now, limits),
    [`g:${day}`, limits.trusted ? limits.globalDaily : untrustedShare(limits.globalDaily), "global_daily"],
  ];
  const taken: string[] = [];
  for (const [bucket, limit, code] of steps) {
    if (!(await take(env, bucket, day, limit))) {
      for (const b of taken) await env.DB.prepare("UPDATE feedback_quota SET n = n - 1 WHERE bucket = ? AND n > 0").bind(b).run();
      return { kind: code === "global_daily" ? "busy" : "limited", details: windowDetails(code, now) };
    }
    taken.push(bucket);
  }
  return { kind: "ok" };
}

// True only for the first caller of the day, so an exhausted budget alerts once.
export async function firstBusyOfDay(env: Env, now: Date): Promise<boolean> {
  const day = now.toISOString().slice(0, 10);
  return take(env, `alert:${day}`, day, 1);
}
