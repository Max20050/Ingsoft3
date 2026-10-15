#!/bin/sh
set -e

# Render inyecta su propio PORT (el puerto al que manda el tráfico externo) y
# espera que el proceso que escucha el puerto expuesto sea el que lo lee. Acá
# ese proceso es nginx, no Next.js: si dejáramos que el server.js de Next
# tomara este mismo PORT (lo hace por defecto), Next terminaría escuchando
# donde Render manda el tráfico y nginx —con su proxy a /api/— quedaría al
# margen, sirviendo en un puerto que nadie usa. Por eso Next se fija a 3000
# siempre, y nginx es el único que escucha en $PORT (80 en compose, donde no
# se define la variable).
: "${PORT:=80}"

# BACKEND_URL y PORT: sin la lista, envsubst también pisaría las variables
# propias de nginx del template ($host, $remote_addr, etc.) con vacío.
envsubst '${BACKEND_URL} ${PORT}' \
  < /etc/nginx/templates/default.conf.template \
  > /etc/nginx/http.d/default.conf

PORT=3000 node server.js &
NODE_PID=$!

nginx -g 'daemon off;' &
NGINX_PID=$!

trap 'kill "$NODE_PID" "$NGINX_PID" 2>/dev/null' TERM INT

# Si cualquiera de los dos procesos muere, tiramos el contenedor entero
# para que Docker lo reinicie, en vez de quedar sirviendo a medias.
while kill -0 "$NODE_PID" 2>/dev/null && kill -0 "$NGINX_PID" 2>/dev/null; do
  sleep 1
done

kill "$NODE_PID" "$NGINX_PID" 2>/dev/null
exit 1
