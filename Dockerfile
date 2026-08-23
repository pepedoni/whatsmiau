FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install gcc and SQLite dev libraries
RUN apk add build-base sqlite-dev gcc musl-dev

RUN adduser -D -u 1001 appuser

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN chown -R appuser:appuser /app

# Enable CGO
RUN CGO_ENABLED=1 GOOS=linux go build -buildvcs=false -a -installsuffix cgo -o whatsmiau main.go

FROM alpine:latest

RUN apk update && apk add --no-cache ffmpeg mailcap

RUN adduser -D -u 1001 appuser

WORKDIR /app

COPY --from=builder /app/whatsmiau /app/whatsmiau

RUN mkdir -p /app/data && chown -R appuser:appuser /app

EXPOSE 8081

USER appuser

ENTRYPOINT ["./whatsmiau"]
