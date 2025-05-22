# This is a renovate-friendly source of Docker images.
FROM avtodev/markdown-lint:v1@sha256:6aeedc2f49138ce7a1cd0adffc1b1c0321b841dc2102408967d9301c031949ee AS markdown
FROM python:3.13.3-slim-bullseye@sha256:45338d24f0fd55d4a7cb0cd3100a12a4350c7015cd1ec983b6f76d3d490a12c8 AS python