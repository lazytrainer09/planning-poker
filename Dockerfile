# Backend: Go API server
FROM golang:1.26-alpine AS builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY internal/ ./internal/
COPY main.go ./
RUN CGO_ENABLED=1 go build -o planning-poker .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /app/planning-poker .
RUN mkdir -p /app/data

ENV PORT=8080
ENV DB_PATH=/app/data/planning_poker.db
EXPOSE 8080

CMD ["./planning-poker"]
