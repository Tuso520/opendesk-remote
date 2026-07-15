FROM golang:1.22-bookworm AS builder

WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/opendesk-builder ./cmd/opendesk-builder

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /out/opendesk-builder /opendesk-builder
ENTRYPOINT ["/opendesk-builder"]

