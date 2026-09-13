FROM scratch

COPY forseti /forseti
COPY LICENSE.md /LICENSE.md

EXPOSE 9099

HEALTHCHECK --interval=30s --timeout=10s \
           --start-period=30s --retries=3 \
  CMD ["/forseti", "healthcheck", "--config", "/config/forseti.yml"]

ENTRYPOINT ["/forseti"]
