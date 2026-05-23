FROM node:20-alpine AS frontend-builder
WORKDIR /app

COPY koschei/frontend/package*.json ./
RUN npm install

COPY koschei/frontend ./
RUN npm run build

FROM golang:1.23-alpine AS go-builder
WORKDIR /src

COPY koschei/api/go.mod koschei/api/go.sum /src/koschei/api/
WORKDIR /src/koschei/api
RUN go mod download

WORKDIR /src
COPY . /src
WORKDIR /src/koschei/api
RUN go build -o /app/koschei-api .
RUN mkdir -p /tmp/migrations \
    && if [ -d /src/koschei/api/migrations ]; then \
        cp -a /src/koschei/api/migrations/. /tmp/migrations/; \
    elif [ -d /src/migrations ]; then \
        cp -a /src/migrations/. /tmp/migrations/; \
    fi

FROM alpine:3.20 AS runner
WORKDIR /app

COPY --from=go-builder /app/koschei-api /app/koschei-api
COPY --from=go-builder /tmp/migrations /app/migrations
COPY --from=frontend-builder /app/dist /app/public

ENV PORT=8080
ENV STATIC_DIR=/app/public

EXPOSE 8080

CMD ["/app/koschei-api"]
