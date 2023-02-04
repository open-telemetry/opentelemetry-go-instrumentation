# offsets-tracker

This project tracks offsets of fields inside of Go structs across versions.

This tracking is needed in order to create a  [stable eBPF based instrumentation](https://github.com/keyval-dev/opentelemetry-go-instrumentation/blob/master/docs/how-it-works.md#instrumentation-stability).

Calculating offsets is accomplished by creating a binary file containing the relevant struct and analyzing its DWARF information.
## Tracking targets

The `main.go` file specifies all the tracking targets.
Each tracking target has a name which can be either a fully qualified go module name or `go` for tracking parameters in the standard library.

For example, in order to track the `URL` field inside the `net/http.Request` struct in the Go standard library we register the following target:
```go
target.New("go").
  FindOffsets([]*binary.DataMember{
    {
      StructName: "net/http.Request",
      Field:      "URL",
    },
  })
```

## Output

offsets-tracker writes all the tracked offsets into a file named `offset_results.json`.
For example, here is the tracking of `method` field inside `transport.Stream` struct in the `google.golang.org/grpc` module:
```go
{
  "name": "google.golang.org/grpc",
  "data_members": [
    {
      "struct": "google.golang.org/grpc/internal/transport.Stream",
      "field_name": "method",
      "offsets": [
        {
          "offset": 72,
          "version": "v1.0.2"
        },
        {
          "offset": 72,
          "version": "v1.0.3"
        },
        {
          "offset": 72,
          "version": "v1.0.4"
        },
        {
          "offset": 80,
          "version": "v1.3.0"
        },
        {
          "offset": 80,
          "version": "v1.4.0"
        },
```

## Versions Discovery

By default, offsets-tracker finds availble versions by executing `go list -versions <target-name>`.

Unfortunately, Go standard library versions are not discoverable via `go list`. 
In order to discover Go versions, offsets-tracker can fetch the versions published at `https://go.dev/dl`.
Fetching `go.dev` for discovering versions can be enabled by setting`.FindVersionsBy(target.GoDevFileVersionsStrategy)` when registering a new target.

## Download Strategy

offsets-tracker wraps every Go module version as a Go application that depends on that module.
Those applications are the result of [generating template files](https://github.com/keyval-dev/offsets-tracker/tree/master/downloader/wrapper) with the appropriate version.

In the case of the Go standard library, offsets-tracker downloads the published binary for the specified version. 

## Version Constraints

offsets-tracker downloads and compiles every version found in the previous step by default.
Some targets do not require support for very old versions. Add the following to limit the version scope:
```go
minimunGoVersion, err := version.NewConstraint(">= 1.12")

target.New('go')
... 
VersionConstraint(&minimunGoVersion)
```

## Project Status

This project is currently in Alpha.
Check out our [issues section](https://github.com/keyval-dev/offsets-tracker/issues) to learn more about improvements we're working on.

## License

This project is licensed under the terms of the Apache 2.0 open source license. Please refer to [LICENSE](LICENSE) for the full terms.
