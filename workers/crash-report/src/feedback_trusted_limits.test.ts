// @ts-expect-error Node 22+ provides node:sqlite for boundary tests.
import { DatabaseSync } from "node:sqlite";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { Env } from "./env";
import { d1 } from "./feedback_testkit";
import { handleFeedbackRoute } from "./feedback_routes";
import { purgeStaleFeedback } from "./feedback_retention";
import migration from "../migrate-feedback.sql?raw";
import triage from "../migrate-feedback-triage.sql?raw";

const NOW = "2026-10-03T12:34:56.000Z";
const HOUR = "2026-10-03T13:00:00.000Z";
const DAY = "2026-10-04T00:00:00.000Z";
const ID = "install-aaaaaaaaaaaaaaaa";
const admin = { authorization: "Bearer fixture-admin" };
let db: DatabaseSync;
let env: Env;
let token: string;
let hash: string;
let receipt: string;
let seq = 0;
let ipAllowed: boolean;
const call = async (path: string, body?: unknown, headers: Record<string, string> = admin, method = body === undefined ? "GET" : "POST") =>
  await handleFeedbackRoute(new Request(`https://crash.test${path}`, { method, headers, ...(body === undefined ? {} : { body: JSON.stringify(body) }) }), env) as Response;
const submit = (extra = {}, key = `fixture-key-${++seq}`) => call("/v1/feedback", {
  installId: ID, idempotencyKey: key, body: "Neutral fixture", category: "bug", displayName: "Fixture", env: {}, ...extra,
}, { "x-install-token": token });
const act = (action: string, body = {}, r = receipt) => call(`/v1/admin/feedback/${r}/${action}`, body);
const trust = (days = 30) => db.prepare("INSERT OR REPLACE INTO feedback_trust VALUES (?,?,?)").run(hash, NOW, new Date(Date.parse(NOW) + days * 86400000).toISOString());
const quota = (prefix: string, n: number) => db.prepare("INSERT OR REPLACE INTO feedback_quota VALUES (?,?,?)").run(`${prefix}:${hash}:${prefix === "id" ? NOW.slice(0, 10) : NOW.slice(0, 13)}`, n, NOW.slice(0, 10));
const block = (expiry: string | null) => db.prepare("INSERT OR REPLACE INTO feedback_blocks VALUES (?,?,?,?)").run(`install:${hash}`, "fixture", NOW, expiry);
const params = async (r: Response) => (await r.json() as any).error.params;
const expectWindow = async (r: Response, limit: string, resetsAt: string | null) => {
  expect(r.status).toBe(429);
  const seconds = resetsAt === null ? null : Math.ceil((Date.parse(resetsAt) - Date.parse(NOW)) / 1000);
  expect(await params(r)).toEqual({ limit, resetsAt, retryAfterSeconds: seconds });
  expect(r.headers.get("retry-after")).toBe(seconds === null ? null : String(seconds));
};

beforeEach(async () => {
  vi.useFakeTimers();
  vi.setSystemTime(NOW);
  db = new DatabaseSync(":memory:");
  db.exec(migration);
  db.exec(triage);
  ipAllowed = true;
  env = { DB: d1(db), FEEDBACK_ENABLED: "true", FEEDBACK_TOKEN_SECRET: "fixture-secret", FEEDBACK_ADMIN_TOKEN: "fixture-admin",
    FEEDBACK_LIMITER: { limit: async () => ({ success: ipAllowed }) } } as unknown as Env;
  token = "";
  const first = await submit();
  const j = await first.json() as any;
  token = j.installToken;
  receipt = j.receipt;
  hash = (db.prepare("SELECT install_hash FROM feedback").get() as any).install_hash;
  db.exec("DELETE FROM feedback_quota");
});
afterEach(() => { vi.useRealTimers(); db.close(); });

