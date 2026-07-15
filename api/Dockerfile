FROM golang:1.25.12-bookworm AS builder

WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/opendesk-api ./cmd/opendesk-api

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app
COPY --from=builder /out/opendesk-api /app/opendesk-api
EXPOSE 21114
ENTRYPOINT ["/app/opendesk-api"]
