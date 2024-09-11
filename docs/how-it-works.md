# OpenTelemetry Go Instrumentation - How it works

We aim to bring the automatic instrumentation experience found in languages like [Java](https://github.com/open-telemetry/opentelemetry-java-instrumentation), [Python](https://github.com/open-telemetry/opentelemetry-python-contrib) and [JavaScript](https://github.com/open-telemetry/opentelemetry-js-contrib) to Go applications.

## Design Goals

- No code changes required - any Go application can be instrumented without modifying the source code.
- Support wide range of Go applications - instrumentation is supported for Go version 1.12 and above. In addition, a common practice for Go applications is to shrink the binary size by stripping debug symbols via `go build -ldflags "-s -w"`. This instrumentation works for stripped binaries as well.
- Configuration is done via `OTEL_*` environment variables according to [OpenTelemetry Environment Variable Specification](https://opentelemetry.io/docs/reference/specification/sdk-environment-variables/#general-sdk-configuration).
  **In order to inject instrumentation into your process, set the `OTEL_GO_AUTO_TARGET_EXE` environment variable to the path of the target executable. This is not a part of the OTEL specification mentioned above.**
- Instrumented libraries follow the [OpenTelemetry specification](https://github.com/open-telemetry/opentelemetry-specification) and semantic conventions to produce standard OpenTelemetry data.

## Why eBPF

Go is a compiled language. Unlike languages such as Java and Python, Go compiles natively to machine code. This makes it impossible to add additional code at runtime to instrument Go applications.
Fortunately, the Linux kernel provides a mechanism to attach user-defined code to the execution of a process. This is called [eBPF](https://ebpf.io/) and it is widely used in other Cloud Native projects such as Cilium and Falco.

## How it works

The Go auto-instrumentation agent runs as a single binary that analyzes a target Go process (the application you want to instrument) and attaches eBPF programs to hooks in the target process. To do this, the agent starts three objects internally:

* A process `Analyzer`, which finds the target process and detects library functions that the agent can auto-instrument.
* An OpenTelemetry `Controller`, which uses the [OpenTelemetry Go SDK](https://opentelemetry.io/docs/languages/go/) to export telemetry data.
* An Instrumentation `Manager`, which coordinates sending events received from eBPF programs to the OpenTelemetry `Controller`.

The agent uses the [Cilium eBPF libraries for Go](https://github.com/cilium/ebpf) to do much of the fundamental eBPF handling such as loading programs and reading events.

### The `Instrumentation` object

The main entry point for the agent is the [`Instrumentation`](https://pkg.go.dev/go.opentelemetry.io/auto#Instrumentation) object, defined in [`instrumentation.go`](https://github.com/open-telemetry/opentelemetry-go-instrumentation/blob/02686f0a149b13fec4da73b9d40233a372fbd13b/instrumentation.go). This object coordinates the `Controller`, `Manager`, and `Analyzer` to orchestrate the auto-instrumentation of a process.

### Finding the target process on the host

When the agent starts up, it first calls [`process.NewAnalyzer`](https://pkg.go.dev/go.opentelemetry.io/auto@v0.10.1-alpha/internal/pkg/process#NewAnalyzer) which creates the [`Analyzer`](https://pkg.go.dev/go.opentelemetry.io/auto@v0.10.1-alpha/internal/pkg/process#Analyzer) object. The `Analyzer` then calls [`a.DiscoverProcessID()`](https://pkg.go.dev/go.opentelemetry.io/auto@v0.10.1-alpha/internal/pkg/process#Analyzer.DiscoverProcessID), which loops through the host's `/proc` directory to find a process matching the target binary's name.

### Registering instrumentation function probes

Next, the `Instrumentation` object calls [`i.NewManager()`](https://pkg.go.dev/go.opentelemetry.io/auto@v0.10.1-alpha/internal/pkg/instrumentation#NewManager) to create a [`Manager`](https://pkg.go.dev/go.opentelemetry.io/auto@v0.10.1-alpha/internal/pkg/instrumentation#Manager) object. The `Manager` object  holds a map of the probed symbols, a channel to receive eBPF events, and a reference the the OpenTelemetry `Controller` to hand off parsed events and export telemetry.

Calling `NewManager()` uses [`m.registerProbes()`](https://github.com/open-telemetry/opentelemetry-go-instrumentation/blob/02686f0a149b13fec4da73b9d40233a372fbd13b/internal/pkg/instrumentation/manager.go#L216) to instantiate each instrumented library's [`Probe` object](https://pkg.go.dev/go.opentelemetry.io/auto@v0.10.1-alpha/internal/pkg/instrumentation/probe#Probe).

Each Go library that is supported by the auto-instrumentation agent implements its own `Probe`. For example, the [gRPC `New()` function](https://pkg.go.dev/go.opentelemetry.io/auto@v0.10.1-alpha/internal/pkg/instrumentation/bpf/google.golang.org/grpc/client#New) does the following:

* Creates an [`ID`](https://pkg.go.dev/go.opentelemetry.io/auto@v0.10.1-alpha/internal/pkg/instrumentation/probe#ID) that uniquely identifies the `Probe` by its OpenTelemetry [`trace.SpanKind`](https://pkg.go.dev/go.opentelemetry.io/otel/trace#SpanKind) and package name.
* Defines the [`probe.Const`](https://pkg.go.dev/go.opentelemetry.io/auto@v0.10.1-alpha/internal/pkg/instrumentation/probe#Const) values to configure settings and define the relevant struct fields (and their offsets) for parsing telemetry data. Each `Const` is an object that implements the `InjectOption()` function to configure settings on the target process. For example, [`RegistersABIConst`](https://pkg.go.dev/go.opentelemetry.io/auto@v0.10.1-alpha/internal/pkg/instrumentation/probe#RegistersABIConst) calls [`inject.WithRegistersABI()`](https://pkg.go.dev/go.opentelemetry.io/auto@v0.10.1-alpha/internal/pkg/inject#WithRegistersABI).
* [Configures functions](https://github.com/open-telemetry/opentelemetry-go-instrumentation/blob/02686f0a149b13fec4da73b9d40233a372fbd13b/internal/pkg/instrumentation/bpf/google.golang.org/grpc/client/probe.go#L75) to attach Uprobes. Each function is represented by a [`probe.Uprobe` object](https://pkg.go.dev/go.opentelemetry.io/auto@v0.10.1-alpha/internal/pkg/instrumentation/probe#Uprobe) that holds the name of the function, the Cilium library call that will attach the eBPF program to the function, and an additional flag to indicate whether the Uprobe is optional.
* [Creates](https://github.com/open-telemetry/opentelemetry-go-instrumentation/blob/02686f0a149b13fec4da73b9d40233a372fbd13b/internal/pkg/instrumentation/bpf/google.golang.org/grpc/client/probe.go#L94) a `ReaderFn` that returns a Perf event reader from the Cilium libraries.
* [Creates](https://github.com/open-telemetry/opentelemetry-go-instrumentation/blob/02686f0a149b13fec4da73b9d40233a372fbd13b/internal/pkg/instrumentation/bpf/google.golang.org/grpc/client/probe.go#L97) a `SpecFn` that holds a reference the eBPF collection from the Cilium libraries.
* [Creates](https://github.com/open-telemetry/opentelemetry-go-instrumentation/blob/02686f0a149b13fec4da73b9d40233a372fbd13b/internal/pkg/instrumentation/bpf/google.golang.org/grpc/client/probe.go#L98) a `ProcessFn` that converts bytes to eBPF events.

Once each library's `Probe` object is created, the `Manager` calls [`m.registerProbe()`](https://github.com/open-telemetry/opentelemetry-go-instrumentation/blob/02686f0a149b13fec4da73b9d40233a372fbd13b/internal/pkg/instrumentation/manager.go#L73) on each one. This uses the `Probe`'s `Id` to store a reference to the `Probe` in a map. In doing this, the `Manager` calls `Manifest()` on each `Probe`. This in turn calls [`NewManifest()`](https://pkg.go.dev/go.opentelemetry.io/auto@v0.10.1-alpha/internal/pkg/instrumentation/probe#NewManifest) that returns a [`Manifest`](https://pkg.go.dev/go.opentelemetry.io/auto@v0.10.1-alpha/internal/pkg/instrumentation/probe#Manifest) object containing the `Id`, `StructFields`, and `Symbols` for the `Probe`.

### Analyzing Go Process details

Next, the `Instrumentation` object calls [`analyzer.Analyze()`](https://pkg.go.dev/go.opentelemetry.io/auto@v0.10.1-alpha/internal/pkg/process#Analyzer.Analyze) to get the Go details and Modules (dependencies) for the target binary.

The `Analyze()` function first creates a [`TargetDetails` object](https://pkg.go.dev/go.opentelemetry.io/auto@v0.10.1-alpha/internal/pkg/process#TargetDetails) to hold information about the target process. By opening `proc/{pid}/exe` (using the PID it found earlier in `DiscoverProcessID()`), it can then read the Go build info, Go version, dependencies, and instrumentable functions in the process. When calling `analyzer.Analyze()`, the `Instrumentation` object passes [`manager.GetRelevantFuncs()`](https://pkg.go.dev/go.opentelemetry.io/auto@v0.10.1-alpha/internal/pkg/instrumentation#Manager.GetRelevantFuncs) as an argument. This function gets the list of `Symbol`s from each `Probe` by checking its `Manifest`, telling `Analyze()` which functions to locate.

With the list of relevant functions, the `Analyzer` can call [`binary.FindFunctionsStripped()`](https://pkg.go.dev/go.opentelemetry.io/auto@v0.10.1-alpha/internal/pkg/process/binary#FindFunctionsStripped) or [`binary.FindFunctionsUnStripped`](https://pkg.go.dev/go.opentelemetry.io/auto@v0.10.1-alpha/internal/pkg/process/binary#FindFunctionsUnStripped) to get the exact location of the functions in the process (see [Instrumentation Stability](#instrumentation-stability) below for more details on this). Once it has that data, it returns a list of [`binary.Func` objects](https://pkg.go.dev/go.opentelemetry.io/auto@v0.10.1-alpha/internal/pkg/process/binary#Func) for each relevant function.

The result of calling `Analyze()` is a `TargetDetails` object.

### Allocating memory for eBPF maps

The `Instrumentation` object calls [`process.Allocate()`](https://pkg.go.dev/go.opentelemetry.io/auto@v0.10.1-alpha/internal/pkg/process#Allocate) to allocate memory for the agent to access eBPF maps. It does this by first calling [`ptrace.NewTracedProgram()`](https://pkg.go.dev/go.opentelemetry.io/auto@v0.10.1-alpha/internal/pkg/process/ptrace#NewTracedProgram) to attach to the target process with `ptrace`. It then makes a number of syscalls to set up the eBPF map:

* `mmap` to create the map
* `madvise` to tell the kernel it will need to read this address soon
* `mlock` to lock the address into RAM

The address of the map data is returned in an [`AllocationDetails` object](https://pkg.go.dev/go.opentelemetry.io/auto@v0.10.1-alpha/internal/pkg/process#AllocationDetails), which is stored in the `TargetDetails` built above.

### Loading eBPF programs

With all of the initialization steps complete (analyzing the target process, finding relevant instrumentation points, allocating memory), the agent calls [`instrumentation.Run()`](https://pkg.go.dev/go.opentelemetry.io/auto#Instrumentation.Run) to start the `Instrumentation` object. This calls [`manager.Run()`](https://pkg.go.dev/go.opentelemetry.io/auto@v0.10.1-alpha/internal/pkg/instrumentation#Manager.Run) and starts the `Manager` object within the `Instrumentation`.

The `Manager` mounts the target binary and calls [`bpffs.Mount()`](https://pkg.go.dev/go.opentelemetry.io/auto@v0.10.1-alpha/internal/pkg/instrumentation/bpffs#Mount) to create a subdirectory for the target executable under `/sys/fs/bpf`. It then calls [`probe.Load()`](https://pkg.go.dev/go.opentelemetry.io/auto@v0.10.1-alpha/internal/pkg/instrumentation/probe#Base.Load) on each registered `Probe`. At this point the inject options are applied to the target, and this is where offsets are loaded from the embedded JSON file with [`inject.WithOffset()`](https://pkg.go.dev/go.opentelemetry.io/auto@v0.10.1-alpha/internal/pkg/inject#WithOffset) (see [Instrumentation Stability](#instrumentation-stability)).

It then builds a [Cilium `CollectionSpec`](https://pkg.go.dev/github.com/cilium/ebpf#CollectionSpec) and calls [`LoadAndAssign()`](https://pkg.go.dev/github.com/cilium/ebpf#CollectionSpec.LoadAndAssign) to load the eBPF map and program into the kernel. It analyzes the relevant Uprobes and stores their links.

After each `Probe` is loaded, they start separate goroutines with [`probe.Run()`](https://pkg.go.dev/go.opentelemetry.io/auto@v0.10.1-alpha/internal/pkg/instrumentation/probe#Base.Run). This function will send [`Event`s](https://pkg.go.dev/go.opentelemetry.io/auto@v0.10.1-alpha/internal/pkg/instrumentation/probe#Event) out to the `Manager` on a channel called `dest` (or `incomingEvents` in the `Manager`).

### Processing events

Each goroutine starts an infinite loop that blocks on a call to the [Cilium `reader.Read()` function](https://pkg.go.dev/github.com/cilium/ebpf/perf#Reader.Read). This reads from the perf ring buffer when there are bytes available.

When an event is received, it is processed from byte data to an eBPF event by the `Probe`'s `ProcessFn` (each instrumented library implements its own `ProcessFn`, as set during the `New()` call when the library was registered).

### Exporting events

With the `Probe`s running in their own goroutines, the `Manager` starts another loop listening on the `m.incomingEvents` channel for `Event`s. When a `Probe` sends a processed `Event` out on its `dest` channel, the `Manager` receives it to the OpenTelemetry `Controller` object with [`c.Trace()`](https://pkg.go.dev/go.opentelemetry.io/auto@v0.10.1-alpha/internal/pkg/opentelemetry#Controller.Trace). The `Controller` then uses the standard OpenTelemetry Go SDK to export the trace.

## Main Challenges and How We Overcome Them

Using eBPF to instrument Go applications is non-trivial. In the following sections we will describe the main challenges and how we solved them.

### Instrumentation Stability

eBPF programs access user code and variables by analyzing the stack and the CPU registers. For example, to read the value of the `target` field in the `google.golang.org/grpc.ClientConn` struct (see gRPC probe for an example), the eBPF program needs to know the offset of the field inside the struct. The offset is determined by the field location inside the struct definition.

Hard coding this offset information into the eBPF programs creates a very unstable instrumentation. Fields locations inside structs are subject to change and the eBPF program needs to be recompiled every time the struct definition changes.
Luckily for us, there is a way to analyze the target binary and extract the required offsets, by using DWARF. The DWARF debug information is generated by the compiler and is stored inside the binary.

Notice that one of our design goals is to support stripped Go binaries - meaning binaries that do not contain debug information. In order to support stripped binaries and to create a stable instrumentation, we created a library called [offsets-tracker](https://github.com/keyval-dev/offsets-tracker). This library tracks the offset of different fields across versions.

We currently track instrumented structs inside the Go standard library and selected open source packages. This solution does not require DWARF information on the target binary and provides stability to instrumentations. Instrumentation authors can get a field location by name instead of hard coding a field offset.

The offsets-tracker generates the [offset_results.json](../internal/pkg/inject/offset_results.json) file. This file contains the offsets of the fields in the instrumented structs.

### Uretprobes

One of the basic requirements of OpenTelemetry spans is to contain start timestamp and end timestamp. Getting those timestamps is possible by placing an eBPF code at the start and the end of the instrumented function. eBPF supports this requirement via uprobes and uretprobes. Uretprobes are used to invoke eBPF code at the end of the function. Unfortunately, uretprobes and Go [do not play well together](https://github.com/golang/go/issues/22008).

We overcome this issue by analyzing the target binary and detecting all the return statements in the instrumented functions. We then place a uprobe at the end of each return statement. This uprobe invokes the eBPF code that collects the end timestamp.

### Timestamp tracking

eBPF programs can access the current timestamp by calling `bpf_ktime_get_ns()`. The value returned by this function is fetched from the `CLOCK_MONOTONIC` clock and represents the number of nanoseconds since the system boot time.

According to OpenTelemetry specification start time and end time should be timestamps and represent exact point in time. Converting from monotonic time to epoch timestamp is automatically handled by this library. Conversion is achieved by discovering the epoch boot time and adding it to the monotonic time collected by the eBPF program.

### Support Go 1.17 and above

Since version 1.17 and above, Go [changed the way it passes arguments to functions](https://go.googlesource.com/go/+/refs/heads/dev.regabi/src/cmd/compile/internal-abi.md#function-call-argument-and-result-passing).
Prior to version 1.17, Go placed arguments in the stack in the order they were defined in the function signature. Version 1.17 and above uses the machine registers to pass arguments.

We overcome this by analyzing the target binary and detecting the compiled Go version. If the compiled Go version is 1.17 or above, we read arguments from the machine registers. If the compiled Go version is below 1.17, we read arguments from the stack. This should be transparent to the instrumentation authors and abstracted by a function named `get_argument()`.
