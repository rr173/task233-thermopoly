# 单阶段构建：Go 1.26.3 + CGO 禁用 + 纯 Go SQLite（modernc）
FROM golang:1.26.3-bookworm AS build
ENV CGO_ENABLED=0 \
    GOTOOLCHAIN=local \
    GOPROXY=https://goproxy.cn,direct \
    GOSUMDB=sum.golang.google.cn
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /bin/thermopoly ./cmd/thermopoly

FROM debian:bookworm-slim
WORKDIR /app
COPY --from=build /bin/thermopoly /bin/thermopoly
ENV GIN_MODE=release
EXPOSE 8080
ENTRYPOINT ["/bin/thermopoly"]
CMD ["--smoke-test"]
