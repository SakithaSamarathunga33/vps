# Changelog

PulseNode releases continuously: every push to `main` is auto-versioned from the
commit message (`feat:` → minor, `fix:`/`chore:` → patch, `BREAKING CHANGE` → major)
and published with generated notes and Docker images.

**The full, always-current changelog lives on the
[GitHub Releases page](https://github.com/SakithaSamarathunga33/PulseNode/releases).**

## Highlights

### v1.x
- One-command installer with admin-account setup and pre-built GHCR images
- Live system metrics over WebSocket/SSE (CPU, RAM, disk I/O, network)
- Docker management: containers, images, networks, logs, in-browser shell
- GitHub deployments: Dockerfile, Compose, Nixpacks, and monorepo builds with
  zero-downtime swaps, health gating, per-deploy rollback, and push webhooks
- Database provisioning and management (PostgreSQL, MySQL, MongoDB, Redis)
- Container security scanning (Trivy) and SBOMs (Syft)
- Alerts with threshold rules, Coolify integration, Caddy auto-HTTPS
