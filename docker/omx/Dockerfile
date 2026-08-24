# syntax=docker/dockerfile:1

ARG OM_IMAGE=opensvc/om:dev
ARG OX_IMAGE=opensvc/ox:dev

FROM ${OM_IMAGE} AS om_builder
FROM ${OX_IMAGE} AS ox_builder

FROM alpine:3.24.1@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b

COPY --from=om_builder /usr/bin/om /usr/bin/om
COPY --from=ox_builder /usr/bin/ox /usr/bin/ox

ENTRYPOINT ["/usr/bin/om"]
CMD ["daemon", "run"]
