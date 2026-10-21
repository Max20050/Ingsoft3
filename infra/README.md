# infra/ — entorno `preprod` (TP8)

Declara, sólo para `preprod`, una base en Neon y dos servicios en Render
(backend y frontend) corriendo la imagen ya promovida a QA. No toca QA ni
PROD, que siguen manejados a mano por los deploy hooks de `ci.yml`.

## Antes de la primera vez

```bash
export NEON_API_KEY="..."
export RENDER_API_KEY="..."
export RENDER_OWNER_ID="..."

cd infra
cp terraform.tfvars.example terraform.tfvars   # completar valores reales, nunca commitear
terraform init
```

## Ciclo normal de trabajo

```bash
terraform fmt
terraform validate
terraform plan
terraform apply

# probar preprod: terraform output preprod_api_url / preprod_front_url

terraform destroy    # SIEMPRE al terminar la sesión
```

Render free tier comparte 750 hs/mes de cómputo entre todos los servicios del
workspace (no es por servicio). Con `preprod` arriba se suman 2 servicios más
a QA y PROD — no dejarlo corriendo entre sesiones para no arriesgar que PROD
se quede sin horas y Render lo suspenda.

**Llegar a la defensa con `preprod` destruido** (`terraform destroy` corrido
y confirmado): el ciclo completo se hace en vivo frente al docente.

## Demostración de `prevent_destroy` (opcional, para la defensa)

Para mostrar el safety net, agregar temporalmente a `neon_project.preprod` en
`main.tf`:

```hcl
lifecycle {
  prevent_destroy = true
}
```

Correr `terraform destroy` (falla a propósito), sacar el bloque, y volver a
destruir. No se deja commiteado porque bloquearía el ciclo normal de
`destroy`/`apply` entre sesiones.

## Por qué no hay `neon_role`/`neon_database` sueltos

`neon_project` ya permite nombrar el branch, la base y el rol por defecto
(bloque `branch`), y expone `database_host`/`database_user`/
`database_password`/`database_name` como atributos de sólo lectura. Usar esos
atributos directamente evita instanciar recursos adicionales sin necesidad.

## Por qué `FRONTEND_URL` del backend es opcional

`front` depende de `api` (necesita su URL para `BACKEND_URL`, que nginx usa
en cada request a `/api/`). Si `api` dependiera a su vez de `front.url` para
`FRONTEND_URL` sería una dependencia circular que Terraform no puede
resolver. El backend sólo usa `FRONTEND_URL` para el header CORS y cae a
`http://localhost:3000` si falta — no bloquea el arranque ni los smoke
tests por curl. Se puede pisar a mano en `terraform.tfvars` después del
primer `apply`, una vez que se conoce la URL real del frontend.
