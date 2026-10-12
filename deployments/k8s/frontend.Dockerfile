# Vue SPA. Build context: web/. URLs are relative, so the image works behind any public hostname.
FROM node:22-alpine AS build
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
ENV VITE_API_BASE_URL="" \
    VITE_KEYCLOAK_URL=/kc \
    VITE_KEYCLOAK_REALM=mytube \
    VITE_KEYCLOAK_CLIENT_ID=mytube-frontend
RUN npm run build

FROM nginx:1.27-alpine
COPY --from=build /app/dist /usr/share/nginx/html
RUN printf 'server {\n  listen 80;\n  root /usr/share/nginx/html;\n  location / { try_files $uri $uri/ /index.html; }\n}\n' > /etc/nginx/conf.d/default.conf
