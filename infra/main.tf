terraform {
  required_version = ">= 1.9.0"

  required_providers {
    neon = {
      source  = "kislerdm/neon"
      version = "0.18.0"
    }
    render = {
      source  = "render-oss/render"
      version = "1.9.1"
    }
  }
}

# Credenciales por variable de entorno (no se piden como variables de Terraform
# para no tener que marcarlas sensitive ni arriesgar que terminen en el state
# en texto plano más veces de las necesarias): NEON_API_KEY, RENDER_API_KEY,
# RENDER_OWNER_ID.
provider "neon" {}
provider "render" {}

# --- Base de datos (Neon) ---------------------------------------------------

resource "neon_project" "preprod" {
  name = "ingsoft3-preprod"

  # Neon exige retención > 0 para habilitar restores PITR en la rama. 6h
  # alcanza para un entorno efímero que se destruye al terminar cada sesión.
  history_retention_seconds = 21600

  branch = {
    name          = "preprod"
    database_name = "preprod"
    role_name     = "preprod_app"
  }
}

locals {
  # Derivado de los atributos del proyecto, nunca hardcodeado.
  database_url = "postgresql://${neon_project.preprod.database_user}:${neon_project.preprod.database_password}@${neon_project.preprod.database_host}/${neon_project.preprod.database_name}?sslmode=require"

  # GOOGLE_* y FRONTEND_URL son opcionales para el backend (ver backend/cmd/api/main.go):
  # si faltan, el login con Google simplemente no funciona y el CORS cae a
  # localhost, pero el proceso arranca igual. Por eso sólo se agregan al mapa
  # de env vars cuando el usuario les puso un valor.
  api_optional_env_vars = merge(
    var.frontend_url != "" ? { FRONTEND_URL = { value = var.frontend_url } } : {},
    var.google_client_id != "" ? { GOOGLE_CLIENT_ID = { value = var.google_client_id } } : {},
    var.google_client_secret != "" ? { GOOGLE_CLIENT_SECRET = { value = var.google_client_secret } } : {},
    var.google_redirect_url != "" ? { GOOGLE_REDIRECT_URL = { value = var.google_redirect_url } } : {},
    var.google_sheets_redirect_url != "" ? { GOOGLE_SHEETS_REDIRECT_URL = { value = var.google_sheets_redirect_url } } : {},
  )
}

# --- Backend (Render) --------------------------------------------------------

resource "render_web_service" "api" {
  name   = "ingsoft3-api-preprod"
  plan   = "free"
  region = "oregon"

  health_check_path = "/health"

  runtime_source = {
    image = {
      image_url = "ghcr.io/max20050/ingsoft3-backend"
      tag       = var.image_tag
    }
  }

  env_vars = merge(
    {
      DATABASE_URL              = { value = local.database_url }
      JWT_SECRET                = { value = var.jwt_secret }
      DATASOURCE_ENCRYPTION_KEY = { value = var.datasource_encryption_key }
      LLM_ENCRYPTION_KEY        = { value = var.llm_encryption_key }
      AI_SUMMARY_ENABLED        = { value = var.ai_summary_enabled }
    },
    local.api_optional_env_vars,
  )
}

# --- Frontend (Render) -------------------------------------------------------
# No puede haber dependencia circular con "api": BACKEND_URL se deriva de la
# URL que Render asigna a "api" (sólo se conoce después de crearlo), por eso
# "front" depende de "api" en un solo sentido. Ver nota de FRONTEND_URL arriba.

resource "render_web_service" "front" {
  name   = "ingsoft3-front-preprod"
  plan   = "free"
  region = "oregon"

  health_check_path = "/"

  runtime_source = {
    image = {
      image_url = "ghcr.io/max20050/ingsoft3-frontend"
      tag       = var.image_tag
    }
  }

  env_vars = {
    BACKEND_URL = { value = render_web_service.api.url }
  }
}
