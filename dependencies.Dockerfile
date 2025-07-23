# This is a renovate-friendly source of Docker images.
FROM avtodev/markdown-lint:v1@sha256:6aeedc2f49138ce7a1cd0adffc1b1c0321b841dc2102408967d9301c031949ee AS markdown
FROM python:3.13.5-slim-bullseye@sha256:846d391e752027be4a490ac42f8bba40b54b8271fd57d18d5304698a02a4efb0 AS python