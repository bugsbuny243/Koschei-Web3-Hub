FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN go build -o /koschei-api ./main.go

FROM alpine:3.20
RUN adduser -D appuser
USER appuser
COPY --from=builder /koschei-api /usr/local/bin/koschei-api
EXPOSE 8080
CMD ["koschei-api"]
