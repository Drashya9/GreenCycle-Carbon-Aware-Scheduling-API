FROM golang:1.26-alpine AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/greencycle ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates \
    && addgroup -S greencycle && adduser -S greencycle -G greencycle

COPY --from=builder /out/greencycle /usr/local/bin/greencycle

USER greencycle
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/greencycle"]
