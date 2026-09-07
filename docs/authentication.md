# Player sessions and ownership

Every API read and command requires an expiring player session except
`GET /healthz`. The API accepts `Authorization: Bearer <session key>`.
PostgreSQL stores a SHA-256 digest of a randomly generated 256-bit secret,
the player, expiry, and optional revocation time. Expired, revoked, or unknown
sessions return 401. Household ownership comes from `households.owner_player_id`;
unauthorized household reads and writes return 403 before gameplay processing.
Market listings require ownership of a household in the requested world.

The first implementation supports host-provisioned playtest accounts. An
operator creates the player and assigns household ownership through the trusted
database administration workflow, then issues a session for that existing player:

```sh
cd backend
go run -tags postgres ./cmd/session -player-id PLAYER_UUID -lifetime 24h
```

`DATABASE_URL` must be supplied through the operator's environment. The command
prints the secret once; distribute it privately to its player. Do not put it in
URLs, repository files, logs, or frontend environment variables. Maximum session
lifetime is 30 days. Revoke a session by setting its `revoked_at` in PostgreSQL;
deleting a player also deletes their sessions. No HTTP endpoint can issue a
session, create a player, or claim or transfer a household.

The browser's `/sign-in` page validates the secret server-side and stores it in
an HttpOnly, SameSite=Lax cookie, Secure outside development. SvelteKit forwards
it as a bearer header only to the configured backend origin. Its form origin
protection remains enabled. Serve the frontend over HTTPS in production and
keep backend transport private or encrypted. PostgreSQL expiry is authoritative;
GET/page load does not extend a session.

Self-service registration, identity-provider login, account recovery, and a
session management UI are not implemented. Host provisioning is sufficient for
the controlled playtest, but those account lifecycle decisions remain necessary
before unrestricted public signup. There is no authentication bypass for local
development; provision a playtest player instead.