describe("trusted admission and concealment", () => {
  it.each([[false, 3], [true, 12]])("enforces the server tier hourly limit (trusted=%s)", async (trusted, limit) => {
    if (trusted) trust();
    for (let i = 0; i < limit; i++) expect((await submit()).status).toBe(201);
    await expectWindow(await submit(), "install_hourly", HOUR);
    expect((db.prepare("SELECT n FROM feedback_quota WHERE bucket LIKE 'g:%'").get() as any).n).toBe(limit);
    vi.setSystemTime(HOUR);
    expect((await submit()).status).toBe(201);
  });

  it.each([[false, 10], [true, 60]])("enforces daily limits and refunds hourly quota (trusted=%s)", async (trusted, limit) => {
    if (trusted) trust();
    quota("id", limit);
    await expectWindow(await submit(), "install_daily", DAY);
    expect((db.prepare("SELECT n FROM feedback_quota WHERE bucket LIKE 'ih:%'").get() as any).n).toBe(0);
    vi.setSystemTime(DAY);
    expect((await submit()).status).toBe(201);
  });

  it("exempts trusted installs from both IP ceilings, but not IP blocks", async () => {
    trust();
    ipAllowed = false;
    const { ipHash } = await import("./feedback_crypto");
    const ip = await ipHash("fixture-secret", "unknown");
    db.prepare("INSERT INTO feedback_quota VALUES (?,10,'2026-10-03')").run(`ip:${ip}:2026-10-03T12`);
    expect((await submit()).status).toBe(201);
    expect((db.prepare("SELECT n FROM feedback_quota WHERE bucket LIKE 'ip:%'").get() as any).n).toBe(10);
    expect((await call("/v1/admin/feedback/block", { target: "ip:unknown", reason: "fixture" })).status).toBe(400);
    db.prepare("INSERT INTO feedback_blocks VALUES (?, 'fixture', ?, NULL)").run(`ip:${await ipHash("fixture-secret", "unknown")}`, NOW);
    await expectWindow(await submit(), "install_hourly", HOUR);
  });

  it("does not accept client trust claims or expired trust", async () => {
    quota("ih", 3);
    trust(-1);
    await expectWindow(await submit({ trusted: true, installTrusted: true }), "install_hourly", HOUR);
    const held = await (await call("/v1/admin/feedback/held")).json() as any;
    expect(held.items[0].installTrusted).toBe(false);
  });

  it.each([false, true])("preserves byte-identical bodies and headers for blocked and ordinary callers (trusted=%s)", async (trusted) => {
    if (trusted) trust();
    quota("ih", trusted ? 12 : 3);
    const ordinary = await submit();
    await expectWindow(ordinary.clone(), "install_hourly", HOUR);
    for (const expiry of [null, "2026-10-20T09:15:00.000Z"]) {
      block(expiry);
      const blocked = await submit();
      expect(blocked.status).toBe(ordinary.status);
      expect(await blocked.text()).toBe(await ordinary.clone().text());
      expect([...blocked.headers]).toEqual([...ordinary.headers]);
    }
    db.exec("DELETE FROM feedback_quota");
    quota("id", trusted ? 60 : 10);
    const blockedDay = await submit();
    db.exec("DELETE FROM feedback_blocks");
    const ordinaryDay = await submit();
    await expectWindow(ordinaryDay.clone(), "install_daily", DAY);
    expect(await blockedDay.text()).toBe(await ordinaryDay.text());
    expect([...blockedDay.headers]).toEqual([...ordinaryDay.headers]);
  });

  it("conceals block expiry on the IP binding refusal too", async () => {
    ipAllowed = false;
    const ordinary = await submit();
    ipAllowed = true;
    block("2026-10-09T11:11:11.000Z");
    const blocked = await submit();
    expect(await blocked.text()).toBe(await ordinary.clone().text());
    expect([...blocked.headers]).toEqual([...ordinary.headers]);
    await expectWindow(ordinary, "ip_hourly", HOUR);
  });

  it("uses ordinary IP-binding precedence when a blocked caller also exhausts install quota", async () => {
    quota("ih", 3);
    ipAllowed = false;
    const ordinary = await submit();
    block(null);
    const blocked = await submit();
    expect(await blocked.text()).toBe(await ordinary.text());
    expect([...blocked.headers]).toEqual([...ordinary.headers]);
  });

  it("reports a conservative global burst wait independent of binding reset knowledge", async () => {
    env.FEEDBACK_BUDGET_LIMITER = { limit: async () => ({ success: false }) } as any;
    const r = await submit();
    expect(r.status).toBe(503);
    expect(await params(r)).toEqual({ limit: "global_burst", resetsAt: "2026-10-03T12:35:56.000Z", retryAfterSeconds: 60 });
  });

  it("preserves global reserve, trusted hard cap, zero caps, refunds and replay", async () => {
    db.prepare("INSERT INTO feedback_quota VALUES ('g:2026-10-03',270,'2026-10-03')").run();
    expect((await submit()).status).toBe(503);
    expect((db.prepare("SELECT n FROM feedback_quota WHERE bucket LIKE 'ih:%'").get() as any).n).toBe(0);
    trust();
    const key = "fixture-replay-key";
    expect((await submit({}, key)).status).toBe(201);
    db.exec("UPDATE feedback_quota SET n = 300 WHERE bucket LIKE 'g:%'");
    expect((await submit()).status).toBe(503);
    block(null);
    expect((await submit({}, key)).status).toBe(200);
    db.exec("DELETE FROM feedback_blocks; DELETE FROM feedback_quota");
    await call("/v1/admin/feedback/cap", { dailyGlobal: 0 });
    expect((await submit()).status).toBe(503);
  });
});

