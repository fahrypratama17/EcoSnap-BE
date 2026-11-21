# Free hosting options (student demo)

Advice only; nothing deployed. Read 2026-11-21 from official docs; re-check before relying on them.

## Recommendation

Use **Render Free web service** for the API, with the **existing Supabase Postgres** and **Firebase Auth**.
This is a demo setup, **not production**. Render Free sleeps when idle, has a cold start and a monthly hour quota.
Keep the database on Supabase: Render's free Postgres expires after 30 days.
Wake the service 1–2 minutes before a demo. Do not rely on local disk.

## Render Free (<https://render.com/docs/free>)

- 512 MB RAM, 0.1 CPU, 750 free workspace hours per month.
- Sleeps after 15 minutes idle. Cold start is about 1 minute.
- Ephemeral filesystem. Free TLS.
- Bandwidth and build overage is billed if a payment method is on file, otherwise the service is suspended.
- Heavy traffic to an external database can suspend the service.
- Card requirement for signup: not verified here.

## Koyeb (<https://www.koyeb.com/docs/reference/instances>)

- Free Instance: free of charge, no trial limit stated. 512 MB RAM, 0.1 vCPU, 2 GB SSD. One per organization.
- Only Frankfurt or Washington, D.C.
- Scales to zero after 1 hour idle. No Volumes. Not for production, per Koyeb.
- Card requirement: not stated on this page, so not verified.

## Railway (<https://docs.railway.com/pricing/free-plan>)

- Free plan: $1 of usage per month, no rollover. The page says no card is required. Not tested.
- Services sleep after 5–10 minutes. When the $1 is gone, services stop until the next cycle.
- Trial: one-time $5, 30 days, no sleep, no card per the page. It ends when 30 days pass or the $5 is spent.
- Unverified accounts get a Limited Trial with restricted outbound network and ports. This may affect calls to Supabase and Firebase.
- Always-on needs Hobby at $5 per month.

## Caveats

- All three are demo-grade. Expect sleep, cold starts, quotas and limited or ephemeral disk.
- Only Railway's page states "no card". Render and Koyeb pages read here are silent, so do not claim "no card" for them.
