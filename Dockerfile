FROM golang:1.24 AS builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOARCH=arm64 go build -o cmpserve main.go

FROM alpine:latest
RUN apk add --no-cache sqlite
WORKDIR /www
VOLUME ["/www"]
VOLUME ["/cache"]
COPY --from=builder /app/cmpserve /usr/local/bin/cmpserve
RUN chmod +x /usr/local/bin/cmpserve
EXPOSE 8080
CMD ["/usr/local/bin/cmpserve", "-dir=/www", "-cache-dir=/cache", "-addr=0.0.0.0", "-port=8080"]