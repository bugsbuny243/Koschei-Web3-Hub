FROM node:20-alpine AS builder
WORKDIR /app

COPY koschei/frontend/package*.json ./
RUN npm ci

COPY koschei/frontend ./
RUN npm run build

FROM nginx:1.27-alpine AS runner

COPY --from=builder /app/dist /usr/share/nginx/html

RUN cat > /etc/nginx/conf.d/default.conf <<'NGINX_EOF'
server {
  listen 8080;
  server_name _;

  root /usr/share/nginx/html;
  index index.html;

  location / {
    try_files $uri $uri/ /index.html;
  }

  location = /health {
    add_header Content-Type text/plain;
    return 200 'ok';
  }
}
NGINX_EOF

EXPOSE 8080

CMD ["nginx", "-g", "daemon off;"]
