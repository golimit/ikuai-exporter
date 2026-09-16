# 官方镜像版本见 https://hub.docker.com/_/golang/tags 与 https://hub.docker.com/_/alpine/tags
FROM golang:1.27.1-alpine3.24 AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /ikuai-exporter .

FROM alpine:3.24 AS local
RUN apk add --no-cache ca-certificates tzdata \
    && ln -snf /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
    && echo Asia/Shanghai > /etc/timezone
ENV TZ=Asia/Shanghai
LABEL org.opencontainers.image.title="ikuai-exporter"
COPY --from=builder /ikuai-exporter /app
EXPOSE 9401
WORKDIR /data
ENTRYPOINT ["/app"]
CMD ["server"]

FROM alpine:3.24 AS release
RUN apk add --no-cache ca-certificates tzdata \
    && ln -snf /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
    && echo Asia/Shanghai > /etc/timezone
ENV TZ=Asia/Shanghai
LABEL org.opencontainers.image.title="ikuai-exporter"
ARG TARGETPLATFORM
COPY ${TARGETPLATFORM}/ikuai-exporter /app
EXPOSE 9401
WORKDIR /data
ENTRYPOINT ["/app"]
CMD ["server"]
