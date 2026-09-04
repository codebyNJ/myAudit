# Multi-stage build for the myIntern control plane (UI embedded into the Go binary).
FROM node:22-alpine AS web
WORKDIR /web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# stage the freshly built UI where //go:embed expects it
COPY --from=web /web/dist ./internal/api/web/dist
RUN CGO_ENABLED=0 go build -o /bin/serve ./cmd/serve

FROM alpine:3.20
RUN adduser -D -u 10001 app
USER app
COPY --from=build /bin/serve /bin/serve
EXPOSE 7788
ENV PORT=7788
ENTRYPOINT ["/bin/serve"]
