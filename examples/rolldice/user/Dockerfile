FROM golang:1.24.4@sha256:1bb140b73b0c33df854496d07cb8d05ac62b358923ab513c18ede977f880c8b6
WORKDIR /usr/src/user
# Copy go.mod for better layer caching
COPY ./go.mod ./go.sum ./
RUN --mount=type=cache,target=/go/pkg go mod download && go mod verify
COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
    go build -v -o /usr/local/bin/user ./server/...
CMD ["user"]
