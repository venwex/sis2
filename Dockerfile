FROM golang:1.26 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -buildvcs=false -o /pastebin ./cmd/api

FROM alpine:3.22

WORKDIR /app

COPY --from=builder /pastebin /app/pastebin

EXPOSE 8080 9090

ENTRYPOINT ["/app/pastebin"]