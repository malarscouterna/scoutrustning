# --- Dev stage: HMR with vite dev server ---
FROM node:24-alpine AS dev
RUN corepack enable
WORKDIR /app
COPY package.json pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
EXPOSE 3000
CMD ["pnpm", "dev", "--host", "--port", "3000"]

# --- Production build ---
FROM node:24-alpine AS build
RUN corepack enable
WORKDIR /app
COPY package.json pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY . .
RUN pnpm build

FROM node:24-alpine AS production
WORKDIR /app
COPY --from=build /app/build ./build
COPY --from=build /app/package.json .
EXPOSE 3000
ENV NODE_ENV=production
CMD ["node", "build"]
