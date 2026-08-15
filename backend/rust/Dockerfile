FROM rust:1.94-slim AS build
WORKDIR /app
COPY Cargo.toml ./
COPY src ./src
RUN cargo build --release

FROM debian:bookworm-slim
WORKDIR /app
COPY --from=build /app/target/release/neon-referee /usr/local/bin/neon-referee
EXPOSE 8081
ENV PORT=8081
CMD ["neon-referee"]
