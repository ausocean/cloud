# AusOcean Roadmap

This directory contains the backend (Go) and frontend (Vite + TypeScript) for
the AusOcean Roadmap web application.

```
cmd/roadmap/
├── main.go, auth.go        # Go backend (Fiber)
├── .env.example            # Template for local backend env vars
├── .env                    # Your local backend env vars (gitignored)
└── webapp/
    ├── .env.development    # Vite env vars used by `npm run dev`
    ├── .env.production     # Vite env vars used by `npm run build`
    └── ...                 # Frontend source
```

## Environment Configuration

### Backend (`cmd/roadmap/.env`)

The Go server loads `cmd/roadmap/.env` automatically on startup using
[`godotenv`](https://github.com/joho/godotenv). Variables that are already set
in the process environment are not overwritten, so production deployments
(which inject variables via `ausocean-roadmap.yaml`) are unaffected.

To get started, copy the template and edit as needed:

```bash
cp cmd/roadmap/.env.example cmd/roadmap/.env
```

`.env.example` documents every variable the backend reads. `.env` itself is
gitignored.

### Frontend (`webapp/.env.development` and `webapp/.env.production`)

Vite automatically loads these files based on the
[mode](https://vite.dev/guide/env-and-mode):

- `npm run dev` runs in `development` mode → loads `.env.development`.
- `npm run build` runs in `production` mode → loads `.env.production`.

Both files are committed because they only contain non-secret URLs. For
machine-specific overrides create `webapp/.env.local` or
`webapp/.env.development.local` (both gitignored).

Only variables prefixed with `VITE_` are exposed to client-side code. **Never
put secrets here** — the values are bundled into the built JavaScript.

## Local Development

You will need two terminals: one for the frontend dev server and one for the
Go backend.

### 1. Frontend

```bash
cd cmd/roadmap/webapp
npm install
npm run dev
```

The Vite dev server starts on `http://localhost:5173` and uses
`VITE_API_URL=http://localhost:8080` from `.env.development` to talk to the
local backend.

### 2. Backend

In a separate terminal:

```bash
cd cmd/roadmap
cp .env.example .env       # only needed once
go build
./roadmap
```

The backend listens on `http://localhost:8080` by default. Open
`http://localhost:5173` in your browser.

## Building for Production

To produce the static assets that get deployed alongside the Go binary:

```bash
cd cmd/roadmap/webapp
npm run build
```

The output is written to `webapp/dist/`, which is what
`ausocean-roadmap.yaml` serves as the static frontend. `npm run build` runs
in `production` mode, so `VITE_API_URL` is taken from `.env.production`
(`https://roadmap.ausocean.org`).

## Deployment

From `cmd/roadmap/webapp`:

```bash
npm run deploy        # production build + deploy
npm run deploy:dev    # build + deploy to dev-dot-* with --no-promote
```

Both scripts wrap `scripts/deploy.sh` and use the env variables defined in
`ausocean-roadmap.yaml` at the repo root.
