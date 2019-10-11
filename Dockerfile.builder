FROM scratch

COPY ./webdf-builder /bin/webdf-builder

CMD ["/bin/webdf-builder"]
