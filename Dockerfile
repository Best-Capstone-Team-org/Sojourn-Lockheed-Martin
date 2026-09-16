# frontend
FROM node:20-alpine AS frontend
WORKDIR /app/frontend
COPY frontend/ ./
RUN npm ci && npm run build

# backend
FROM golang:1.27-alpine AS backend
WORKDIR /app/backend
COPY backend/src/ ./
RUN CGO_ENABLED=0 go build -o /out/sojourn .

# firmware
FROM debian:trixie-slim AS firmware
RUN apt-get update && apt-get install -y --no-install-recommends \
        make python3 gcc-arm-none-eabi binutils-arm-none-eabi libnewlib-arm-none-eabi \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /firmware
COPY deps/sojourn/firmware/ ./
RUN make

# runtime
FROM alpine:3.22
RUN apk add --no-cache qemu-system-arm
WORKDIR /app
COPY --from=backend /out/sojourn /usr/local/bin/sojourn
COPY --from=frontend /app/frontend/build ./frontend
COPY --from=firmware /firmware/build/probe_rom.elf ./probe_rom.elf

EXPOSE 8080
CMD ["sojourn", "/app/probe_rom.elf"]
