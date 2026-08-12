# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM golang:1.24-alpine3.22@sha256:3641e0d9b931dc4f2f185dcd669c4679670e9277c8166a838ddb98a2d4389cb5 AS build

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev

WORKDIR /src

RUN apk add --no-cache ca-certificates

COPY go.mod ./
RUN go mod download

COPY cmd ./cmd

RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath \
    -ldflags "-s -w -X main.version=${VERSION}" \
    -o /out/outagedeck ./cmd/outagedeck

FROM scratch

LABEL org.opencontainers.image.title="OutageDeck CLI" \
      org.opencontainers.image.description="Check live cloud and SaaS provider status from a terminal or CI job" \
      org.opencontainers.image.source="https://github.com/outagedeck/cli" \
      org.opencontainers.image.licenses="MIT"

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/outagedeck /usr/local/bin/outagedeck

USER 65532:65532
ENTRYPOINT ["/usr/local/bin/outagedeck"]
CMD ["--help"]
