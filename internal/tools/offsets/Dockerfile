FROM golang:1.20
COPY . /offsets
WORKDIR /offsets
RUN go build -o offsets
ENTRYPOINT ["/offsets/offsets"]
