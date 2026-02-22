FROM golang:1.26-alpine AS builder

WORKDIR /app
COPY go.mod ./
RUN go mod download
RUN mkdir -p /app/data
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o server ./cmd/server

FROM scratch

WORKDIR /root
COPY --from=builder /app/server .
COPY --from=builder /app/data /root/data

EXPOSE 8080
CMD ["./server"]