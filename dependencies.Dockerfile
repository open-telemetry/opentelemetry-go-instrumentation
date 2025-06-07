# This is a renovate-friendly source of Docker images.
FROM avtodev/markdown-lint:v1@sha256:6aeedc2f49138ce7a1cd0adffc1b1c0321b841dc2102408967d9301c031949ee AS markdown
FROM python:3.13.4-slim-bullseye@sha256:de0e7494b82e073b7586bfb2b081428ca2c8d1db2f6a6c94bc2e4c9bd4e71276 AS python