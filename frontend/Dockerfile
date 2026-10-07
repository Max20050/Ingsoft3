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

# ---- Etapa 4: runner sirve la app standalone (ya la tenías, queda ÚLTIMA) ----
FROM node:20-alpine AS runner
LABEL org.opencontainers.image.source https://github.com/Max20050/ingsoft3
WORKDIR /app
ENV NODE_ENV=production
COPY --from=builder /app/.next/standalone ./
COPY --from=builder /app/.next/static ./.next/static
COPY --from=builder /app/public ./public
EXPOSE 3000
CMD ["node", "server.js"]
