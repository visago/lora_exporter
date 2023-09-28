############################
# STEP 1 build executable binary
############################
FROM golang:1.20.0 AS builder
RUN mkdir /work
WORKDIR /work
COPY ./ ./
RUN make build
############################
# STEP 2 build a small image
############################
#FROM gcr.io/distroless/static-debian11
FROM scratch
EXPOSE 5672
COPY --from=builder /work/lora_exporter /usr/bin/lora_exporter
ENTRYPOINT ["/usr/bin/lora_exporter"]
