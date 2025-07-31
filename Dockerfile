FROM golang:1.24-alpine

RUN apk add --no-cache git ca-certificates
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY main.go ./
RUN go build -o stackit-maintenance-exporter main.go

ENTRYPOINT ["/app/stackit-maintenance-exporter"]