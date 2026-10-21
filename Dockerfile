FROM golang:1.27-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/shlink-go ./cmd/shlink-go

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S shlink \
    && adduser -S -G shlink shlink

WORKDIR /app
COPY --from=build /out/shlink-go /usr/local/bin/shlink-go

USER shlink
EXPOSE 8080
ENTRYPOINT ["shlink-go"]
CMD ["serve"]
