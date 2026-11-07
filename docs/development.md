# Development

Use the pinned versions in .node-version/.nvmrc and go.mod. README.md contains the credential-free local setup and verification commands. Node 20 cannot run the current browser test dependencies; use `nvm use` in every terminal.

The testserver listens only on 127.0.0.1:8081 and uses ephemeral SQLite data, random test keys and local HTTP fake providers. Vite proxies /api to this server when API_PROXY_TARGET is set. Open exactly http://127.0.0.1:5173; localhost is a different origin. OAuth and mutation origin checks deliberately require the canonical origin.

The Playwright suite starts its own servers. Stop manual servers on 5173 and 8081 first. On unsupported Linux variants Playwright may use a compatible Ubuntu browser build; installation errors for shared libraries require the operating-system dependencies described by Playwright.

Synthetic fixtures model duplicates, ambiguous recordings, quota exhaustion and success followed by response timeout. They do not demonstrate real catalog accuracy. To inspect supported fixture IDs and scenarios see web/e2e/helpers.ts and internal/testfixtures/providers/server.go.

Private project workflow notes are excluded from the public release inventory. Public project knowledge belongs in docs; temporary verification reports remain local.