describe("release ledger and administration", () => {
  it("gives admin lockouts the same typed 429 shape", async () => {
    for (let i = 0; i < 5; i++) expect((await call("/v1/admin/feedback/held", undefined, { authorization: "Bearer wrong-fixture" })).status).toBe(401);
    await expectWindow(await call("/v1/admin/feedback/held"), "admin_attempts", "2026-10-03T12:45:00.000Z");
  });
  it("counts only distinct explicit releases, extends at five, survives retention and migration retries", async () => {
    for (let i = 0; i < 5; i++) {
      db.exec("DELETE FROM feedback_quota");
      const r = i === 0 ? receipt : (await (await submit()).json() as any).receipt;
      expect((await act("release", {}, r)).status).toBe(200);
      expect((await act("release", {}, r)).status).toBe(200);
      const expiry = (db.prepare("SELECT expires_at FROM feedback_trust").get() as any).expires_at;
      expect(expiry).toBe(new Date(Date.parse(NOW) + (i < 4 ? 30 : 90) * 86400000).toISOString());
    }
    db.exec("UPDATE feedback SET created_at = '2026-01-01', updated_at = '2026-01-01'");
    await purgeStaleFeedback(env);
    db.exec(triage);
    db.exec(triage);
    expect((db.prepare("SELECT COUNT(*) AS n FROM feedback_releases").get() as any).n).toBe(5);
    expect((db.prepare("SELECT COUNT(*) AS n FROM feedback").get() as any).n).toBe(0);
    const r = (await (await submit()).json() as any).receipt;
    await act("release", {}, r);
    expect((db.prepare("SELECT COUNT(*) AS n FROM feedback_releases").get() as any).n).toBe(6);
  });

  it("does not count automatic received submissions or shorten a longer grant", async () => {
    trust(365);
    const r = (await (await submit()).json() as any).receipt;
    expect((db.prepare("SELECT COUNT(*) AS n FROM feedback_releases").get() as any).n).toBe(0);
    await act("release", {}, r);
    expect((db.prepare("SELECT expires_at FROM feedback_trust").get() as any).expires_at).toBe("2027-10-03T12:34:56.000Z");
  });

  it("authenticates receipt grants/revokes, audits them, and never unblocks", async () => {
    const path = `/v1/admin/feedback/${receipt}/trust`;
    expect((await call(path, {}, {})).status).toBe(401);
    expect((await call(path, {}, admin, "PUT")).status).toBe(405);
    expect((await call(path, {})).status).toBe(200);
    expect((db.prepare("SELECT expires_at FROM feedback_trust").get() as any).expires_at).toBe("2027-10-03T12:34:56.000Z");
    await act("release");
    expect((db.prepare("SELECT expires_at FROM feedback_trust").get() as any).expires_at).toBe("2027-10-03T12:34:56.000Z");
    block(null);
    await expectWindow(await submit(), "install_hourly", HOUR);
    expect((await call(path, undefined, admin, "DELETE")).status).toBe(200);
    expect(db.prepare("SELECT * FROM feedback_trust").get()).toBeUndefined();
    expect(db.prepare("SELECT action FROM feedback_audit").all()).toEqual([{ action: "trust" }, { action: "untrust" }]);
    expect(db.prepare("SELECT * FROM feedback_blocks").get()).toBeDefined();
  });

  it.each(["POST", "DELETE"])("rolls back a %s trust mutation when audit fails", async (method) => {
    if (method === "DELETE") trust();
    db.exec("CREATE TRIGGER fail_audit BEFORE INSERT ON feedback_audit BEGIN SELECT RAISE(ABORT, 'fixture'); END");
    await expect(call(`/v1/admin/feedback/${receipt}/trust`, method === "POST" ? {} : undefined, admin, method)).rejects.toThrow();
    expect(db.prepare("SELECT * FROM feedback_trust").get() !== undefined).toBe(method === "DELETE");
  });

  it("reject and takedown revoke manual trust and auto-block still overrides it", async () => {
    expect((await act("trust")).status).toBe(200);
    await act("takedown");
    expect(db.prepare("SELECT * FROM feedback_trust").get()).toBeUndefined();
    expect((await act("trust")).status).toBe(200);
    await act("reject", { reason: "fixture" });
    expect(db.prepare("SELECT * FROM feedback_trust").get()).toBeUndefined();
    for (let i = 0; i < 2; i++) {
      db.exec("DELETE FROM feedback_quota");
      const r = (await (await submit()).json() as any).receipt;
      await act("reject", { reason: "fixture" }, r);
    }
    expect((await act("trust")).status).toBe(200);
    await expectWindow(await submit(), "install_hourly", HOUR);
  });
});

