FROM golang:1.21
COPY . /offsets-tracker
WORKDIR /offsets-tracker
RUN go build -o offsets-tracker
ENTRYPOINT ["/offsets-tracker/offsets-tracker"]