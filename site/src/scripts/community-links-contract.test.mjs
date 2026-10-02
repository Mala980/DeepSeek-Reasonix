import assert from "node:assert/strict";
import { access, readFile } from "node:fs/promises";
import test from "node:test";
import { DISCORD_URL, DOUYIN, QQ_GROUP } from "../lib/community-links.mjs";

const source = (path) => readFile(new URL(path, import.meta.url), "utf8");

test("community links are the official QQ group and the repo's Discord invite", () => {
  assert.equal(QQ_GROUP.name, "DeepSeek-Reasonix官方群");
  assert.equal(QQ_GROUP.number, "1093562660");
  assert.equal(QQ_GROUP.joinUrl, "https://qm.qq.com/q/i59b0z2R8s");
  assert.equal(DISCORD_URL, "https://discord.gg/XF78rEME2D");
});

test("the Douyin account is shown as an ID and a QR image, never a guessed link", async () => {
  assert.equal(DOUYIN.name, "做游戏的小鱼");
  assert.equal(DOUYIN.id, "22703872788");
  await access(new URL("../../public/community/douyin.png", import.meta.url));
  const component = await source("../components/CommunityJoin.astro");
  assert.match(component, /DOUYIN\.qrPath/);
  assert.doesNotMatch(component, /douyin\.com/);
});

test("the QR asset exists and the Discord invite lives only in the shared links module", async () => {
  await access(new URL("../../public/community/qq-group.svg", import.meta.url));
  const svg = await source("../../public/community/qq-group.svg");
  assert.match(svg, /<svg\b[^>]*viewBox=/);
  const component = await source("../components/CommunityJoin.astro");
  assert.doesNotMatch(component, /discord\.gg\//);
});

test("every footer carries the join block", async () => {
  const hosts = [
    "../pages/index.astro",
    "../pages/docs.astro",
    "../pages/skills.astro",
    "../pages/404.astro",
    "../layouts/Account.astro",
    "../layouts/Community.astro",
    "../components/ChangelogPage.astro",
  ];
  for (const host of hosts) {
    assert.match(await source(host), /<CommunityJoin\b/, host);
  }
  const component = await source("../components/CommunityJoin.astro");
  assert.match(component, /QQ_GROUP\.joinUrl/);
  assert.match(component, /DISCORD_URL/);
  assert.match(component, /QQ_GROUP\.qrPath/);
});
