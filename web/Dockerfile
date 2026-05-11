# --- Dev stage: HMR with vite dev server ---
# Build context: ./web (from docker-compose.override.yml)
FROM node:24-alpine AS dev
RUN corepack enable
WORKDIR /app
COPY package.json pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile --ignore-scripts
EXPOSE 3000
CMD ["pnpm", "dev", "--host", "--port", "3000"]

# --- Production build ---
# Build context: . (repo root, from docker-compose.yml)
# Paraglide needs api/internal/i18n/messages/*.json at build time.
FROM node:24-alpine AS build
RUN corepack enable
WORKDIR /app
COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile --ignore-scripts
COPY web/ .
COPY api/internal/i18n/messages/ ../api/internal/i18n/messages/
RUN pnpm build

FROM node:24-alpine AS production
RUN corepack enable
WORKDIR /app
COPY --from=build /app/build ./build
COPY --from=build /app/package.json /app/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile --prod --ignore-scripts
EXPOSE 3000
ENV NODE_ENV=production
CMD ["node", "build"]
