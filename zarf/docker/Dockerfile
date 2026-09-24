# Aether container image. Built by goreleaser (dockers_v2 in .goreleaser.yaml),
# which places the prebuilt static binary at $TARGETPLATFORM/aether in the
# build context — this file does not compile anything.
FROM alpine:3.22

# Optional runtime tools, same as the .deb's Recommends:
#   ffmpeg      ffprobe, the fallback tag reader (internal/tags/ffprobe.go)
#   chromaprint fpcalc, AcoustID identification (libs/fpcalc)
# ca-certificates for the HTTPS metadata providers, tzdata for TZ=.
RUN apk add --no-cache ffmpeg chromaprint ca-certificates tzdata \
    && addgroup -S -g 1000 aether \
    && adduser -S -D -H -u 1000 -G aether -h /var/lib/aether aether \
    && mkdir -p /var/lib/aether /music \
    && chown aether:aether /var/lib/aether

ARG TARGETPLATFORM
COPY $TARGETPLATFORM/aether /usr/bin/aether
COPY zarf/docker/config.yaml /etc/aether/config.yaml

USER aether
WORKDIR /var/lib/aether
VOLUME /var/lib/aether
EXPOSE 8075

ENTRYPOINT ["/usr/bin/aether"]
CMD ["start", "--config", "/etc/aether/config.yaml"]
