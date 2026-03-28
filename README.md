# AuraPanel

AuraPanel is being refactored into a `Vue + Vite -> Go API Gateway -> Go Panel Service` architecture.

## Current Direction

- Frontend: Vue 3 + Vite
- Edge/API Gateway: Go
- Domain Service Layer: Go
- Database tooling: MariaDB + PostgreSQL management
- Legacy runtime components have been removed from the active codebase and startup flow.

## Development

Requirements:

- Go 1.22+
- Node.js 20+

Run the local stack:

```powershell
.\start-dev.ps1
```

Or start services manually:

```powershell
cd panel-service
go run .
```

```powershell
cd api-gateway
$env:AURAPANEL_SERVICE_URL='http://127.0.0.1:8081'
go run .
```

```powershell
cd frontend
npm install
npm run dev
```

## Build

```bash
make build
```

## Repository Layout

```text
aurapanel/
├── panel-service/   # Go domain service
├── api-gateway/     # Go edge gateway
├── frontend/        # Vue + Vite panel UI
├── installer/
├── docs/
└── start-dev.ps1
```
