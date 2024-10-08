FROM golang:1.19.1 as builder

LABEL org.opencontainers.image.description="Dockerized Pyrin Stratum Bridge"
LABEL org.opencontainers.image.authors="waglayla,waglayla"
LABEL org.opencontainers.image.source="https://github.com/Waglayla/waglayla-stratum-bridge"

WORKDIR /go/src/app
ADD go.mod .
ADD go.sum .
RUN go mod download

ADD . .
RUN go build -o /go/bin/app ./cmd/waglaylabridge


FROM gcr.io/distroless/base:nonroot
COPY --from=builder /go/bin/app /
COPY cmd/waglaylabridge/config.yaml /

WORKDIR /
ENTRYPOINT ["/app"]
