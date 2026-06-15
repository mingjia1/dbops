FROM golang:1.22-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o platform-backend ./cmd/main.go

FROM alpine:3.20

RUN apk --no-cache add ca-certificates tzdata wget dumb-init

WORKDIR /app

COPY --from=builder /app/platform-backend .

ENV TZ=Asia/Shanghai
ENV DBOPS_DB_URL=""
ENV DBOPS_JWT_SECRET=""
ENV DBOPS_ENCRYPTION_KEY=""
ENV DBOPS_AGENT_TOKEN=""
ENV LOG_LEVEL=info

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD wget -q --spider http://localhost:8080/health || exit 1

ENTRYPOINT ["dumb-init", "--"]
CMD ["./platform-backend"]
