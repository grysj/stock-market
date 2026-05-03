FROM golang:1.23-alpine AS build

WORKDIR /src
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN go build -o /out/api ./cmd/api

FROM alpine:3.20

RUN adduser -D -H appuser
USER appuser
WORKDIR /app
COPY --from=build /out/api /app/api
EXPOSE 8080

ENTRYPOINT ["/app/api"]
