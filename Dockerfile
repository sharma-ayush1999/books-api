FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o server .

FROM alpine:3.19

RUN adduser -D -u 1001 appuser
USER appuser

WORKDIR /app
COPY --from=builder /app/server .

EXPOSE 8080
CMD ["./server"]