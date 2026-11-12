FROM node:22-bookworm-slim AS frontend
WORKDIR /src/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.25-bookworm AS backend
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
RUN CGO_ENABLED=0 go build -trimpath -o /studio ./cmd/studio

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --system --uid 10001 --create-home studio
WORKDIR /app
COPY --from=backend /studio /app/studio
COPY --from=frontend /src/web/dist /app/web/dist
USER studio
ENV STUDIO_ADDR=0.0.0.0:8080 STUDIO_WEB_DIR=/app/web/dist
EXPOSE 8080
ENTRYPOINT ["/app/studio"]
