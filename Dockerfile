# syntax=docker/dockerfile:1.7

ARG NODE_VERSION=22
ARG GO_VERSION=1.26
ARG ALPINE_VERSION=3.23
ARG NGINX_VERSION=1.28-alpine

FROM node:${NODE_VERSION}-alpine AS web-builder
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:${GO_VERSION}-alpine AS api-builder
ARG GOPROXY=https://goproxy.cn,direct
ARG GOSUMDB=sum.golang.google.cn
# GIT_COMMIT is stamped into the binary so /api/v1/healthz can report which
# build is actually serving traffic. The deploy script relies on it.
ARG GIT_COMMIT=dev
ARG BUILD_TIME=""
ENV GOPROXY=${GOPROXY} \
    GOSUMDB=${GOSUMDB}
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY 8v/ ./8v/
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w \
      -X vhome/internal/version.Commit=${GIT_COMMIT} \
      -X vhome/internal/version.BuildTime=${BUILD_TIME}" \
    -o /out/vhome \
    ./cmd

FROM alpine:${ALPINE_VERSION} AS api
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S vhome \
    && adduser -S -G vhome vhome \
    && mkdir -p /app/data/uploads \
    && chown -R vhome:vhome /app
WORKDIR /app
COPY --from=api-builder /out/vhome /app/vhome
USER vhome
EXPOSE 8080
ENTRYPOINT ["/app/vhome"]

FROM nginx:${NGINX_VERSION} AS web
COPY deploy/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=web-builder /src/web/dist/ /usr/share/nginx/html/
EXPOSE 5173
