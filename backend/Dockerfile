# 1. Rust referee 빌드 (게임 로직/심판)
FROM rust:1.94-slim AS referee-build
WORKDIR /app
COPY backend/rust/Cargo.toml backend/rust/Cargo.lock* ./
COPY backend/rust/src ./src
RUN cargo build --release

# 2. Go gateway 빌드 (인증/방 관리/WebSocket 게이트웨이)
FROM golang:1.24-alpine AS gateway-build
WORKDIR /app
COPY backend/go/go.mod backend/go/go.sum ./
RUN go mod download
COPY backend/go .
RUN CGO_ENABLED=0 go build -o /gateway ./cmd/gateway

# 3. 통합 실행 환경 (Debian Slim)
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*
WORKDIR /app

COPY --from=referee-build /app/target/release/neon-referee /usr/local/bin/neon-referee
COPY --from=gateway-build /gateway /usr/local/bin/gateway
COPY backend/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

EXPOSE 8000
ENV PORT=8000
ENV REFEREE_URL=http://127.0.0.1:8081

ENTRYPOINT ["/entrypoint.sh"]
