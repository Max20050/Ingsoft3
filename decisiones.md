# Decisiones — TP1

## 1. Por qué Git no pudo resolver el conflicto solo — y qué habría tenido que pasar para que nunca apareciera.
Git no puede resolver el conflicto por su propia cuenta ya que no sabe cual es la intencion de la persona que pusheo.
Para que el problema nunca hubiera aparecido lo que se puede hacer es traer los cambios del main antes de hacer la pull request. Resolves conflictos antes de pushear.

## 2. Qué problemas encontraste y cómo los solucionaste.
El unico problema que tuve y se pueden ver en los commits es que me olvide de activar el ruleset entonces el primer commit si paso directo a produccion.
despues active el ruleset y ya funciono.

## 3. Declaración de uso de IA: qué partes hiciste con ayuda de inteligencia artificial y cómo verificaste lo que te devolvió.
Hasta ahora no utlice la IA para mucho mas que buscar comandos.

# Decisiones — TP2

## 1. Porque elegi esta app?
Docuwave es una app que estoy construyendo activamente y que conozco muy bien. Ademas cumple con los requerimientos de estructura del proyecto (Front + back + db).
Aparte de todo esto es una app en la cual ya tengo CI y test implementados. Trabajo con Github Flow.

## Decisiones de contenerización:
Las imagenes base elegidas son:
- Backend (go), en etapa de build golang:1.24-alpine ya que esta incluye todas las herramientas necesarias para que el proyecto compile. Es muy importante utilizar esta version ya que la 1.23 no compila por falta de funcionalidades.
Luego en la etapa de build se utiliza la alpine:3.20. Go compila a binario estatico por la imagen de ejecucion no necesita runtime alguno. Solo se necesita el binario y el cerificado TLS para las llamadas como las de OAuth.
- Frontend (Next.js), todas las etpas del multistage utilizan el node:20-alpine esto es para evitar incompatibilidades de binarios nativos entre etapas.

Etapas: 
- Backend: Utiliza 2 etapas:
builder en donde se copia go.mod y go.sum y despues se corre go mod download esto es para cachear las dependencias y no descargalas en cada build. Luego copia el codigo y lo compila.
Despues esta la etapa final la cual copia unicamente el binario compilado.
- Frontend: utiliza 3 capas:
La primera capa "deps" instala las dependencias con npm ci utilizando el package.json, esto lo hace esta capa para una vez para cachear las dependencias y no tener que instalarlas en cada build.
La etapa de "builder" utiliza las deps cacheadas y copia el codigo fuente y corre npm run build. 
Luego la etapa de "runner" copia lo minimo para poder ejecutar (.next/standalone, .next/static y la carpeta public). Esta etapa no copia el codigo fuente ni "node_modules".

Que persiste y que no?:
Lo unico que persiste es el volumen de la base de datos de postgres. Este solo se borra si se hace de manera manual o si se agrega la flag -v en el docker compose down

- Se utiliza el dns interno de docker compose.

## 2. Qué problemas encontraste y cómo los solucionaste.
No me encontre con muchos problemas ya que yo ya contaba con los Dockerfiles del proyecto, si tuve problemas con las creedenciales de github ya que no me dejaba iniciar sesion para subir las imagenes compiladas.

## 3. Declaración de uso de IA: qué partes hiciste con ayuda de inteligencia artificial y cómo verificaste lo que te devolvió.
Para este tp utilize a la IA como guia paso a paso para crear la imagenes y subirlas como package de github. Pude verifcar que lo que me daba estaba bien a traves de las consignas del tp ya que fui comparando paso a paso que todo estaba correcto.

# Decisiones — TP3

## 1. Decision tiempo de la sprint:
Se decidio que la sprint va a durar 1 semana ya que esto es el tiempo en el cual se desarollan las actividades de la materia, un tp por semana. Por lo que el tiempo limite para desarollar el tp4 (teoricamente) es hasta el jueves 3 de septiembre.

## Decision de la cantidad de tareas que se pueden hacer al mismo tiempo:
Se decidio solo tener 2 tareas como maximo al mismo tiempo para que no se acumule trabajo sin terminar que el que peudo gestionar. Este numero viene de utlizar la regla de participantes + 1, este 1 nos permite tener flexibilidad opertiva ya que si ocurre un bloqueo o queremos preparan una tarea antes de terminal la que esta en progreso esto nos permite hacerlo.

