FROM alpine:3.19.0

LABEL org.opencontainers.image.authors="ProjectDiscovery"
LABEL org.opencontainers.image.description="Fast and customizable subdomain wordlist generator using DSL"
LABEL org.opencontainers.image.licenses="MIT"
LABEL org.opencontainers.image.title="alterx"
LABEL org.opencontainers.image.url="https://github.com/projectdiscovery/alterx"

RUN apk -U upgrade --no-cache \
    && apk add --no-cache bind-tools ca-certificates

ARG TARGETPLATFORM
COPY $TARGETPLATFORM/alterx /usr/local/bin/

ENTRYPOINT ["alterx"]
