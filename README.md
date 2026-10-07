# DocuWave

[![CI](https://github.com/Max20050/Ingsoft3/actions/workflows/ci.yml/badge.svg)](https://github.com/Max20050/Ingsoft3/actions/workflows/ci.yml)

Proyecto de la materia **Ingeniería de Software 3**.

DocuWave es una herramienta de automatización de reportes (SaaS híbrido): el usuario conecta sus
propias fuentes de datos (PostgreSQL, MySQL, Google Sheets, APIs REST), arma un reporte a partir de
plantillas y lo descarga en PDF, CSV o XLSX.

- Especificación funcional completa: [`documents/v1/PRD.md`](documents/v1/PRD.md)
- Decisiones de cada TP: [`decisiones.md`](decisiones.md)
- Evidencias (capturas): [`evidencias.md`](evidencias.md)
- Convenciones de trabajo con agentes: [`AGENTS.md`](AGENTS.md)

---

## Stack

| Capa | Tecnología |
|---|---|
| Frontend | Next.js 16 (React 19, TypeScript, Tailwind CSS 4) |
| Backend | Go 1.24 (`net/http`, sin framework) |
| Base de datos | PostgreSQL 17 |
| Contenedores | Docker + Docker Compose |
| CI | GitHub Actions |

---

## Cómo levantar el proyecto

### Requisitos

- Docker y Docker Compose
- (Solo para desarrollo sin contenedores) Go 1.24+ y Node.js 20+

### 1. Variables de entorno

```bash
cp .env.example .env
```

Editar `.env`. Como mínimo hay que cambiar `JWT_SECRET` y, si se quiere usar login con Google o
Google Sheets, completar `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET` y las URLs de callback.

Variables principales:

| Variable | Para qué sirve |
|---|---|
| `POSTGRES_USER` / `POSTGRES_PASSWORD` / `POSTGRES_DB` | Credenciales del contenedor de Postgres |
| `DATABASE_URL` | Cadena de conexión que usa el backend |
| `PORT` | Puerto del backend (por defecto `8080`) |
| `JWT_SECRET` | Firma de los tokens de sesión |
| `DATASOURCE_ENCRYPTION_KEY` | Clave AES para cifrar las credenciales de las fuentes de datos |
| `LLM_ENCRYPTION_KEY` | Clave AES para cifrar las API keys de los proveedores de LLM |
| `FRONTEND_URL` | Origen permitido por CORS (el de nginx, no el del frontend directamente) |
| `AI_SUMMARY_ENABLED` | Habilita los bloques de resumen por IA (apagado por defecto) |

`DATASOURCE_ENCRYPTION_KEY` y `LLM_ENCRYPTION_KEY` deben ser claves de 32 bytes en base64. Se
generan con:

```bash
openssl rand -base64 32
```

### 2. Levantar todo con Docker Compose (recomendado)

Construye las imágenes desde el código local:

```bash
docker compose up -d --build
```

Servicios:

- App (vía nginx): http://localhost
- PostgreSQL: `localhost:5432`

El backend ya no publica su puerto al host. El contenedor del frontend corre nginx como reverse
proxy delante del server de Next: nginx escucha en `:80`, rutea `/api/*` al backend y el resto al
proceso de Next dentro del mismo contenedor (ver [Arquitectura](#arquitectura)).

Las migraciones de base de datos corren solas al arrancar el backend, así que no hay ningún paso
manual de setup de esquema.

Para ver logs y bajar el entorno:

```bash
docker compose logs -f
docker compose down          # agregar -v para borrar también el volumen de Postgres
```

### 3. Levantar usando las imágenes publicadas

No compila nada: descarga las imágenes ya publicadas en GitHub Container Registry (son públicas, no
hace falta login).

```bash
docker compose -f docker-compose.registry.yml up -d
```

Usa `ghcr.io/max20050/ingsoft3-backend` y `ghcr.io/max20050/ingsoft3-frontend`.

### 4. Desarrollo sin contenedores (opcional)

Necesita un PostgreSQL corriendo. La forma más simple es levantar solo la base:

```bash
docker compose up -d db
```

Backend:

```bash
cd backend/cmd/api
go run .
```

> El backend carga el archivo `.env` ubicado en `../../.env` respecto del directorio actual, por eso
> se ejecuta desde `backend/cmd/api` con un `.env` en `backend/`. Como alternativa, exportá las
> variables en la shell antes de correrlo.
>
> Los valores por defecto de `FRONTEND_URL`, `GOOGLE_REDIRECT_URL` y `GOOGLE_SHEETS_REDIRECT_URL`
> en `.env.example` asumen que se entra por nginx en `http://localhost`. Corriendo el backend
> suelto (sin nginx de por medio) hay que apuntarlos directo al puerto del backend, por ejemplo
> `http://localhost:8080/api/auth/google/callback`.

Frontend:

```bash
cd frontend
npm ci
npm run dev
```

Tests y chequeos:

```bash
cd backend && go test ./...
cd frontend && npm run lint && npm run type-check
```

---

## Arquitectura

### Vista general

```
                    ┌───────────────────────────────────────┐
   navegador ─────▶ │  frontend (contenedor)           :80  │
                    │  ┌───────────┐        ┌─────────────┐ │
                    │  │  nginx    │  /     │  Next.js    │ │
                    │  │  :80      ├───────▶│  :3000      │ │
                    │  └─────┬─────┘        └─────────────┘ │
                    └────────┼───────────────────────────────┘
                              │ /api/*
                              ▼
                    ┌──────────────────────────┐
                    │  backend  (Go)           │  :8080
                    │  net/http ServeMux+CORS  │
                    └────┬────────────────┬────┘
                         │                │
              pgx pool   │                │  conectores salientes
                         ▼                ▼
                 ┌───────────────┐   ┌──────────────────────────────┐
                 │ PostgreSQL 17 │   │ Fuentes del usuario:         │
                 │ (estado app)  │   │ Postgres · MySQL · Sheets ·  │
                 └───────────────┘   │ API REST · proveedores LLM   │
                                     └──────────────────────────────┘
```

Los tres servicios (frontend, backend, db) corren en contenedores separados dentro de un mismo
`docker-compose.yml`. Dentro del contenedor `frontend`, nginx hace de reverse proxy: escucha en
`:80`, sirve `/api/*` reenviando al contenedor `backend` y todo lo demás al proceso de Next.js
(`:3000`) corriendo en el mismo contenedor. El backend no publica su puerto al host.

### Backend (`backend/`)

Go sin framework web: el ruteo es el `http.ServeMux` de la biblioteca estándar y todo el cableado de
dependencias se arma explícitamente en `cmd/api/main.go`. Cada dominio vive en un paquete bajo
`internal/` y sigue el mismo patrón: un `store` (acceso a Postgres vía `pgx`) y unos `handlers`
(HTTP) por encima.

| Paquete | Responsabilidad |
|---|---|
| `internal/auth` | Registro/login con email + password (bcrypt), OAuth con Google, emisión y validación de JWT, middleware `RequireAuth` |
| `internal/datasource` | Conectores pluggables (Postgres, MySQL, REST, Google Sheets), cifrado AES de credenciales, descubrimiento y persistencia del esquema, mapeo de campos y construcción de queries |
| `internal/template` | Plantillas de reporte: starters built-in, plantillas propias del usuario, archivado, y bloques (tabular, KPI, agrupado, resumen por IA) |
| `internal/report` | El *runner*: orquesta query → plantilla → archivo, más el CRUD y la descarga de reportes |
| `internal/render` | Salida final en PDF (`fpdf`), XLSX (`excelize`) y CSV |
| `internal/llm` | Proveedores de LLM (Claude, OpenAI, OpenRouter) detrás de una interfaz común, con API keys cifradas |
| `internal/recipient` | Destinatarios y grupos de destinatarios para la distribución de reportes |
| `internal/migrate` | Migraciones SQL versionadas, aplicadas automáticamente al iniciar |

Dos decisiones de diseño que conviene tener presentes:

- **Conectores pluggables.** `datasource` expone una interfaz de conector, de modo que agregar una
  fuente nueva no obliga a reescribir el pipeline de reportes.
- **El runner es el único camino.** `report.Runner` es toda la tubería de generación, y es la misma
  pieza que va a usar la entrega programada por email, no solo la generación bajo demanda.

La API es REST bajo `/api/*`, con `GET /health` sin autenticar para el healthcheck.

### Frontend (`frontend/`)

Next.js con App Router. Las páginas autenticadas viven en el route group `app/(app)/`
(dashboard, datasources, reports, recipients, settings) y comparten el layout con la sidebar; el
login, el registro y el callback de OAuth quedan fuera de ese grupo. `lib/api.ts` centraliza las
llamadas al backend y `lib/auth-context.tsx` mantiene la sesión. Los componentes de UI están en
`app/ui/`. La imagen de producción usa el output `standalone` de Next, y la etapa final del
`Dockerfile` arranca ese server junto con nginx (`frontend/nginx/default.conf`,
`frontend/docker-entrypoint.sh`), que es quien expone `:80` y reenvía `/api/*` al backend.

### Base de datos

PostgreSQL 17 guarda el estado de la aplicación: usuarios, fuentes de datos (con credenciales
cifradas), esquemas cacheados, mapeos de campos, configuración de LLM, reportes, plantillas y
destinatarios. El esquema se versiona en `backend/internal/migrate/migrations/` y se aplica solo al
arrancar el backend.

---

## CI

El pipeline está en [`.github/workflows/ci.yml`](.github/workflows/ci.yml) y corre en cada push a
`main` y en cada pull request contra `main`.

Tiene dos jobs paralelos e independientes — `Build backend (Go)` y `Build frontend (Next.js)` — que
construyen la imagen Docker de cada servicio con Buildx, usando el mismo Dockerfile con el que se
levanta el proyecto. Así lo que valida el CI es exactamente el artefacto que después se despliega, en
vez de una compilación aparte. Las capas se cachean en GitHub Actions Cache (`type=gha`) con un scope
por servicio, para que un cambio en el frontend no invalide el caché del backend.

El badge del encabezado refleja el estado del pipeline en `main`.

---

## Flujo de trabajo

El repositorio usa **GitHub Flow**: `main` es la única rama permanente y está protegida.

- Cada issue se trabaja en su propia rama `agent/issue-{n}` y su propio worktree `.worktrees/issue-{n}`.
- Nunca se pushea directo a `main`; todo entra por pull request.
- Las dependencias entre issues se declaran explícitamente en el cuerpo del issue con `Depends on: #123`,
  y solo se consideran satisfechas cuando el PR correspondiente fue mergeado.

El detalle completo está en [`AGENTS.md`](AGENTS.md) y [`CLAUDE.md`](CLAUDE.md).