## Decision de las tareas para la HU-1:
Yo ya tengo una implementacion de un workflow de CI en el repo orginal del proyecto [docuwave](https://github.com/max20050/DocuWave) por lo que la primer tarea para mi sera migrar ese archivo .yml al repo actual.
Despues puse como segunda tarea agregar el badge del CI al readme.md

## 2. Qué problemas encontraste y cómo los solucionaste.
En este tp no hubo problemas ya que segui la guia paso a paso e implemnte todo lo que pedia. 

## 3. Declaración de uso de IA: qué partes hiciste con ayuda de inteligencia artificial y cómo verificaste lo que te devolvió.
Para este tp no utilice la IA.

# Decisiones — TP4

## 1. Estructura elegida del pipeline
 
El workflow (`.github/workflows/ci.yml`) corre en cada Pull Request contra main y en
cada push a main. Se definieron dos jobs independientes:
 
- `build-backend`: construye la imagen del backend (Go) usando el Dockerfile de `backend/`.
- `build-frontend`: construye la imagen del frontend (Next.js) usando el Dockerfile de `frontend/`.
Son dos jobs porque la app tiene dos Dockerfiles reales (uno por componente). Cada imagen se construye con un proceso de build totalmente distinto (Go compila a un binario estático, Next.js
necesita instalar dependencias de Node y correr un build de la aplicación), así que
tiene sentido que sean pasos separados en vez de mezclarlos en un mismo job.
 
Corren en paralelo porque no hay ningúna dependencia entre ellos para compilar. Esto además acorta el tiempo total del pipeline: en vez de sumar el tiempo de build de
las dos imágenes, GitHub Actions las corre en runners simultáneos y el tiempo total es
el del job más lento, no la suma de ambos.
 
## 2. Qué cachea el pipeline
 
Se usa el backend de cache de Buildx `type=gha` (cache nativa de GitHub Actions), con
un `scope` separado por job (`scope=backend` y `scope=frontend`) para que no se pisen
entre sí.
 
**Qué se reutiliza:**
- Las capas de dependencias que no cambiaron entre corridas. Por ejemplo, en el
  backend, la capa de `go mod download` (o `go mod tidy`) se reutiliza si el
  `go.mod`/`go.sum` no cambió. En el frontend, la capa de `npm install` /
  `npm ci` se reutiliza si `package.json` / `package-lock.json` no cambiaron.
- Cualquier capa intermedia del Dockerfile cuyo contexto de entrada (los archivos que
  copia esa instrucción `COPY`) no haya sido modificado respecto a la corrida anterior.
  Esas capas aparecen como `CACHED` en el log del build.
**Qué NO se reutiliza:**
- Las capas posteriores a la primera línea del Dockerfile que cambió. Docker cachea
  capa por capa en orden: si cambia el código fuente (por ejemplo un `.go` o un `.tsx`),
  todo lo que dependa de ese `COPY` en adelante se reconstruye (compilación del
  binario, build de Next.js), aunque las dependencias de más arriba sigan cacheadas.
- Si cambian `go.mod`/`go.sum` o `package.json`/`package-lock.json`, se invalida
  también la capa de instalación de dependencias, y todo lo posterior a esa capa.
**Qué pasa si el cache desaparece** (por ejemplo, expira, se borra manualmente, o es la
primera corrida del workflow en el repo): el build simplemente corre completo, sin
ningún `CACHED` en el log, exactamente igual que en un ambiente limpio. No rompe el
pipeline ni el resultado del build; el único costo es que la corrida tarda más porque
tiene que descargar dependencias y reconstruir todas las capas desde cero. La próxima
corrida vuelve a poblar la cache normalmente.
 
## 3. Por qué el pipeline construye con el Dockerfile en vez de compilar por su cuenta:

La razón es que el pipeline tiene que verificar exactamente lo que se va a desplegar,
no una aproximación. Si el CI compilara "a mano" con el toolchain del runner
(su propia versión de Go o de Node, sus propias variables de entorno), podría pasar
que el pipeline esté verde pero la imagen Docker real falle al construirse

## 4. Problemas encontrados y cómo los resolví:
No fue un problema pero tuve que cambiar bastante la estructura de ci.yml ya que en docuwave yo no publico aun el package del build, si hago que pase los tests y pruebo que las imagenes compilen bien antes de mergear. Ademas borre algunas funcionalidades que no entran en este tp como publicar los resultados de los tests.

## 5. Declaración de uso de IA

Usé Claude para generar un borrador inicial del ci.yml. Luego ajusté manualmente los nombres de los jobs, los paths de los Dockerfiles
y verifiqué el comportamiento del cache corriendo el pipeline dos veces en mi propio
repo antes de dar esto por válido.

## 6. Algunas cosas del tp3 las resolvi aca:
La parte de generar un bug y resolverlo y que quede reflejada en el tablero la resolvi en este tp. Esto es para que se pueda aprovehcar ese bug y probar que el build falle y no nos deje mergear. Luego agrege el bug como issue, publique la solucion y lo cerre.
![Build Fallando por error](capturas/build-fail.png)

# Decisiones — TP5

> Esta sección se va completando a medida que avanzo (como pide la consigna). Lo que
> todavía no tiene link es porque todavía no lo hice — lo marco como **Pendiente**.

## 1. Qué lógica elegí testear y por qué

Backend — de todo lo que ya tenía testeado (el proyecto ya tenía bastante suite de antes
de este TP), elegí citar estas 4 reglas porque son las que más duelen si se rompen:

- **Hashing de contraseñas** (`internal/auth`): si esto se rompe, se filtran contraseñas o
  se deja entrar a cualquiera. Es lo más crítico de seguridad que tengo.
- **Emisión/verificación de JWT** (`internal/auth`): es el guardián de cada endpoint
  autenticado. Un bug acá es un login falso o una sesión que no expira.
- **Compilación de queries** (`internal/query`): acá se arma el SQL real contra la base del
  usuario a partir de lo que mandó el frontend. Si esto falla mal, es una inyección SQL o
  un reporte con datos de otro usuario.
- **El runner le pasa el query compilado al connector tal cual** (`internal/report`): es el
  único camino por el que pasa CADA reporte (preview, descarga, programado). Si acá se
  llama dos veces o con el query equivocado, el usuario recibe datos mal.

Para el **mock obligatorio** elegí `datasource.Connector` (la interfaz que usa
`runnable.run`): es el único boundary externo de verdad que tengo bien aislado detrás de
una interfaz (habla con Postgres/MySQL/Sheets/REST según el tipo de fuente). Armé un
`spyConnector` a mano que graba cuántas veces y con qué argumentos se llamó `RunQuery`, y
el test (`TestRunnableRunCallsConnectorWithTheCompiledQuery`) verifica la interacción, no
solo el resultado — por eso es un mock y no un fake como los que ya tenía
(`fakeCustomSource`, `stubTemplate`, que solo devuelven datos fijos).

Frontend — mi app no tiene un archivo de lógica pura como el `tareas.js` del ejemplo:
`lib/api.ts` es casi todo wrappers CRUD contra el backend, y la UI vive en componentes
`"use client"`. Dentro de `report-builder.tsx` había funciones sueltas, ya puras, que
nadie testeaba porque vivían pegadas al componente: `errorMessage`, `sameColumns`,
`operatorArity`, `columnBadge`. Las saqué a `lib/report-builder-helpers.ts` (mismo
comportamiento, sin tocar nada de lo que hacen) para poder testearlas sin DOM. Para el
mock, mockeé `global.fetch` en `lib/api.ts` y verifiqué que `login`/`authFetch` lo llaman
con la URL, método, headers y body correctos — es la misma idea del `pendientesDe` de la
guía, pero sin tener que inyectar un parámetro nuevo porque `fetch` ya es una dependencia
global reemplazable.

## 2. Umbral de coverage

**Backend: 44% sobre statements, total agregado** (no por paquete). Medido hoy:
**46.0%**. Elegí statements porque Go no mide branch coverage de forma nativa — ni
`go test -cover` ni `go tool cover` tienen un modo de rama; es una limitación real del
toolchain, no una elección mía (la guía lo permite explícitamente para stacks donde no
existe). Puse el umbral un poco por debajo de mi medición real (46.0%) para que no
arranque roto ni sea tan ajustado que una corrida con una sola función nueva sin tests ya
lo tire — hice la cuenta: con 3509 statements contados hoy y 1613 cubiertos, hacen falta
más de ~157 statements nuevos sin cubrir para bajar del 44%. Eso es una feature de tamaño
normal, no una trampa de 3 líneas — es justo lo que necesito para la demostración de la
Tarea 3 sin que sea ni imposible ni trivial de romper.

**Frontend: 90% de líneas y 90% de ramas**, sobre `lib/report-builder-helpers.ts`
únicamente. Hoy da **100% / 100%** en los dos. Acá vitest sí mide rama (a diferencia del
backend), así que el número de rama existe y lo uso como parte del umbral.

## 3. Qué dejé afuera de la cuenta de cobertura, y por qué

**Backend:**
- `cmd/api` — es el arranque: wiring de rutas, middlewares y configuración. No tiene
  reglas de negocio, y si está mal el servidor directamente no levanta.
- `internal/migrate` — aplica archivos `.sql` embebidos contra Postgres. No tiene ninguna
  regla que verificar (es "ejecutá estos archivos en orden"), y testearlo de verdad
  requeriría una base de datos real, lo que lo vuelve un test de integración, no unitario.

Los excluí filtrando los paquetes antes de correr `go test` (`go list ./... | grep -v
'/cmd/api$|/internal/migrate$'`), ya que Go no tiene un atributo tipo
`[ExcludeFromCodeCoverage]` — la exclusión es a nivel paquete, no a nivel clase. Ninguno de
los dos paquetes tenía ninguna regla movida a otro lado para "esconderla": están vacíos de
lógica de negocio desde que se escribieron.

**Frontend:**
- `lib/api.ts` — son ~890 líneas, la gran mayoría wrappers CRUD casi idénticos
  (`listDataSources`, `createReport`, `deleteRecipient`...) que solo llaman a `authFetch` y
  parsean la respuesta, sin ninguna rama propia. Sí tiene tests (`lib/api.test.ts`, el
  mock de `login`/`authFetch`), pero no cuentan para el umbral porque medirían sobre todo
  el boilerplate sin probar, no sobre lógica.
- `auth-context.tsx` — wiring de React Context, sin ninguna regla de negocio.

## 4. Por qué coverage alto no garantiza calidad (con mi ejemplo)

El caso real lo tengo en mi propio reporte: `wholeNumber` (en `internal/query/resolve.go`)
tenía 30% de cobertura de statements mientras el paquete entero estaba en 88%+ — ese 88%
alto escondía una función con casi ninguna rama probada. Si en vez de eso alguien hubiera
escrito un test que solo llama a `wholeNumber(float64(7))` sin ningún `assert` sobre el
resultado, esa línea igual queda "cubierta" (se ejecutó), y el número de cobertura del
paquete no se mueve ni un poco — pero no demuestra que la función rechace un valor
fraccionario como `7.5`, que es exactamente el bug que encontré en el punto 5. Cobertura
mide que una línea corrió, no que alguien comprobó qué devolvió.

## 5. El ejercicio de la rama sin cubrir (§3.0)

- **Línea**: `internal/query/resolve.go:383`, dentro de `wholeNumber` —
  `if typed != float64(int(typed))`, la rama que rechaza un `float64` que no es un número
  entero.
- **Entrada que la recorre**: un filtro `last_days` con un valor fraccionario, por ejemplo
  `7.5` días.
- **Qué decidí**: lo agregué. `TestCompileRejectsFractionalLastDays` en
  `internal/query/compile_test.go` — subió `wholeNumber` de 30.0% a 40.0% de statements.
  Lo elegí porque es exactamente el tipo de bug silencioso que preocupa: antes de este
  test, nada impedía que `7.5` se redondeara o truncara silenciosamente a un reporte con
  una ventana de fechas distinta a la que el usuario pidió.

## 6. Mi stack no es el de la cátedra (.NET + vitest) — backend es Go

| Fila de la tabla | Lo que usé |
|---|---|
| Dónde viven los tests | al lado del código que prueban, `algo_test.go` (es la convención de Go, no una carpeta aparte) |
| Un test parametrizado | tabla de casos (`[]struct{...}`) + `t.Run` por caso — equivalente a `[Theory]`+`[InlineData]` |
| Que la dependencia entre desde afuera | interfaz de Go (`datasource.Connector`) recibida por `runnable`, ya existía |
| Fabricar el doble (mock) | no usé ninguna librería (no hay Moq en Go estándar): escribí `spyConnector` a mano implementando la interfaz y grabando las llamadas |
| Medir la cobertura | `go test -coverprofile=coverage.out` + `go tool cover -func=coverage.out` |
| Umbral que rompe el build | no existe un flag nativo (a diferencia de `coverlet.msbuild`): escribí `backend/scripts/check-coverage.sh`, que lee el total de `go tool cover` y compara a mano — tal cual lo anticipa la guía para stacks sin esa bandera |
| Qué entra en la cuenta | filtrado de paquetes con `go list ./... \| grep -v ...` antes de correr los tests — no hay atributo por clase, la exclusión es por paquete |

El frontend sí es el de la cátedra (Next.js usa vitest igual que el ejemplo), así que esa
mitad de la guía se siguió tal cual.

## 7. Si refactoricé para poder mockear

- **Backend**: no hizo falta ningún refactor. `runnable.connector` ya era
  `datasource.Connector`, una interfaz inyectada desde `Runner.prepare`. Solo escribí el
  spy y el test.
- **Frontend**: extraje `errorMessage`, `sameColumns`, `operatorArity` y `columnBadge` de
  `app/ui/report-builder.tsx` a `lib/report-builder-helpers.ts`. No cambié qué hacen —
  verifiqué con `npm run type-check` y `npm run lint` que el comportamiento quedó
  idéntico — solo las saqué del componente para poder importarlas sin renderizar nada.

## 8. Problemas encontrados y cómo los resolví

- El `.gitignore` de la raíz tenía una regla `scripts` (sin la barra inicial), que en
  sintaxis de `.gitignore` ignora **cualquier** carpeta llamada `scripts` en cualquier
  parte del repo — incluida `backend/scripts/`, donde iba el script del umbral. Si no lo
  encontraba, el script se hubiera commiteado "bien" en mi máquina y desaparecido en
  cualquier otro lado (exactamente la clase de bug silencioso que advierte la guía). Lo
  acoté a `/scripts` (solo la raíz), que es lo único que necesitaba estar ignorado.
- Al instalar vitest, `npm i -D vitest` sin versión trajo vitest 5, que pide
  `@types/node` 22+/24+ como peer — mi proyecto tiene `@types/node` fijado en `^20` (por
  Next 16) y el install fallaba con `ERESOLVE`. Instalé `vitest@3` explícitamente (mismo
  problema que la guía anticipa para el ejemplo de cátedra) y después
  `@vitest/coverage-v8@3` para que coincida la versión mayor.

## 9. Declaración de uso de IA

_(pendiente — lo escribo yo a mano)_

---

## 10. La demostración de la Tarea 3

**Primer PR — cuenta la historia completa (rojo → fix → verde → merge):**
[#18](https://github.com/Max20050/Ingsoft3/pull/18). Agregué `filterSummary` en
`lib/report-builder-helpers.ts` — una función con 5 caminos (none/pair/many/count/one) que
describe en una frase cada fila de filtro, para un lector de pantalla — y la subí sin un
solo test. Compiló perfecto y ningún test existente se rompió.

- Corrida roja, por cobertura: [`…/actions/runs/37655996935`](https://github.com/Max20050/Ingsoft3/actions/runs/37655996935) —
  `ERROR: Coverage for lines (52.94%) does not meet global threshold (90%)`. Rompió por
  **líneas**, no por ramas: la función todavía no la llamaba ningún test, y en vitest 3 una
  función que nadie invoca no suma a la cuenta de ramas (sólo a la de líneas sin cubrir) —
  es la misma distinción del §3.5 de la guía.
- Agregué un test por camino (5) más los casos que ejercitan cada `??` de respaldo (6 más,
  11 tests en total) — al llamar la función, sus ramas internas empezaron a contar, y la
  primera tanda de tests (sin los casos de respaldo) la dejó en 81.25% de rama, todavía por
  debajo del 90%. Hubo que cubrir también los `??`, no solo los 5 switch-case.
- Corrida verde después del fix, mismo PR: el check `Build frontend (Next.js)` pasó a
  `pass` y `mergeStateStatus` de la PR pasó de `BLOCKED` a `CLEAN`.
- **Pendiente**: el link al estado ya mergeado (lo mergea el dueño del repo, no el agente —
  regla del propio `AGENTS.md` del repo) y el link al segundo PR, chiquito, que queda
  abierto y en rojo hasta la defensa.

# Decisiones — TP6

## Enlaces de este TP

- **Paquetes públicos** (probado con un `docker pull`/`tags list` totalmente anónimo, sin
  `docker login` ni token — una llamada directa al registry con un token anónimo de
  `ghcr.io/token`, sin usar mi cuenta):
  - Backend: https://github.com/Max20050/Ingsoft3/pkgs/container/ingsoft3-backend —
    tag `sha-91fb71db674b0c479fde887e95fd27e8cbe60638`
  - Frontend: https://github.com/Max20050/Ingsoft3/pkgs/container/ingsoft3-frontend —
    mismo tag
- **La cadena de confianza del registry** (Tarea 1):
  1. PR con los tests en verde, donde «Entrar al registry» aparece **salteado** (el evento
     no es `push`, así que el `if` lo saltea, no un error):
     https://github.com/Max20050/Ingsoft3/actions/runs/38503034378/job/115654149935
  2. Corrida de `main` donde «Construir y publicar la imagen del backend» es el **último**
     paso del job (antes sólo hay `Post …` que agrega GitHub):
     https://github.com/Max20050/Ingsoft3/actions/runs/38503181350/job/115654603548
- **URLs vivas**:
  - QA: front https://ingsoft3-front-qa-dtom.onrender.com · api https://ingsoft3-api-qa-6umv.onrender.com
  - PROD: front https://ingsoft3-front-prod-5clx.onrender.com · api https://ingsoft3-api-prod-hb4s.onrender.com
- **Release**: https://github.com/Max20050/Ingsoft3/releases/tag/v6.0.0

> Mi stack no es el de la guía (.NET + Vite + nginx aparte): backend es **Go** y frontend es
> **Next.js**. Donde la guía asume algo específico de ese stack lo aclaro en el punto
> correspondiente — en general salió más simple de lo que la guía anticipa (Go lee
> `DATABASE_URL` como URI cruda, sin traducir a pares `Clave=Valor`; las migraciones corren
> solas con mi propio `migrate.Run()`, equivalente al `EnsureCreated()`).

## 1. Por qué el artefacto se publica sólo con la verificación en verde

Porque si el registry tuviera imágenes publicadas sin pasar los tests, la palabra
«publicado» dejaría de significar «pasó la verificación» y pasaría a significar
únicamente «alguien construyó esto alguna vez» — que es la trampa que describe la guía. En
mi `ci.yml` esto no es un `if` que lea el resultado de los tests: es **orden de pasos**. El
paso que construye y publica la imagen (`Construir y publicar la imagen del backend/frontend`)
es el **último** del job, después de «Build the backend/frontend tests stage», «Run … tests
with coverage» y el reporte de coverage. Si el `docker run` de los tests devuelve rojo, el
job muere ahí mismo y ese último paso directamente **no llega a ejecutarse** — no hace falta
ningún `if: success()` porque es el comportamiento por defecto de GitHub Actions (cortar al
primer error). Lo verifiqué mirando el log de una corrida real (la del punto «Enlaces»):
en la corrida de main los 10 pasos corrieron en ese orden exacto.

La cadena completa son tres eslabones, no uno: (1) nada entra a `main` sin el pipeline en
verde (gate del TP4/TP5), (2) `push: ${{ github.event_name == 'push' && github.ref ==
'refs/heads/main' }}` hace que sólo un push a `main` pueda llegar a publicar, y (3) el paso
que publica es el último del mismo job que corrió los tests. Ninguno de los tres solo
alcanza: sin el (3), un pipeline podría publicar primero y testear después y cumplir (1) y
(2) igual, con el registry lleno de imágenes rotas.

## 2. Continuous Delivery vs Continuous Deployment: cuál implementé

Implementé **Continuous Delivery**: todo push verde a `main` llega automáticamente a QA, y
de ahí a PROD sólo con una aprobación humana explícita (`environment: production` con
required reviewer). No es Continuous Deployment porque el último paso hacia producción no es
automático — hay un `Waiting for review` real en el medio, que yo mismo aprobé o rechacé.

Elegí Delivery y no Deployment porque mi red de seguridad automática no es lo
suficientemente madura para sacar al humano: mi cobertura de tests (TP5) es sobre lógica
puntual (hashing, JWT, compilación de queries), no sobre todo el sistema, y mi smoke test
sólo prueba que la app responde — no prueba que una feature nueva funcione bien de punta a
punta. Si sacara la aprobación humana hoy, estaría automatizando la propagación de errores a
producción sin tener la observabilidad (TP9) que avisaría a tiempo. Para llegar a Continuous
Deployment en serio me faltaría: monitoreo real en producción con alertas, un conjunto de
smoke/e2e tests que cubra los flujos críticos (no sólo "responde"), y feature flags para
poder apagar algo sin un rollback completo.

## 3. El diseño de la cadena (`needs`/`if`/`environment`) y el alcance de los secrets

```
build-backend    (sin needs)                              ← publica sólo si push a main
build-frontend   (sin needs)                               ← ídem
deploy-qa        needs: [build-backend, build-frontend]    · environment: qa
                 if: github.ref == 'refs/heads/main'         (los PR no despliegan)
deploy-prod      needs: deploy-qa                           · environment: production
                 (sin if propio — lo hereda de deploy-qa: si deploy-qa no corrió          
                  porque es un PR, deploy-prod tampoco corre)
                 concurrency: { group: deploy-prod, cancel-in-progress: false }
```

Los secrets de deploy viven **en el environment**, no en el repo:

| Secret | Environment | Quién lo puede leer |
|---|---|---|
| `RENDER_HOOK_API_QA`, `RENDER_HOOK_FRONT_QA` | `qa` | cualquier job con `environment: qa` (sin aprobación, QA es automático a propósito) |
| `RENDER_HOOK_API_PROD`, `RENDER_HOOK_FRONT_PROD` | `production` | sólo un job con `environment: production` **después** de que el reviewer lo aprueba |
| `GITHUB_TOKEN` | repo (automático) | lo emite GitHub por corrida, con los permisos que declaro en `permissions:` de cada job (`packages: write` sólo en `build-backend`/`build-frontend`, mínimo privilegio) |

Las connection strings de Neon (`DATABASE_URL` de `app_qa`/`app_prod`) **no son secrets de
GitHub**: son variables de entorno configuradas directamente en cada servicio de Render
(`ingsoft3-api-qa` / `ingsoft3-api-prod`). Mi pipeline nunca las toca ni las necesita — sólo
dispara el deploy hook, y Render arranca el contenedor con la configuración que ya tiene
guardada. Separar esto así es justo lo que hace que la misma imagen (o en este caso, el
mismo commit reconstruido, ver punto 6) sirva en los dos entornos: lo único que cambia entre
QA y PROD vive en Render y en Neon, no en GitHub ni en el código.

Si estos secrets estuvieran en el repo en vez del environment, cualquier job de cualquier
workflow (incluido uno que corre sobre un PR de un fork, si algún día acepto contribuciones
externas) tendría acceso a disparar un deploy a producción. Con el secret en el environment
`production`, sólo un job que declare `environment: production` **y** que pase la revisión
puede leerlo — es el mismo alcance de secrets de environment vs. repo que ya usamos para
separar QA de PROD.

## 4. Qué mira mi aprobador antes de aprobar

Antes de aprobar un deploy a producción, lo que reviso es: (1) que `deploy-qa` haya
terminado en verde en esa misma corrida (si QA no pasó, no tiene sentido promover ese
commit); (2) qué cambia ese commit — si toca algo sensible (credenciales, connection string,
esquema de datos) lo miro con más cuidado que un cambio de texto; (3) si hay más de una
corrida esperando aprobación a la vez, reviso cuál es la más reciente y rechazo las viejas a
mano (ver §3.4 de la guía: aprobar una corrida vieja después de una nueva hace retroceder
PROD). En la práctica de este TP usé estos criterios literalmente: rechacé la primera
corrida porque el smoke de QA había tardado sólo 5 segundos y quería confirmar en el log que
no era un falso verde por cache antes de promoverlo, y aprobé la segunda después de
confirmar que el smoke de QA había corrido bien y que el cambio (un texto del login) era de
riesgo mínimo.

Lo que el aprobador **no** puede ver, y es una limitación real: no tengo ninguna métrica de
producción (latencia, tasa de error, uso de CPU/memoria) delante en el momento de aprobar —
sólo tengo el resultado binario del smoke test de QA. Si algo degrada performance sin
romper el `/health`, no lo veo hasta después del deploy. Eso es exactamente lo que el TP9
(observabilidad) viene a cerrar.

## 5. La letra chica de mi free tier

- **Cold start**: los 4 servicios de Render duermen a los ~15 minutos sin tráfico; el primer
  request después de dormir tarda hasta ~1 minuto. Por eso el smoke test de `deploy-qa` y
  `deploy-prod` reintenta 30 veces cada 20 segundos (hasta 10 minutos) en vez de un `curl`
  seco — en la práctica, en mis corridas reales el smoke pasó en 5-33 segundos porque los
  servicios ya estaban despiertos de pruebas anteriores, pero el reintento está para el caso
  frío.
- **Horas de instancia** (750/mes por workspace) y **minutos de build** (500/mes): cada
  push a `main` reconstruye 2 imágenes en Render (backend y frontend de QA), y cada
  aprobación a PROD reconstruye 2 más — hasta 4 builds de Render por ciclo completo. Todavía
  no me quedé sin cupo, pero lo voy a estar mirando en *Workspace → Billing* a medida que se
  acerque la defensa, porque si se agota, Render deja de construir hasta fin de mes y el
  smoke seguiría dando verde contra la versión vieja sin que nada lo avise.
- **Neon**: el cómputo se suspende a los ~5 minutos de inactividad y se despierta solo, más
  rápido que Render. Hasta ahora no pegué contra el límite de almacenamiento (0.5 GB) ni de
  horas de cómputo mensuales.

## 6. Qué garantía pierdo porque Render reconstruye desde el repo

Mi pipeline publica una imagen etiquetada con el commit (`ghcr.io/.../ingsoft3-backend:sha-<commit>`),
pero el deploy hook de Render no usa esa imagen: le pide a Render que **vuelva a construir**
el mismo commit desde el Dockerfile del repositorio. Así que lo que corre en QA y PROD es
una **segunda construcción** del mismo código, no la imagen que mis tests verificaron. En la
práctica eso significa que si una dependencia de Go o de npm resuelve a una versión distinta
entre el build de GitHub Actions y el build de Render (porque alguna no está fijada con una
versión exacta, o una imagen base como `node:20-alpine`/`golang:1.24-alpine` se actualizó
entre un build y el otro), el binario que corre en producción podría no ser bit-a-bit
idéntico al que pasó los tests — aunque venga del mismo commit. Lo que promuevo es el
**commit verificado**, no la **imagen verificada**. Convertir esa garantía en real —que
Render (o lo que sea) despliegue literalmente la imagen que publiqué en el §3.0, no una
reconstrucción— es, según la guía, exactamente el problema que resuelve el TP7.

## 7. Qué prueba mi smoke test y qué no

Mi smoke test (en `deploy-qa` y `deploy-prod`) pega a `GET /health` y a `GET /` del front, en
loop con reintentos. Le agregué algo que el ejemplo original no tenía: `/health` ahora hace
`pool.Ping(ctx)` contra la base antes de contestar `ok` (si la conexión falla, devuelve 503)
— así un `DATABASE_URL` mal copiado entre QA y PROD (el error más silencioso que señala la
guía) hace fallar el smoke en vez de dar un falso verde. Lo que el smoke **no** prueba: no
confirma que la *feature* que acabo de desplegar funcione — sólo que el proceso levantó y
que puede hablar con su base. Tampoco me dice **qué versión** está respondiendo: si Render
sigue sirviendo el build anterior mientras el nuevo termina de construir (la guía lo marca
como un riesgo real: el verde puede llegar antes de que el build nuevo esté listo), mi smoke
igual daría verde. No implementé todavía que `/health` devuelva el commit que corre y que el
smoke lo compare contra `github.sha` — queda señalado como **pendiente**, la guía lo marca
como opcional en el TP6 y obligatorio en el TP7.

## 8. Deployment pattern para producción real + plan de rollback

Para esta app en una producción real con usuarios eligiría **blue-green**: tengo dos
entornos completos (QA/PROD ya los tengo separados) y el riesgo de un deploy que rompe algo
crítico —como la conexión a una base de datos ajena en `datasource.Connector`, o el cifrado
de credenciales— lo quiero poder revertir al instante, no de a tandas (que es lo que ofrece
rolling). No elegiría canary todavía porque no tengo la observabilidad (TP9) para decidir
automáticamente «el canario sangra, corta el tráfico» — sin métricas, canary es repartir
tráfico a ciegas. Un caso donde sí usaría **feature flags**: liberar `AI_SUMMARY_ENABLED`
gradualmente por usuario en vez del booleano global que tengo hoy, para probar el costo real
de los resúmenes de IA con un grupo chico antes de prenderlo para todos.

**Plan de rollback actual**, paso a paso: (1) identificar el commit bueno anterior —
`gh api 'repos/.../deployments?environment=production' --jq '.[0].sha'` me da el que está
live, el anterior lo saco de `git log` o de la release previa; (2) disparar los mismos dos
hooks de PROD con `&ref=<ese-sha>` (no hace falta aprobación de GitHub para esto: ya no pasa
por el workflow, es un `curl` directo — lo cual es en sí mismo una limitación: el rollback de
emergencia *evita* el gate humano a propósito, para no tener que esperar un run completo);
(3) esperar en Render → `ingsoft3-api-prod`/`ingsoft3-front-prod` → **Deploys** a que ese
commit figure como **live**, no fiarme del `/health` (contestaría verde con la versión que
estoy revirtiendo).

**Pendiente**: todavía no ejecuté un rollback real para medir el tiempo — lo que tengo es el
plan y el comando, no el número cronometrado que pide la Tarea 6 (la guía es explícita: tiene
que ser medido, no estimado). Lo voy a hacer antes de la defensa y voy a completar este punto
con la hora de inicio y la hora de *live* que muestre Render. Lo que el rollback de código
**no** deshace, en cualquier caso: si llegara a desplegar una migración que borra o altera una
columna, revertir el código no recupera esos datos — para eso haría falta un backup de Neon
restaurado a un punto anterior, que es una operación separada del deploy.

## 9. Problemas encontrados y cómo los resolví

- **nginx con el host del backend hardcodeado**: el frontend (agregado en un PR anterior,
  #17, que metió nginx como reverse proxy único dentro del contenedor del frontend) tenía
  `proxy_pass http://backend:8080;` fijo en `frontend/nginx/default.conf`. Ese nombre sólo
  existe en la red interna de `docker-compose`; en Render, nginx no podía resolverlo y el
  contenedor moría al arrancar (`host not found in upstream "backend"`). Lo resolví con el
  mismo patrón que usa la guía para el nginx de Vite, pero manual (esta imagen no usa la
  imagen oficial de nginx, instala el paquete de Alpine a mano): renombré el archivo a
  `default.conf.template`, metí `${BACKEND_URL}` en el `proxy_pass`, y en
  `docker-entrypoint.sh` corro `envsubst '${BACKEND_URL}'` **antes** de levantar nginx —
  limitando la sustitución a esa sola variable para no pisar las variables propias de nginx
  del mismo archivo (`$host`, `$remote_addr`, etc., que `envsubst` sin lista de por medio
  también reemplazaría por vacío). `BACKEND_URL` ahora es una variable de entorno de Render,
  distinta por servicio, con default `http://backend:8080` para no romper mi
  `docker-compose.yml` local.
- **`permission_denied: write_package` al publicar en ghcr.io**: fueron dos causas
  encadenadas, no una. Primero, *Settings → Actions → General → Workflow permissions* del
  repo estaba en **Read** — ese setting es un **techo**: aunque el job declare
  `permissions: packages: write`, GitHub lo capa a sólo lectura si el default del repo es
  read-only. Lo cambié a *Read and write* vía `gh api`. Eso no alcanzó: los paquetes
  `ingsoft3-backend`/`ingsoft3-frontend` ya existían desde el TP2, publicados a mano con mi
  propio token — y un paquete así **no** le da automáticamente permiso de escritura al
  `GITHUB_TOKEN` de Actions de mi repo (a diferencia de un paquete que un workflow crea de
  cero, que queda enlazado solo). Tuve que ir a la *Package settings* de cada uno → **Manage
  Actions access** → agregar el repo con rol **Write** (lo había agregado primero con
  **Read**, que da el mismo error).

## 10. Declaración de uso de IA

_(pendiente — lo escribo yo a mano)_
