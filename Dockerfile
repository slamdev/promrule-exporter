FROM gcr.io/distroless/static:nonroot
WORKDIR /
ADD promrule-exporter promrule-exporter
USER 65532:65532

ENTRYPOINT ["/promrule-exporter"]
