variable "image_tag" {
  description = "Tag sha-<commit> de las imágenes en ghcr.io/max20050/ingsoft3-{backend,frontend} a desplegar en preprod (sacado de un merge a main ya promovido a QA)."
  type        = string
}

variable "jwt_secret" {
  description = "Secreto para firmar JWT en el backend de preprod."
  type        = string
  sensitive   = true
}

variable "datasource_encryption_key" {
  description = "Clave AES en base64 (32 bytes) para cifrar credenciales de datasources. Generar con: openssl rand -base64 32"
  type        = string
  sensitive   = true
}

variable "llm_encryption_key" {
  description = "Clave AES en base64 (32 bytes) para cifrar credenciales de LLM. Generar con: openssl rand -base64 32"
  type        = string
  sensitive   = true
}

variable "ai_summary_enabled" {
  description = "Habilita el resumen con IA (AI_SUMMARY_ENABLED) en preprod."
  type        = string
  default     = "false"
}

variable "frontend_url" {
  description = "URL pública del frontend, usada por el backend sólo para el header CORS. Opcional: se desconoce en el primer apply porque crearía una dependencia circular con el servicio del frontend (ver comentario en main.tf)."
  type        = string
  default     = ""
}

variable "google_client_id" {
  description = "Google OAuth client ID. Opcional: sin esto, el login con Google simplemente no funciona en preprod."
  type        = string
  default     = ""
  sensitive   = true
}

variable "google_client_secret" {
  description = "Google OAuth client secret. Opcional."
  type        = string
  default     = ""
  sensitive   = true
}

variable "google_redirect_url" {
  description = "Redirect URL de Google OAuth para login. Opcional."
  type        = string
  default     = ""
}

variable "google_sheets_redirect_url" {
  description = "Redirect URL de Google OAuth para Sheets. Opcional."
  type        = string
  default     = ""
}
