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

FROM python:3.12-alpine AS runner
WORKDIR /app

RUN apk add --no-cache bash gzip tar openjdk17-jre
COPY koschei/workers/requirements.txt /app/requirements.txt
RUN pip install --no-cache-dir -r /app/requirements.txt

COPY --from=go-builder /app/koschei-api /app/koschei-api
COPY --from=go-builder /tmp/migrations /app/migrations
COPY public /app/public
COPY koschei/workers/worker.py /app/worker.py
COPY start.sh /app/start.sh

ENV PORT=8080
ENV STATIC_DIR=/app/public
ENV ANDROID_HOME=/opt/android-cache/sdk
ENV ANDROID_SDK_ROOT=/opt/android-cache/sdk
ENV ANDROID_NDK_HOME=/opt/android-cache/ndk
ENV ENGINE_PROJECT_PATH=/opt/android-cache/koschei/KoscheiGame.project

EXPOSE 8080

CMD ["/app/start.sh"]
