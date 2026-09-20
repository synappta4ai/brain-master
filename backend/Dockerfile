# Dockerfile para Backend en Go (Brain-Master Gateway)

# Etapa 1: Compilación de la aplicación
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Instalar dependencias del sistema necesarias para compilar
RUN apk add --no-cache git

# Cachear módulos de Go
COPY go.mod go.sum* ./
RUN go mod download

# Copiar el código fuente y compilar binario estático optimizado
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/brain-master-server .

# Etapa 2: Contenedor final ligero de ejecución
FROM alpine:3.19

WORKDIR /app

# Instalar FFmpeg para transcodificación y certificados CA
RUN apk add --no-cache ffmpeg ca-certificates tzdata

# Copiar el binario desde la etapa de compilación
COPY --from=builder /app/brain-master-server /app/brain-master-server

# Crear carpeta para almacenamiento de salidas multimedia
RUN mkdir -p /app/outputs

EXPOSE 8080

CMD ["/app/brain-master-server"]
