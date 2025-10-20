FROM node:18-alpine AS builder

WORKDIR /app

COPY webui/package*.json ./

RUN npm ci

COPY webui/ .

RUN npm run build

FROM nginx:alpine

COPY --from=builder /app/build /usr/share/nginx/html

COPY config/webui/nginx.conf /etc/nginx/nginx.conf

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]