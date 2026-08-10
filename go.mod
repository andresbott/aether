module github.com/andresbott/aether

go 1.25.7

require (
	github.com/glebarez/sqlite v1.11.0
	github.com/go-bumbu/config v0.4.0
	github.com/go-bumbu/http v0.5.1
	github.com/go-bumbu/tempo v0.2.0
	github.com/go-bumbu/userauth v0.4.0
	github.com/google/uuid v1.6.0
	github.com/gorilla/mux v1.8.1
	github.com/mattn/go-isatty v0.0.21
	github.com/phsym/console-slog v0.3.1
	github.com/prometheus/client_golang v1.23.2
	github.com/rainycape/unidecode v0.0.0-20150907023854-cb7f23ec59be
	github.com/reugn/go-quartz v0.15.2
	github.com/samber/slog-formatter v1.3.0
	github.com/spf13/cobra v1.10.2
	go.senan.xyz/taglib v0.11.1
	golang.org/x/crypto v0.54.0
	golang.org/x/sync v0.22.0
	golang.org/x/term v0.45.0
	golang.org/x/time v0.15.0
	gorm.io/gorm v1.31.1
)

require (
	github.com/boombuler/barcode v1.0.1-0.20190219062509-6c824513bacc // indirect
	github.com/gorilla/securecookie v1.1.2 // indirect
	github.com/gorilla/sessions v1.4.0 // indirect
	github.com/pquerna/otp v1.5.0 // indirect
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/ebitengine/purego v0.10.1 // indirect
	github.com/gen2brain/webp v0.6.4
	github.com/glebarez/go-sqlite v1.21.2 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.66.1 // indirect
	github.com/prometheus/procfs v0.16.1 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/samber/lo v1.53.0 // indirect
	github.com/samber/slog-common v0.21.0 // indirect
	github.com/samber/slog-multi v1.8.0 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	github.com/tetratelabs/wazero v1.11.1-0.20260428013916-2bbd517b7633 // indirect
	go.yaml.in/yaml/v2 v2.4.2 // indirect
	golang.org/x/image v0.44.0
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.40.0 // indirect
	google.golang.org/protobuf v1.36.8 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	modernc.org/libc v1.22.5 // indirect
	modernc.org/mathutil v1.5.0 // indirect
	modernc.org/memory v1.5.0 // indirect
	modernc.org/sqlite v1.23.1 // indirect
)

replace go.senan.xyz/taglib => github.com/andresbott/go-taglib v0.0.0-20260721170622-b78b455bd52d

replace github.com/go-bumbu/userauth => ../bumbu/userauth
