# --- build stage ---
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# static binary of the always-on HTTP server (not the lambda entrypoint)
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /app ./cmd/server

# --- run stage ---
FROM alpine:3.20
# ca-certificates: needed for outbound HTTPS (DynamoDB, Google OAuth, Web Push)
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 app
COPY --from=build /app /app
USER app
ENV PORT=8080
EXPOSE 8080
ENTRYPOINT ["/app"]
