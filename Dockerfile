# ── Stage 1: Build Go binary ──────────────────────────────────────────────────
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/server ./cmd/server/

# ── Stage 2: Build frontend ──────────────────────────────────────────────────
FROM node:22-alpine AS frontend

WORKDIR /app/frontend

COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci

COPY frontend/ .
RUN npm run build

# ── Stage 3: Production image ────────────────────────────────────────────────
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

RUN addgroup -S gatie && adduser -S gatie -G gatie

WORKDIR /app

COPY --from=builder /app/server .
COPY --from=frontend /app/frontend/dist ./frontend/dist
COPY migrations/ ./migrations/

USER gatie

EXPOSE 8080

ENTRYPOINT ["./server"]
