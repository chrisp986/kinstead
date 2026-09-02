# Game scaffold — browser vertical slice

Go + PostgreSQL technical Vertical Slice baseline. The visible game name is intentionally configuration-only and absent from domain packages, SQL table names and API routes.

See `backend/README.md` for database and Go setup. The initial SvelteKit household dashboard is documented in `frontend/README.md`.

With the API running on port 8080, start the browser client:

```bash
cd frontend
cp .env.example .env
npm install
npm run dev
```

Open `http://localhost:5173` to view the seeded Bjornvik household, schedule work, purchase a market offer, inspect shipments, and read its structured activity log.
# kinstead
