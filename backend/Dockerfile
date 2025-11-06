FROM golang:1.25 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app/main .

FROM alpine:latest

WORKDIR /app

RUN apk add --no-cache tzdata curl

COPY --from=builder /app/main .
RUN chmod +x ./main

CMD ["./main"]