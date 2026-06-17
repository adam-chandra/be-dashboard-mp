# syntax=docker/dockerfile:1.6

# ---------- builder ----------
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Cache dependency layer
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build static binary
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64
RUN go build -trimpath -ldflags="-s -w" -o /out/server .

# ---------- runtime ----------
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app
COPY --from=builder /out/server /app/server

# Render/Fly inject PORT env saat runtime; default 8084 untuk lokal.
ENV PORT=8084
EXPOSE 8084

USER nonroot:nonroot
ENTRYPOINT ["/app/server"]
