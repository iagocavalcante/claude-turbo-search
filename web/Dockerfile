FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
RUN adduser -D -u 10001 turbo
USER turbo
COPY --from=build /out/server /usr/local/bin/server
ENV PORT=8080 DATA_DIR=/data
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/server"]
