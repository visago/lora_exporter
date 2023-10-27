############################
# STEP 1 build executable binary
############################
FROM golang:1.20.0 AS builder
RUN mkdir /work
WORKDIR /work
COPY ./ ./
RUN make build
############################
# STEP 2 get certs
############################
FROM alpine:latest as certs
RUN apk --update add ca-certificates
############################
# STEP 3 build a small image
############################
FROM scratch
EXPOSE 5672
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /work/lora_exporter /usr/bin/lora_exporter
ENTRYPOINT ["/usr/bin/lora_exporter"]
