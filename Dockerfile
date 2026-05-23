FROM node:20-alpine AS frontend-builder
WORKDIR /app

COPY koschei/frontend/package*.json ./
RUN npm install

COPY koschei/frontend ./
RUN npm run build

FROM golang:1.23-alpine AS go-builder
WORKDIR /src/koschei/api

COPY koschei/api/go.mod koschei/api/go.sum ./
RUN go mod download

COPY koschei/api ./
RUN go build -o /app/koschei-api .

FROM alpine:3.20 AS runner
WORKDIR /app

COPY --from=go-builder /app/koschei-api /app/koschei-api
COPY --from=go-builder /src/koschei/api/migrations /app/migrations
COPY --from=frontend-builder /app/dist /app/public

ENV PORT=8080
ENV STATIC_DIR=/app/public

EXPOSE 8080

CMD ["/app/koschei-api"]
