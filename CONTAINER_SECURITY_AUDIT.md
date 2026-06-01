# Container Security Audit — Phase 15G

## Engine Dockerfile Hardening Status

| Control | Before | After | Status |
|---------|--------|-------|--------|
| Non-root user | Root (UID 0) | UID 10001 | ✅ Fixed |
| Minimal base image | `alpine:latest` | `scratch` | ✅ Fixed |
| Strip debug symbols | No | `-ldflags="-s -w"` | ✅ Fixed |
| Static binary | No | CGO_ENABLED=0 | ✅ Fixed |
| Read-only filesystem | No | Documented guidance | ⚠️ Runtime flag |
| No secrets in image | Partial | Never baked in | ✅ Fixed |
| Signed image | No | TODO: cosign | ❌ Remaining |
| Vulnerability scan | No | TODO: trivy in CI | ❌ Remaining |

---

## Recommended Docker Run (Production)

```bash
docker run \
  --read-only \
  --tmpfs /tmp:rw,noexec,nosuid,size=64m \
  --security-opt no-new-privileges \
  --cap-drop ALL \
  --user 10001:10001 \
  --env VAULT_ADDR=https://vault.internal:8200 \
  --env VAULT_TOKEN="${VAULT_TOKEN}" \
  --env PORT=8080 \
  --publish 8080:8080 \
  --health-cmd="wget -qO- http://localhost:8080/health || exit 1" \
  --health-interval=30s \
  --health-retries=3 \
  ghcr.io/your-org/antigravity-engine:latest
```

---

## docker-compose.prod.yml Security Additions

```yaml
services:
  engine:
    image: ghcr.io/your-org/antigravity-engine:latest
    user: "10001:10001"
    read_only: true
    tmpfs:
      - /tmp:rw,noexec,nosuid,size=64m
    security_opt:
      - no-new-privileges:true
    cap_drop:
      - ALL
    environment:
      - VAULT_ADDR=${VAULT_ADDR}
      - VAULT_TOKEN=${VAULT_TOKEN}
      - PORT=8080
      - SECURITY_ENFORCE_AUTH=true
      - ALLOWED_ORIGINS=https://antigravity.vercel.app
      - ENGINE_ADMIN_CIDR=10.0.0.0/8,172.16.0.0/12
    # REMOVE: env_file: - .env  (secrets must come from Vault)
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:8080/health"]
      interval: 30s
      retries: 3
      start_period: 10s
    restart: unless-stopped
    networks:
      - internal
    deploy:
      resources:
        limits:
          cpus: "2"
          memory: 512M
```

---

## Remaining Items

1. **Image signing**: Add `cosign sign` to CI/CD after `docker push`
2. **Vulnerability scanning**: Add `trivy image` step in GitHub Actions
3. **SBOM generation**: Add `syft` for software bill of materials
4. **Network policy**: Restrict egress to exchange IPs only (Binance/Delta CIDR)
5. **Secret injection**: Use Vault Agent sidecar instead of environment variable for VAULT_TOKEN

---

## Threat Model

| Threat | Mitigation |
|--------|-----------|
| Container escape | scratch base (no shell), cap-drop ALL |
| Secret exfiltration | read-only FS, Vault injection, no disk secrets |
| Privilege escalation | no-new-privileges, non-root UID |
| Image tampering | TODO: cosign signature verification |
| Exposed debug endpoints | /api/logs requires audit.view permission (RBAC) |
