FROM golang:1.23-alpine AS build
WORKDIR /app

RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /dolgo-bot ./cmd/bot

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /dolgo-bot /app/dolgo-bot
COPY migrations /app/migrations
ENTRYPOINT ["/app/dolgo-bot"]
