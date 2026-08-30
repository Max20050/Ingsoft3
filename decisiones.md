# Decisiones — TP1

## 1. Por qué Git no pudo resolver el conflicto solo — y qué habría tenido que pasar para que nunca apareciera.
Git no puede resolver el conflicto por su propia cuenta ya que no sabe cual es la intencion de la persona que pusheo.
Para que el problema nunca hubiera aparecido lo que se puede hacer es traer los cambios del main antes de hacer la pull request. Resolves conflictos antes de pushear.

## 2. Qué problemas encontraste y cómo los solucionaste.
El unico problema que tuve y se pueden ver en los commits es que me olvide de activar el ruleset entonces el primer commit si paso directo a produccion.
despues active el ruleset y ya funciono.

## 3. Declaración de uso de IA: qué partes hiciste con ayuda de inteligencia artificial y cómo verificaste lo que te devolvió (§ Uso de IA del enunciado).
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