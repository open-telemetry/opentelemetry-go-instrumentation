FROM golang:1.23@sha256:08e1417a41687ce17a8cfbf65285446a794ac5023635b848faf1c2ed1d7e475f

WORKDIR /usr/src/user

COPY ./go.mod ./go.sum ./
RUN --mount=type=cache,target=/go/pkg go mod download && go mod verify

COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
    go build -v -o /usr/local/bin/user ./server/...

CMD ["user"]
