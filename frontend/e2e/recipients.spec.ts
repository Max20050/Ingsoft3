import { expect, test, type Page } from "@playwright/test";

// Registra una cuenta nueva (única por test, con la hora adentro) y espera
// a aterrizar en el dashboard: deja la sesión en localStorage, como lo haría
// un usuario real.
async function registrarse(page: Page, email: string) {
  await page.goto("/register");
  await page.getByLabel("Email").fill(email);
  await page.getByLabel("Password").fill("contraseña-e2e-123");
  await page.getByRole("button", { name: "Sign up" }).click();
  await expect(page).toHaveURL(/\/dashboard/);
}

test("crear un destinatario lo muestra en la lista, y borrarlo lo saca", async ({ page }) => {
  const cuenta = `e2e-alta-${Date.now()}@example.com`;
  const email = `destinatario-${Date.now()}@example.com`; // único: no choca con otra corrida
  const nombre = `E2E ${Date.now()}`;

  await registrarse(page, cuenta);
  await page.getByRole("link", { name: "Recipients", exact: true }).click();
  await expect(page).toHaveURL(/\/recipients/);

  await page.getByLabel("Email").fill(email);
  await page.getByLabel("Name", { exact: true }).fill(nombre);
  await page.getByRole("button", { name: "Save recipient" }).click();
  await expect(page.getByText(nombre)).toBeVisible(); // el dato que ESTE test creó, sin sleeps

  await page.getByRole("button", { name: `Delete recipient ${email}` }).click();
  await expect(page.getByText(nombre)).toHaveCount(0); // y el borrado, comprobado
});

test("un login con credenciales inválidas muestra el error y no entra", async ({ page }) => {
  await page.goto("/login");
  await page.getByLabel("Email").fill(`no-existe-${Date.now()}@example.com`);
  await page.getByLabel("Password").fill("no-es-la-clave");
  await page.getByRole("button", { name: "Log in" }).click();

  // Next.js monta su propio role="alert" vacío para anunciar cambios de ruta
  // a lectores de pantalla; filtramos por el que de verdad tiene texto.
  const errorAlert = page.getByRole("alert").filter({ hasText: /\S/ });
  await expect(errorAlert).toBeVisible(); // el usuario ve el error…
  await expect(page).toHaveURL(/\/login/); // …y no entró
});

test("el destinatario y la sesión sobreviven a recargar la página", async ({ page }) => {
  const cuenta = `e2e-reload-${Date.now()}@example.com`;
  const email = `reload-${Date.now()}@example.com`;
  const nombre = `Reload ${Date.now()}`;

  await registrarse(page, cuenta);
  await page.getByRole("link", { name: "Recipients", exact: true }).click();
  await page.getByLabel("Email").fill(email);
  await page.getByLabel("Name", { exact: true }).fill(nombre);
  await page.getByRole("button", { name: "Save recipient" }).click();
  await expect(page.getByText(nombre)).toBeVisible();

  await page.reload();

  // Ni la sesión (localStorage) ni el dato (la base) dependen del recargo:
  // si esto se rompe, nadie puede refrescar la página sin perder su trabajo.
  await expect(page).toHaveURL(/\/recipients/);
  await expect(page.getByText(nombre)).toBeVisible();

  await page.getByRole("button", { name: `Delete recipient ${email}` }).click();
  await expect(page.getByText(nombre)).toHaveCount(0);
});
