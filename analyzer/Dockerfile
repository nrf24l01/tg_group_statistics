FROM golang:1.25 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w"  -o /app/main .

FROM alpine:latest

WORKDIR /app

RUN apk add --no-cache tzdata curl

COPY --from=builder /app/main .
COPY --from=builder /app/start.sh .
RUN chmod +x ./main
RUN chmod +x ./start.sh

CMD ["./start.sh"]