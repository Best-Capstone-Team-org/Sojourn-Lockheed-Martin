# frontend
FROM node:24-alpine AS frontend
WORKDIR /app/frontend
COPY frontend/ ./
RUN npm ci && npm run build

# backend
FROM golang:1.27-alpine AS backend
WORKDIR /app/backend
COPY backend/src/ ./
RUN CGO_ENABLED=0 go build -o /out/sojourn .

# scenarios
FROM debian:trixie-slim AS scenarios
COPY scripts/copy-scenarios.sh /usr/local/bin/
COPY deps/sojourn/scenarios/ /src/
RUN copy-scenarios.sh /src /scenarios

# runtime
FROM alpine:3.22
RUN apk add --no-cache qemu-system-arm
WORKDIR /app
COPY --from=backend /out/sojourn /usr/local/bin/sojourn
COPY --from=frontend /app/frontend/build ./frontend
COPY --from=scenarios /scenarios ./scenarios

# Player state lives only on this volume (charter R14.1).
ENV SAVE_DIR=/savedata
RUN mkdir -p /savedata
VOLUME /savedata

EXPOSE 8080
CMD ["sojourn"]
