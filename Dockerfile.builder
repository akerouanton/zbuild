FROM debian:buster-slim

RUN apt-get update && \
    apt-get install -y ca-certificates && \
    rm -rf /var/lib/apt/lists/*

COPY ./zbuilder /bin/zbuild

ENTRYPOINT ["/bin/zbuild"]
