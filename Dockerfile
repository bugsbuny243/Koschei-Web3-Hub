FROM golang:1.23-alpine AS go-builder
WORKDIR /app
COPY koschei/api/go.mod koschei/api/go.sum ./
RUN go mod download
COPY koschei/api .
RUN CGO_ENABLED=0 GOOS=linux go build -o koschei-api .

FROM alpine:3.19
WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata
COPY --from=go-builder /app/koschei-api .
COPY koschei/api/public ./public
COPY koschei/api/migrations ./migrations
ENV STATIC_DIR=/app/public
EXPOSE 8080
CMD ["./koschei-api"]
