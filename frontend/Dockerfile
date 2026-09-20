# Dockerfile para Frontend en Angular 21 (Brain-Master UI)

# Etapa 1: Compilación de la aplicación Angular con Node.js
FROM node:20-alpine AS build

WORKDIR /app

# Cachear dependencias de package.json
COPY package*.json ./
RUN npm ci || npm install

# Copiar código fuente y compilar bundle de producción
COPY . .
RUN npm run build -- --configuration production

# Etapa 2: Servir con servidor web ligero Nginx
FROM nginx:1.25-alpine

# Copiar configuración personalizada de Nginx para SPA y WebSockets
COPY nginx.conf /etc/nginx/conf.d/default.conf

# Copiar los assets compilados desde la etapa de build
COPY --from=build /app/dist/brain-master-frontend/browser /usr/share/nginx/html

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
