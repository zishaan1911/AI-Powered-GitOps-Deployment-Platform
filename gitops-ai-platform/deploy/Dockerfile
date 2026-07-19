# Builds all gitops-ai-platform CLIs into one small image. The CronJob
# manifest invokes /app/health-watcher; the same image also contains
# /app/pipeline, /app/risk-reviewer, etc. for use in CI or other jobs.
FROM golang:1.22 AS build
WORKDIR /src
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/pipeline ./cmd/pipeline && \
    CGO_ENABLED=0 GOOS=linux go build -o /out/risk-reviewer ./cmd/risk-reviewer && \
    CGO_ENABLED=0 GOOS=linux go build -o /out/health-watcher ./cmd/health-watcher && \
    CGO_ENABLED=0 GOOS=linux go build -o /out/detector ./cmd/detector && \
    CGO_ENABLED=0 GOOS=linux go build -o /out/containerizer ./cmd/containerizer && \
    CGO_ENABLED=0 GOOS=linux go build -o /out/manifest-generator ./cmd/manifest-generator && \
    CGO_ENABLED=0 GOOS=linux go build -o /out/gitops-writer ./cmd/gitops-writer

FROM alpine:3.20
RUN apk add --no-cache git curl ca-certificates && \
    KUBECTL_VERSION="$(curl -Ls https://dl.k8s.io/release/stable.txt)" && \
    curl -Lo /usr/local/bin/kubectl "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl" && \
    chmod +x /usr/local/bin/kubectl && \
    apk del curl
WORKDIR /app
COPY --from=build /out/ /app/
USER 10001
ENTRYPOINT ["/app/pipeline"]
