# Browser client

SvelteKit household dashboard backed by the Go command/projection API. Server loads read the current report, shipments, and market offers; form actions send assignment and purchase intents.

## Local development

Start PostgreSQL and the Go API as described in `../backend/README.md`, then:

```bash
cp .env.example .env
npm install
npm run dev
```

Open `http://localhost:5173`. The root route redirects to the seeded Bjornvik household. `BACKEND_URL` is private to the SvelteKit server and defaults to `http://localhost:8080`.

## Checks

```bash
npm run check
npm run lint
npm run test:unit -- --run
npm run build
npx playwright install chromium  # first run only
npm run test:e2e
```

The Playwright test starts its own mock API and does not modify the development database.

After changing `../backend/openapi.yaml`, regenerate the typed client:

```bash
npm run generate:api
```
