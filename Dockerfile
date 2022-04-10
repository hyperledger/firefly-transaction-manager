FROM golang:1.16-buster AS builder
ARG BUILD_VERSION
ENV BUILD_VERSION=${BUILD_VERSION}
ADD . /fftm
WORKDIR /fftm
RUN make

FROM debian:buster-slim
WORKDIR /fftm
COPY --from=builder /fftm/firefly-transaction-manager /usr/bin/fftm

ENTRYPOINT [ "/usr/bin/fftm" ]
