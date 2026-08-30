# DocuWave — Product Requirements Document (v1)

## Overview

DocuWave is a hybrid SaaS report automation tool. The platform is hosted by the operator; clients connect their own data sources remotely, configure report templates, and schedule automated delivery via email.

---

## Problem Statement

Businesses need recurring reports pulled from their data sources and distributed to stakeholders. Today this is done manually — someone queries a database or spreadsheet, formats the output, and emails it. DocuWave automates this entire pipeline: connect a data source, describe what you want in natural language, pick a template, set a schedule, and reports go out automatically.

---

## Target Users

- Businesses that run recurring reports (weekly sales summaries, monthly finance reports, operational dashboards, etc.)
- Single user per account — one person configures and manages the workspace
- Non-technical and technical users alike (natural language query removes the need to write SQL)

---

## Deployment Model

**Hybrid SaaS** — DocuWave is hosted and operated centrally. Clients authenticate via the web app and connect their own external data sources (their DB or Google Sheets stays on their side; queries run from DocuWave's servers against those sources).

---

## Tech Stack

| Layer | Technology |
|---|---|
| Frontend | Next.js (React) |
| Backend | Go |
| Database | PostgreSQL |
| Containerization | Docker (docker-compose — all services in one compose file) |

All services run in separate containers under a single `docker-compose.yml`. Architecture must be K8s-ready for future migration.

---

## Authentication

- Email + password
- Google OAuth ("Sign in with Google")
- Single user per account (no team/role management in v1)

---

## Data Sources (v1)

| Source | Connection Method |
|---|---|
| SQL databases (PostgreSQL, MySQL, MSSQL) | Connection string (stored encrypted) |
| Google Sheets | OAuth 2.0 (user connects their Google account) |

**Architectural constraint:** The data source connector layer must be designed as a pluggable interface so additional sources (REST APIs, SaaS tools, file uploads, etc.) can be added without rewriting core logic.

---

## Query Generation

Users describe what they want to see in **natural language** (e.g., "sum of sales by region for last month"). An AI agent interprets the data source schema and generates the appropriate query.

- LLM is **pluggable and client-configurable** — clients provide their own API key and select their model (Claude, GPT-4o, etc.)
- Agent inspects the source schema (table/column names, types) and uses the LLM to produce a valid query
- For Google Sheets, the agent reads column headers and generates the appropriate aggregation/filter logic

---

## Report Templates

**v1:** A set of pre-designed templates. Users pick a template and map their data fields to the template's variable slots.

**v2 (future):** A drag-and-drop visual template builder.

**Architectural constraint:** The template system must expose a clean interface so the v2 builder can be plugged in and used as a drop-in replacement without changing the rendering layer.

---

## Report Output Formats

Clients configure which format(s) they want per report. Supported options:

- PDF
- Excel (.xlsx)
- CSV

---

## Report Delivery

**v1:** Email — reports sent as attachments on schedule or on-demand trigger.

**Future:** WhatsApp, Slack, webhook, cloud storage (S3, Google Drive, Dropbox).

**Architectural constraint:** The delivery layer must be pluggable so new channels can be added without touching report generation logic.

---

## Scheduling

Two modes, configured per report:

1. **Fixed intervals** — Daily, Weekly, Monthly (simple dropdown selection)
2. **Manual / on-demand trigger** — User triggers the report manually from the app

---

## Key Architectural Constraints (must be respected from day one)

1. **Connector layer is pluggable** — new data sources (REST APIs, SaaS tools, etc.) must be addable without rewrites
2. **Delivery layer is pluggable** — new channels (WhatsApp, Slack, webhooks) must be addable without touching report generation
3. **Template system is pluggable** — the v2 drag-and-drop builder must be droppable in without changing the render pipeline
4. **LLM is swappable** — clients configure their own provider and API key; no LLM is hardcoded

---

## Out of Scope for v1

- Team/multi-user accounts and role management
- In-app dashboard / interactive chart views
- Cloud storage delivery (S3, Google Drive, Dropbox)
- WhatsApp / Slack delivery
- Event/threshold-based triggers
- SSO (SAML/OIDC)
- Drag-and-drop template builder
- Monetization / billing

---

## MVP Checklist

- [ ] Auth — email/password + Google OAuth
- [ ] Data source connection — SQL (connection string) + Google Sheets (OAuth)
- [ ] Schema introspection — agent reads source schema
- [ ] Natural language → query generation via pluggable LLM
- [ ] Pre-designed report templates with field mapping
- [ ] Report rendering — PDF, Excel, CSV
- [ ] Email delivery
- [ ] Scheduling — fixed intervals + manual trigger
- [ ] Docker — all services containerized in docker-compose
