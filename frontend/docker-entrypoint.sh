#!/bin/sh
set -e

node server.js &
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
