FROM golang:1.24 AS builder

WORKDIR /app

COPY go.mod go.sum main.go ./
RUN go mod download

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o stackit-maintenance-exporter main.go

FROM alpine:latest

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=builder /app/stackit-maintenance-exporter .

ENTRYPOINT ["/app/stackit-maintenance-exporter"]