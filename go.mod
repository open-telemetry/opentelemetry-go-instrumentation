module go.opentelemetry.io/auto

go 1.22.7

retract (
	v0.6.5 // Contains retractions only.
	v0.6.4 // Published accidentally.
	v0.6.3 // Published accidentally.
	v0.6.2 // Published accidentally.
	v0.6.1 // Published accidentally.
	v0.6.0 // Published accidentally.
	v0.5.4 // Published accidentally.
	v0.5.3 // Published accidentally.
	v0.5.2 // Published accidentally.
	v0.5.1 // Published accidentally.
	v0.5.0 // Published accidentally.
	v0.0.0 // Published accidentally.
)

require (
	github.com/MrAlias/collex v0.0.5-alpha
	github.com/cilium/ebpf v0.17.1
	github.com/hashicorp/go-version v1.7.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter v0.115.0
	github.com/pkg/errors v0.9.1
	github.com/stretchr/testify v1.10.0
	github.com/xwb1989/sqlparser v0.0.0-20180606152119-120387863bf2
	go.opentelemetry.io/collector/pdata v1.22.0
	go.opentelemetry.io/contrib/exporters/autoexport v0.58.0
	go.opentelemetry.io/otel v1.33.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.33.0
	go.opentelemetry.io/otel/sdk v1.33.0
	go.opentelemetry.io/otel/trace v1.33.0
	golang.org/x/arch v0.12.0
	golang.org/x/sys v0.28.0
	google.golang.org/grpc v1.69.2
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.2.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.25.1 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.17.11 // indirect
	github.com/knadh/koanf/maps v0.1.1 // indirect
	github.com/knadh/koanf/providers/confmap v0.1.0 // indirect
	github.com/knadh/koanf/v2 v2.1.2 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent v0.115.0 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_golang v1.20.5 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/common v0.61.0 // indirect
	github.com/prometheus/procfs v0.15.1 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/collector/component v0.115.0 // indirect
	go.opentelemetry.io/collector/config/configretry v1.21.0 // indirect
	go.opentelemetry.io/collector/config/configtelemetry v0.115.0 // indirect
	go.opentelemetry.io/collector/confmap v1.21.0 // indirect
	go.opentelemetry.io/collector/consumer v1.21.0 // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.115.0 // indirect
	go.opentelemetry.io/collector/exporter v0.115.0 // indirect
	go.opentelemetry.io/collector/extension v0.115.0 // indirect
	go.opentelemetry.io/collector/extension/experimental/storage v0.115.0 // indirect
	go.opentelemetry.io/collector/featuregate v1.21.0 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.115.0 // indirect
	go.opentelemetry.io/collector/pipeline v0.115.0 // indirect
	go.opentelemetry.io/contrib/bridges/prometheus v0.58.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc v0.9.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp v0.9.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc v1.33.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp v1.33.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.33.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.33.0 // indirect
	go.opentelemetry.io/otel/exporters/prometheus v0.55.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdoutlog v0.9.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdoutmetric v1.33.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.33.0 // indirect
	go.opentelemetry.io/otel/log v0.9.0 // indirect
	go.opentelemetry.io/otel/metric v1.33.0 // indirect
	go.opentelemetry.io/otel/sdk/log v0.9.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.33.0 // indirect
	go.opentelemetry.io/proto/otlp v1.4.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	golang.org/x/net v0.33.0 // indirect
	golang.org/x/text v0.21.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20241219192143-6b3ec007d9bb // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20241219192143-6b3ec007d9bb // indirect
	google.golang.org/protobuf v1.36.0 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.2.1 // indirect
)
