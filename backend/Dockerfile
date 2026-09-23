FROM golang:1.25-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/server ./cmd/server

FROM alpine:3.19
RUN adduser -D -u 10001 nocturne
COPY --from=builder /out/server /usr/local/bin/server
USER nocturne
EXPOSE 8080
ENTRYPOINT ["server"]
