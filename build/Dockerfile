# syntax=docker/dockerfile:1.6

########################
# Build stage
########################
FROM golang:1.24-alpine AS build

RUN apk add --no-cache ca-certificates tzdata git

WORKDIR /src

# Cache deps
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build target selector: front | mteam | mtask
ARG SERVICE=front

# Build the binary
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/app ./cmd/${SERVICE}

########################
# Runtime stage
########################
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy binary
COPY --from=build /out/app /app/app

# If front serves templates/static, copy them (safe even if other services don’t need them)
# Adjust paths to your real template/static directories.
COPY internal/front/web/templates /app/templates
COPY internal/front/web/static /app/static

COPY internal/mtask/db/init.sql /app/mtask.sql
COPY internal/mteam/db/init.sql /app/mteam.sql
# If you have migrations/sql files needed at runtime, copy them too:
# COPY migrations /app/migrations

EXPOSE 8080

ENTRYPOINT ["/app/app"]
