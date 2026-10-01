import { APP_CSS } from "./feedback_admin_ui_css";
import { APP_JS } from "./feedback_admin_ui_js";

export const PAGE_ROUTE = "/admin/feedback";

const CSP = "default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self' blob:; connect-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'";

const HTML = `<!doctype html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex, nofollow, noarchive">
<meta name="color-scheme" content="light dark">
<title>Feedback admin</title>
<link rel="stylesheet" href="${PAGE_ROUTE}/app.css">
</head>
<body>
<div id="app"></div>
<noscript>This page needs JavaScript.</noscript>
<script src="${PAGE_ROUTE}/app.js" defer></script>
</body>
</html>
`;

const ASSETS: Record<string, { body: string; type: string }> = {
  [PAGE_ROUTE]: { body: HTML, type: "text/html; charset=utf-8" },
  [`${PAGE_ROUTE}/app.js`]: { body: APP_JS, type: "text/javascript; charset=utf-8" },
  [`${PAGE_ROUTE}/app.css`]: { body: APP_CSS, type: "text/css; charset=utf-8" },
};

// The shell and its assets hold no data and no secret; everything private comes
// from the bearer-guarded JSON endpoints. Returns null when the path is not ours.
export function servePage(request: Request, path: string): Response | null {
  const asset = Object.hasOwn(ASSETS, path) ? ASSETS[path] : undefined;
  if (!asset) return null;
  if (request.method !== "GET" && request.method !== "HEAD") {
    return new Response("method not allowed", { status: 405, headers: { allow: "GET, HEAD", "cache-control": "no-store" } });
  }
  return new Response(request.method === "HEAD" ? null : asset.body, {
    headers: {
      "content-type": asset.type,
      "content-security-policy": CSP,
      "x-robots-tag": "noindex, nofollow, noarchive",
      "cache-control": "no-store",
      "x-content-type-options": "nosniff",
      "x-frame-options": "DENY",
      "referrer-policy": "no-referrer",
      "cross-origin-opener-policy": "same-origin",
      "cross-origin-resource-policy": "same-origin",
    },
  });
}
