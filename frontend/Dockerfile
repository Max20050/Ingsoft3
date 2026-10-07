FROM node:20-alpine AS deps
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci

FROM node:20-alpine AS builder
WORKDIR /app
ARG NEXT_PUBLIC_API_URL=""
ENV NEXT_PUBLIC_API_URL=$NEXT_PUBLIC_API_URL
COPY --from=deps /app/node_modules ./node_modules
COPY . .
RUN npm run build

# ---- Etapa 3: tests (parte del build: ya tiene Node y las dependencias) ----
FROM builder AS test
ENTRYPOINT ["npm", "run", "test:ci"]

# ---- Etapa 4: runner sirve la app standalone + nginx como reverse proxy ----
FROM node:20-alpine AS runner
LABEL org.opencontainers.image.source https://github.com/Max20050/ingsoft3
RUN apk add --no-cache nginx
WORKDIR /app
ENV NODE_ENV=production
# Docker setea HOSTNAME al id del contenedor; sin esto el server de Next
# bindea solo esa IP y nginx no puede alcanzarlo por 127.0.0.1.
ENV HOSTNAME=0.0.0.0
COPY --from=builder /app/.next/standalone ./
COPY --from=builder /app/.next/static ./.next/static
COPY --from=builder /app/public ./public
COPY nginx/default.conf /etc/nginx/http.d/default.conf
COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh
EXPOSE 80
CMD ["/docker-entrypoint.sh"]