describe("reply neighbourhood", () => {
  it("conceals fresh untrusted reply blocks behind the ordinary IP window", async () => {
    await act("answer", { body: "Neutral answer" });
    const headers = { "x-install-id": ID, "x-install-token": token };
    ipAllowed = false;
    const ordinary = await call(`/v1/feedback/${receipt}/reply`, { body: "Neutral reply" }, headers);
    await expectWindow(ordinary.clone(), "ip_hourly", HOUR);
    ipAllowed = true;
    block(null);
    const blocked = await call(`/v1/feedback/${receipt}/reply`, { body: "Neutral reply" }, headers);
    expect(await blocked.text()).toBe(await ordinary.text());
    expect([...blocked.headers]).toEqual([...ordinary.headers]);
  });
  it.each([[false, 3], [true, 10]])("enforces reply tier with typed resets (trusted=%s)", async (trusted, limit) => {
    if (trusted) trust();
    await act("answer", { body: "Neutral answer" });
    const headers = { "x-install-id": ID, "x-install-token": token };
    if (trusted) ipAllowed = false;
    for (let i = 0; i < limit; i++) expect((await call(`/v1/feedback/${receipt}/reply`, { body: "Neutral reply" }, headers)).status).toBe(201);
    const ordinary = await call(`/v1/feedback/${receipt}/reply`, { body: "Neutral reply" }, headers);
    block("2026-11-01T00:00:00.000Z");
    const blocked = await call(`/v1/feedback/${receipt}/reply`, { body: "Neutral reply" }, headers);
    expect(await blocked.text()).toBe(await ordinary.clone().text());
    expect([...blocked.headers]).toEqual([...ordinary.headers]);
    await expectWindow(ordinary, "reply_hourly", HOUR);
    db.exec("DELETE FROM feedback_blocks");
    vi.setSystemTime(HOUR);
    const next = await call(`/v1/feedback/${receipt}/reply`, { body: "Neutral reply" }, headers);
    if (trusted) await expectWindow(next, "reply_item", null);
    else expect(next.status).toBe(201);
  });

  it("refunds invalid ownership/status and preserves ask-to-held transition", async () => {
    trust();
    const headers = { "x-install-id": ID, "x-install-token": token };
    const reply = () => call(`/v1/feedback/${receipt}/reply`, { body: "Neutral reply" }, headers);
    expect((await reply()).status).toBe(409);
    expect((db.prepare("SELECT n FROM feedback_quota WHERE bucket LIKE 'rh:%'").get() as any).n).toBe(0);
    await act("ask", { body: "Neutral question" });
    expect((await reply()).status).toBe(201);
    expect((db.prepare("SELECT status FROM feedback WHERE receipt = ?").get(receipt) as any).status).toBe("held");
    expect((await reply()).status).toBe(409);
    expect((db.prepare("SELECT n FROM feedback_quota WHERE bucket LIKE 'rh:%'").get() as any).n).toBe(1);
  });
});
