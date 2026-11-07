FROM node:24.22.2-bookworm-slim AS web
WORKDIR /src/web
RUN npm install --global pnpm@12.11.2
COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ ./
# Public, non-secret build settings; see docs/deployment.md.
ARG VITE_SOURCE_URL=
ARG VITE_SITE_URL=
RUN pnpm build
FROM golang:1.27.3-bookworm AS backend
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/server ./cmd/server
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -o /converter ./cmd/server
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/* && useradd --uid 10001 --create-home converter && mkdir /data && chown converter /data
COPY --from=backend /converter /usr/local/bin/converter
COPY --from=web /src/web/dist /app/web/dist
USER converter
WORKDIR /app
ENV DATABASE_PATH=/data/converter.db ASSETS_PATH=/app/web/dist
EXPOSE 8080
ENTRYPOINT ["converter"]
