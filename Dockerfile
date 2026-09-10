FROM golang:1.26.2-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /ikuai-exporter .

FROM jakes/base-image:alpine-3.14.0-tz AS local
LABEL maintainers="Jakes Lee"
LABEL description="iKuai exporter"
COPY --from=builder /ikuai-exporter /app
EXPOSE 9090
WORKDIR /data
CMD ["/app", "server"]

FROM jakes/base-image:alpine-3.14.0-tz AS release
LABEL maintainers="Jakes Lee"
LABEL description="iKuai exporter"
ARG TARGETPLATFORM
COPY ${TARGETPLATFORM}/ikuai-exporter /app
EXPOSE 9090
WORKDIR /data
CMD ["/app", "server"]
