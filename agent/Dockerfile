FROM golang:1.22-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o agent ./cmd/main.go

FROM alpine:3.20

RUN apk --no-cache add ca-certificates tzdata wget mysql-client dumb-init

WORKDIR /app

COPY --from=builder /app/agent .

ENV TZ=Asia/Shanghai
ENV PLATFORM_URL=http://localhost:8080
ENV AGENT_ID=default-agent
ENV LOG_LEVEL=info

EXPOSE 9090

HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD wget -q --spider http://localhost:9090/health || exit 1

ENTRYPOINT ["dumb-init", "--"]
CMD ["./agent"]
