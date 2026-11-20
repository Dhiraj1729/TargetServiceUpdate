# Build the static Linux binary with `make image` before building this image.
FROM scratch
COPY bin/manager-linux /manager
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/manager"]
