# Infrastructure Module

## Purpose
Support deployment, persistence, observability, recovery, and operational controls for the trading platform.

## Dependencies
- Vercel for the Next.js app.
- AWS Lightsail for the Go engine.
- MongoDB Atlas, PostgreSQL Neon, Redis, and SQLite fallback.
- Cron/admin controls.
- Logging, metrics, health, and runbooks.

## Entry Points
- `infrastructure/`
- `vercel.json`
- `docker-compose.prod.yml`
- `engine/internal/persistence/`
- `engine/internal/observability/`
- `engine/internal/security/`
- `client/src/app/api/cron/*`
- `client/src/app/api/admin/*`

## Public APIs
- Health endpoints.
- Admin and kill/reset routes.
- Cron worker routes.
- Engine health/metrics endpoints.

## Major Concepts
- Deployment boundary.
- Secrets and environment variables.
- Cron limit.
- Health check.
- Persistence fallback.
- Observability.
- Disaster recovery.

## Files
Respect the Vercel cron limit: no more than 2 cron jobs total across root and client config.
