output "preprod_api_url" {
  description = "URL pública del backend en preprod."
  value       = render_web_service.api.url
}

output "preprod_front_url" {
  description = "URL pública del frontend en preprod."
  value       = render_web_service.front.url
}

output "neon_project_id" {
  description = "ID del proyecto Neon de preprod."
  value       = neon_project.preprod.id
}

output "database_url" {
  description = "Connection string de preprod, derivada de los atributos del proyecto Neon."
  value       = local.database_url
  sensitive   = true
}
