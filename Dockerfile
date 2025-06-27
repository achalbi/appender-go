ARG GO_VERSION=1.21
ARG ALPINE_VERSION=3.18

# Builder stage
FROM golang:${GO_VERSION}-alpine${ALPINE_VERSION} AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /appender-go

# Final stage
FROM alpine:${ALPINE_VERSION}

RUN apk --no-cache add ca-certificates

WORKDIR /

COPY --from=builder /appender-go /appender-go

CMD ["/appender-go"]