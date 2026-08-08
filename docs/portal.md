# Portal (Next.js frontend)

A Next.js 15 App Router portal for the control plane, covering the full Catalog
and Tenant REST surface: list with filters, create, view, and every state
transition (service lifecycle and ownership, tenant status and quota), with
optimistic concurrency and idempotent creates.

## Backend-for-frontend proxy

The browser never holds a token. It calls `/api/proxy/<path>` on the Next.js
server, and a route handler mints a short-lived dev token server-side (from the
HMAC secret, which stays on the server) and forwards to the control plane. This
keeps credentials off the client and sidesteps CORS entirely, since the browser
only ever talks to the Next.js origin. In production the dev-token step is
replaced by an OIDC session; the browser still never sees a raw token.

Optimistic concurrency and idempotency flow through the proxy unchanged: the
portal sends `If-Match: <version>` on updates and an `Idempotency-Key` on service
creation, and the proxy forwards those headers.

## Running

The backend must be running first (see the root README), with its
`FORGE_AUTH_HMAC_SECRET` matching the portal's.

```bash
cd web
cp .env.example .env.local     # ensure the secret matches the backend
npm install                    # first time (add --legacy-peer-deps if peers conflict)
npm run dev                    # http://localhost:3000
```

On Cloud Shell, use Web Preview on port 3000 to open it.

## Scope

The portal covers everything the control plane exposes over HTTP (catalog and
tenants). The reconciler's Application resources and the provisioning workflows
are managed on the Kubernetes and Temporal sides and are not yet HTTP-exposed;
surfacing them in the portal is a clean follow-up that needs small read
endpoints on the control plane.

## Build gate

`npm run build` runs TypeScript type-checking and compiles the app; treat it as
the equivalent of `go build` for the frontend.
