# KyNotes Web MVP

This is the responsive browser client for KyNotes Server.

## Run locally

Start the Go server first, then run:

```bash
cd web
npm install
cp .env.example .env.local
npm run dev -- --host 0.0.0.0
```

Open `http://127.0.0.1:5173` on the server machine. To use a server on another
host or port, set `VITE_API_PROXY_TARGET` in `.env.local` to that server's
address. The browser still talks to the Vite origin, so cookies and CSRF remain
same-origin during development.

## Checks

```bash
npm test
npm run build
```

The production build is in `dist/`. Serve it from a static HTTPS web server
and proxy `/api/` to KyNotes Server on the same origin. Do not expose the Go
API and frontend on unrelated origins without adding a narrowly scoped CORS
policy and reviewing cookie behavior.

## Current MVP boundary

The first slice supports session login, encrypted note creation/editing,
container navigation, local IndexedDB ciphertext caching, and responsive
layout. Browser device-envelope enrollment and full offline queueing require
the frozen X25519 envelope wire format to be implemented by the client.
