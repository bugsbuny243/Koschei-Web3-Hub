FROM golang:1.23-alpine AS go-builder
WORKDIR /app
COPY services/auth-api/go.mod ./
RUN go mod download
COPY services/auth-api .
RUN CGO_ENABLED=0 GOOS=linux go build -o koschei-api .

FROM alpine:3.19
WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata
COPY --from=go-builder /app/koschei-api .
COPY public ./public
ENV STATIC_DIR=/app/public
EXPOSE 8080
CMD ["./koschei-api"]
