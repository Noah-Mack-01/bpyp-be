FROM golang:1.24-alpine AS build

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . . 
RUN CGO_ENABLED=0 GOOS=linux go build -o server .

FROM alpine:latest
RUN apk --no-cache add ca-certificates

WORKDIR /app
COPY --from=build /app/server .
COPY --from=build /app/launcher.sh .
RUN chmod +x launcher.sh

# Default configuration
ENV PORT=3000
ENV INSTANCE_COUNT=3
ENV BPYP_WORKER_MULTIPLIER=2

# Expose a range of ports for multiple instances
EXPOSE 3000-3010

CMD ["./launcher.sh"]