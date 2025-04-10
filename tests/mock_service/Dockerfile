FROM golang:1.20-alpine AS builder

WORKDIR /app

COPY . .

RUN go mod download
RUN go build -o /app/mock-service ./mock_service.go

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/mock-service .

EXPOSE 8080

CMD ["./mock-service"] 