# ---- Build stage ----
FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /auth-service ./cmd/server

# ---- Runtime stage ----
FROM alpine:3.21

RUN apk --no-cache add ca-certificates

COPY --from=builder /auth-service /auth-service

EXPOSE 3000

CMD ["/auth-service"]
