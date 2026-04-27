# This is a renovate-friendly source of Docker images.
FROM avtodev/markdown-lint:v1@sha256:6aeedc2f49138ce7a1cd0adffc1b1c0321b841dc2102408967d9301c031949ee AS markdown
FROM python:3.13.6-slim-bullseye@sha256:e98b521460ee75bca92175c16247bdf7275637a8faaeb2bcfa19d879ae5c4b9a AS python
FROM xianpengshen/clang-tools:19@sha256:08ceb9f83b59e431d5ac2d008ca7e5c876dd486c18387cb7af42e520a688e3ab AS clang-format