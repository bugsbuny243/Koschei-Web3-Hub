FROM node:20-alpine AS builder
WORKDIR /app/koschei/frontend
COPY koschei/frontend/package*.json ./
RUN npm ci
COPY koschei/frontend ./
RUN npm run build

FROM node:20-alpine AS runner
WORKDIR /app/koschei/frontend
ENV NODE_ENV=production
ENV PORT=8080
RUN npm i -g vite
COPY --from=builder /app/koschei/frontend/dist ./dist
EXPOSE 8080
CMD ["vite", "preview", "--host", "0.0.0.0", "--port", "8080"]
