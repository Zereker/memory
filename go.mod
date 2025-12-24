module github.com/Zereker/memory

go 1.24.3

require (
	github.com/IBM/sarama v1.46.3
	github.com/cloudwego/hertz v0.10.3
	github.com/firebase/genkit/go v1.2.0
	github.com/google/uuid v1.6.0
	github.com/lestrrat-go/file-rotatelogs v2.4.0+incompatible
	github.com/mitchellh/mapstructure v1.5.0
	github.com/neo4j/neo4j-go-driver/v5 v5.28.4
	github.com/openai/openai-go v1.12.0
	github.com/opensearch-project/opensearch-go/v4 v4.6.0
	github.com/pelletier/go-toml/v2 v2.2.4
	github.com/pkg/errors v0.9.1
	github.com/redis/go-redis/v9 v9.17.2
	github.com/stretchr/testify v1.11.1
	golang.org/x/sync v0.19.0
)

require (
	github.com/bahlo/generic-list-go v0.2.0 // indirect
	github.com/buger/jsonparser v1.1.1 // indirect
	github.com/bytedance/gopkg v0.1.1 // indirect
	github.com/bytedance/sonic v1.14.0 // indirect
	github.com/bytedance/sonic/loader v0.3.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cloudwego/base64x v0.1.5 // indirect
	github.com/cloudwego/gopkg v0.1.4 // indirect
	github.com/cloudwego/netpoll v0.7.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/eapache/go-resiliency v1.7.0 // indirect
	github.com/eapache/go-xerial-snappy v0.0.0-20230731223053-c322873962e3 // indirect
	github.com/eapache/queue v1.1.0 // indirect
	github.com/fsnotify/fsnotify v1.5.4 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/goccy/go-yaml v1.18.0 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/golang/snappy v1.0.0 // indirect
	github.com/google/dotprompt/go v0.0.0-20251014011017-8d056e027254 // indirect
	github.com/hashicorp/go-uuid v1.0.3 // indirect
	github.com/invopop/jsonschema v0.13.0 // indirect
	github.com/jcmturner/aescts/v2 v2.0.0 // indirect
	github.com/jcmturner/dnsutils/v2 v2.0.0 // indirect
	github.com/jcmturner/gofork v1.7.6 // indirect
	github.com/jcmturner/gokrb5/v8 v8.4.4 // indirect
	github.com/jcmturner/rpc/v2 v2.0.3 // indirect
	github.com/jonboulle/clockwork v0.5.0 // indirect
	github.com/klauspost/compress v1.18.2 // indirect
	github.com/klauspost/cpuid/v2 v2.2.9 // indirect
	github.com/lestrrat-go/strftime v1.1.1 // indirect
	github.com/mailru/easyjson v0.9.1 // indirect
	github.com/mbleigh/raymond v0.0.0-20250414171441-6b3a58ab9e0a // indirect
	github.com/nyaruka/phonenumbers v1.0.55 // indirect
	github.com/pierrec/lz4/v4 v4.1.23 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/rcrowley/go-metrics v0.0.0-20250401214520-65e299d6c5c9 // indirect
	github.com/tidwall/gjson v1.18.0 // indirect
	github.com/tidwall/match v1.2.0 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/tidwall/sjson v1.2.5 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/wk8/go-ordered-map/v2 v2.1.8 // indirect
	github.com/xeipuuv/gojsonpointer v0.0.0-20190905194746-02993c407bfb // indirect
	github.com/xeipuuv/gojsonreference v0.0.0-20180127040603-bd5ef7bd5415 // indirect
	github.com/xeipuuv/gojsonschema v1.2.0 // indirect
	github.com/yosida95/uritemplate/v3 v3.0.2 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/otel v1.36.0 // indirect
	go.opentelemetry.io/otel/metric v1.36.0 // indirect
	go.opentelemetry.io/otel/sdk v1.36.0 // indirect
	go.opentelemetry.io/otel/trace v1.36.0 // indirect
	golang.org/x/arch v0.3.0 // indirect
	golang.org/x/crypto v0.46.0 // indirect
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
	google.golang.org/protobuf v1.36.6 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

// Use fixed versions from GitHub forks
replace (
	github.com/firebase/genkit/go => github.com/Zereker/genkit/go v1.2.1-0.20251216034102-64ef67666d4d
	github.com/google/dotprompt/go => github.com/Zereker/dotprompt/go v0.0.0-20251211120905-21db81b128d2
)
