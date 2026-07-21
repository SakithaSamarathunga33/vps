# Contributing to PulseNode

Thanks for wanting to help! All contributions are welcome — bug reports, docs fixes, features, and ideas.

## Reporting bugs / requesting features

Use the [issue templates](https://github.com/SakithaSamarathunga33/PulseNode/issues/new/choose). For security vulnerabilities, **do not open a public issue** — see [SECURITY.md](SECURITY.md).

## Development setup

PulseNode is a Next.js 14 frontend (repo root) + Go backend (`backend/`), run together via Docker Compose behind Caddy.

**Prerequisites:** Docker 24+ with Compose v2, Node 20+, Go 1.22+.

```bash
git clone https://github.com/SakithaSamarathunga33/PulseNode.git
cd PulseNode
cp .env.example .env.local          # then edit values

# Run the full stack (recommended — matches production routing):
docker compose -f docker-compose.yml -f docker-compose.standalone.yml up -d --build

# Frontend only (hot reload):
npm install
npm run dev

# Backend only:
cd backend && go build ./... && go vet ./...
```

After changing Go code or a Dockerfile, rebuild just that service:

```bash
docker compose -f docker-compose.yml -f docker-compose.standalone.yml up -d --build go-api   # or: web
```

Note: the Go API needs `/var/run/docker.sock` and `pid: host`, so the metrics and Docker features only fully work inside the compose stack on Linux.

## Commit messages — they drive releases

CI auto-versions and publishes a release on every push to `main` based on the commit prefix:

| Prefix | Effect |
|--------|--------|
| `feat: ...` | minor bump (v1.1.0 → v1.2.0) |
| `fix: ...` / `chore: ...` / others | patch bump (v1.1.0 → v1.1.1) |
| `BREAKING CHANGE` in body | major bump (v1.x → v2.0.0) |

Please write commits as `type(scope): summary`, e.g. `fix(containers): handle null ports in list view`.

## Pull requests

1. Fork and create a branch from `main`.
2. Keep PRs focused — one change per PR.
3. Make sure the stack builds: `docker compose -f docker-compose.yml -f docker-compose.standalone.yml build` and `cd backend && go build ./...`.
4. Fill in the PR template — a screenshot or clip is very helpful for UI changes.

## Code style

- **Go:** standard `gofmt`; keep handlers in `backend/internal/api`, business logic in the relevant `internal/` package. Initialize JSON slices to non-nil empty slices (Go `nil` marshals to `null` and breaks the React client).
- **TypeScript/React:** match the existing shadcn/ui + Tailwind patterns in `components/` and `app/`.
- Match the surrounding code's style, naming, and comment density.
