# Feedback admin page

Read and triage in-app feedback in a browser instead of the `feedback-admin` CLI.

- **Where:** `https://crash.reasonix.io/admin/feedback`. It is not served on the
  `*.workers.dev` host (the host gate answers 404), only on the custom domain.
- **Token:** the page asks for `FEEDBACK_ADMIN_TOKEN`, the same secret the
  converter and the CLI use (a Worker secret: `wrangler secret put
  FEEDBACK_ADMIN_TOKEN`). It is kept in the tab's `sessionStorage` only, sent in
  the `Authorization` header, never placed in a URL, and dropped when the tab
  closes or on "退出".
- **List:** newest first; filter by status, kind, app version, nickname
  (substring) and install (id, or 8+ hex characters of the install hash); 25 per
  page. Backed by `GET /v1/admin/feedback/list` (`status`, `category`,
  `version`, `nickname`, `install`, `limit`, `before`).
- **Detail:** full text, device info, contact (shown only here), thread, and
  screenshots. Unreleased screenshots are fetched through the admin-only
  attachment endpoint, so they are visible here before anything is public.
- **Actions:** release (optionally publishing screenshots), answer, ask,
  reply, reject, delete screenshots, block the install.

Safeguards: five wrong tokens from one address within 15 minutes lock that
address out of the whole admin API for the rest of the window, a correct token
included. The lock lifts by itself after the 15-minute window; to clear it sooner, delete the `af:%` rows from `feedback_quota` (`wrangler d1 execute reasonix-crash --remote --command "DELETE FROM feedback_quota WHERE bucket LIKE 'af:%'"`). User text is only ever written with `textContent`, the page ships a
strict CSP (`script-src 'self'`, no inline script or style), `noindex`, and
`no-store`. The page and its assets contain no feedback data.
