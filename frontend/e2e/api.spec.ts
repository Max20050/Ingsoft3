import { expect, test, type APIRequestContext } from "@playwright/test";

// La api tiene su propia dirección: NO es la del front. Sin barra al final.
// Local (compose): comparte host con el front, a través del proxy /api/ de
// nginx (no hay puerto propio publicado). En QA/PROD es la URL pública de
// la api de ESE entorno.
const API = process.env.API_BASE_URL || "http://localhost";

let token: string;

// Un solo usuario para todo el archivo: lo que estas pruebas ejercitan es
// /api/recipients, no el alta de cuentas — registrar una vez evita dejar
// una fila de usuario de más por test.
test.beforeAll(async ({ request }) => {
  const email = `e2e-integracion-${Date.now()}@example.com`;
  const res = await request.post(`${API}/api/auth/register`, {
    data: { email, password: "contraseña-e2e-123" },
  });
  expect(res.status()).toBe(201);
  const body = await res.json();
  token = body.token;
});

function authHeaders() {
  return { Authorization: `Bearer ${token}` };
}

async function listRecipients(request: APIRequestContext) {
  const r = await request.get(`${API}/api/recipients`, { headers: authHeaders() });
  expect(r.status()).toBe(200);
  return await r.json();
}

test("el alta guarda en la base de verdad, y el borrado la saca", async ({ request }) => {
  const email = `api-${Date.now()}@example.com`; // único: no choca con otra corrida

  const alta = await request.post(`${API}/api/recipients`, {
    headers: authHeaders(),
    data: { email, name: "Integración TP7", attributes: {} },
  });
  expect(alta.status()).toBe(201); // la api dice «creado»
  const creado = await alta.json();

  const lista = await listRecipients(request); // y la BASE lo devuelve: no es un doble
  expect(lista.some((r: { email: string }) => r.email === email)).toBe(true);

  const borrado = await request.delete(`${API}/api/recipients/${creado.id}`, {
    headers: authHeaders(),
  });
  expect(borrado.status()).toBe(204); // «borrado, no hay nada que devolver»

  const despues = await listRecipients(request); // y el borrado, comprobado
  expect(despues.some((r: { email: string }) => r.email === email)).toBe(false);
});

test("un email vacío lo rechaza la api, y no crea nada", async ({ request }) => {
  const antes = await listRecipients(request);

  const alta = await request.post(`${API}/api/recipients`, {
    headers: authHeaders(),
    data: { email: "", name: "no debería existir", attributes: {} },
  });
  expect(alta.status()).toBe(400); // el contrato del error

  const despues = await listRecipients(request);
  expect(despues.length).toBe(antes.length); // y de verdad no se creó nada
});

test("borrar un destinatario que no existe devuelve 404", async ({ request }) => {
  // Un id con formato válido pero que no corresponde a ningún recipient de
  // este usuario: el store hace DELETE ... WHERE id = $1 AND user_id = $2,
  // y esa comprobación sólo la hace la base de verdad, no un doble.
  const borrado = await request.delete(`${API}/api/recipients/00000000-0000-0000-0000-000000000000`, {
    headers: authHeaders(),
  });
  expect(borrado.status()).toBe(404);
});
