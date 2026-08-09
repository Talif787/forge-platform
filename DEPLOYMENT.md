# Deployment

A live, $0/month deployment of Forge that a reviewer can visit. The portal and the
catalog/tenant API run for free; the Kubernetes- and Temporal-dependent parts
(Applications, Provisioning) show their graceful "unavailable" panels, because no
free tier can run a Kubernetes cluster or a Temporal server.

## Architecture (what is live vs local)

```
Visitor
  -> Vercel (Next.js portal, HTTPS)            [FREE, always on]
       -> BFF proxy (Vercel serverless fn) mints a token, forwards
            -> Render (Go control-plane API)   [FREE, sleeps when idle]
                 -> Neon (Postgres)            [FREE, scale-to-zero]

Applications view  -> API returns 503 (no cluster)   -> portal shows "unavailable"
Provisioning view  -> API returns 503 (no Temporal)  -> portal shows "unavailable"
```

Live and working: dashboard, service catalog (list, create, lifecycle, ownership,
retire), tenants (create, status, quota), command palette, theming, all with
optimistic concurrency. Not live (by design, cost): the operator/reconciler and
its Applications view, and the Temporal provisioning view. Those remain a local
`make` demo.

## Accounts required (all free, no credit card)

GitHub (you have it), Neon (neon.tech), Render (render.com), Vercel (vercel.com).
None require a card for the tiers used here.

## Step 1: Postgres on Neon

1. Create a Neon project (region near you). Create a database named `forge`.
2. Copy the POOLED connection string (Connection Details -> Pooled connection).
   It looks like `postgresql://user:pass@ep-xxx-pooler.region.aws.neon.tech/forge?sslmode=require`.
   Keep `sslmode=require`.
3. That string is your `FORGE_DB_URL`. The API runs migrations automatically on
   first boot, so no manual schema step is needed.

## Step 2: API on Render

1. Generate the shared auth secret once and save it:
   `openssl rand -base64 48`
2. In Render: New -> Blueprint, point at your backend repo. Render reads
   `render.yaml` and creates the `forge-api` web service (free plan, Docker,
   `deploy/Dockerfile`, health check `/readyz`).
3. When prompted for the `sync: false` secrets, set:
   - `FORGE_DB_URL` = your Neon pooled string
   - `FORGE_AUTH_HMAC_SECRET` = the secret from step 1
4. Deploy. First build takes a few minutes. When live, note the URL
   (`https://forge-api-XXXX.onrender.com`) and verify:
   `curl https://forge-api-XXXX.onrender.com/readyz`

## Step 3: Portal on Vercel

1. In Vercel: New Project -> import your frontend repo. Vercel detects Next.js.
2. Set Environment Variables (Production):
   - `FORGE_BACKEND_URL` = your Render URL from step 2
   - `FORGE_AUTH_HMAC_SECRET` = the SAME secret as the API
   - `FORGE_DEV_SUBJECT` = `portal`
   - `FORGE_DEV_GROUPS` = `platform`
3. Deploy. Vercel gives you `https://your-portal.vercel.app` with HTTPS.
   Open it: the dashboard should load, and Services/Tenants should work live
   (allow ~30-60s on the very first request while Render wakes from sleep).

## Step 4: Keep the API warm (avoid cold starts)

Render free services sleep after 15 minutes idle (30-60s cold start). To avoid a
reviewer hitting that:

- Simplest: create a free UptimeRobot monitor hitting
  `https://forge-api-XXXX.onrender.com/readyz` every 5 minutes. No card.
- Or enable the included `.github/workflows/keepalive.yml` and set the repo
  variable `API_URL` to your Render URL.

Budget note: one free Render service kept awake ~24/7 uses close to the 750 free
instance-hours per month, which is within budget for a single service.

## Environments

Development is local (`make run`, `npm run dev`). This guide is the single hosted
environment (labeled `staging` via `FORGE_ENV` so shared-secret auth is allowed).
Vercel automatically gives every pull request a preview deployment.

## Security

The portal never exposes a token to the browser: its server-side BFF proxy mints
a short-lived HS256 token and forwards to the API, which verifies it. The secret
lives only in Render and Vercel server env, never in the client. This is
shared-secret service auth, appropriate because the portal is the only client.
A real multi-user production deployment would set `FORGE_ENV=production` and use
JWKS/OIDC (the API already supports this via `FORGE_AUTH_MODE=jwks`).

Security headers (HSTS, X-Frame-Options DENY, X-Content-Type-Options, Referrer
and Permissions policies) are set by the portal. There is no CORS surface: the
browser only talks to the Vercel origin.

## Verification (smoke test)

1. `curl $API/readyz` returns ok.
2. Portal dashboard shows control plane Ready and counts.
3. Create a tenant, then a service under it; promote it (with an on-call ref).
4. Applications and Provisioning pages show the "unavailable" panel (expected).
5. Toggle theme; open the command palette (Ctrl/Cmd-K).

## Cost

$0/month. Neon free (0.5 GB, 100 compute-hours, never expires), Render free web
service (750 instance-hours), Vercel Hobby (free, non-commercial). No paid
service is used.

## What a full production deployment would require (not free)

Surfacing Applications and Provisioning live needs a managed Kubernetes cluster
(EKS/GKE/AKS, roughly $70+/month for control plane plus nodes) with the operator
and CRD installed, and a Temporal server (Temporal Cloud, or self-hosted on a
paid VM). The relay and worker would run as small always-on containers. These are
deliberately out of scope for a $0 deployment; the portal degrades gracefully in
their absence, which is itself a designed feature.

## Rollback

Both Render and Vercel keep previous deployments. To roll back, select a prior
successful deploy in either dashboard and promote it. Database migrations are
additive; a bad app deploy is rolled back at the platform layer without touching
data.

## Troubleshooting

Slow first load: Render/Neon cold start; the keep-alive monitor prevents it.
Dashboard shows "could not reach control plane": the API is asleep or waking, or
`FORGE_BACKEND_URL`/secret mismatch; confirm both match and `curl $API/readyz`.
401 from the API: the portal's `FORGE_AUTH_HMAC_SECRET` does not match the API's.
