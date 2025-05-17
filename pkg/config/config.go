// pkg/config/config.go
package config

type Config struct {
	Server     Server     `mapstructure:"server"      json:"server"      yaml:"server"      validate:"required"`
	Postgres   Postgres   `mapstructure:"postgres"    json:"postgres"    yaml:"postgres"    validate:"required"`
	Valkey     Valkey     `mapstructure:"valkey"      json:"valkey"      yaml:"valkey"`
	OTLPTrace  OTLPTrace  `mapstructure:"otlp_trace"  json:"otlp_trace"  yaml:"otlp_trace"`
	OTLPMetric OTLPMetric `mapstructure:"otlp_metric" json:"otlp_metric" yaml:"otlp_metric"`
	OTLPLog    OTLPLog    `mapstructure:"otlp_log"    json:"otlp_log"    yaml:"otlp_log"`
	Prometheus Prometheus `mapstructure:"prometheus"  json:"prometheus"  yaml:"prometheus"`
	Pyroscope  Pyroscope  `mapstructure:"pyroscope"   json:"pyroscope"   yaml:"pyroscope"`
}

type Server struct {
	Host string `mapstructure:"host" json:"host" yaml:"host" validate:"required,hostname|ip"`
	Port uint   `mapstructure:"port" json:"port" yaml:"port" validate:"required,gt=0,lte=65535"`
	CORS CORS   `mapstructure:"cors" json:"cors" yaml:"cors"`
	OIDC OIDC   `mapstructure:"oidc" json:"oidc" yaml:"oidc"`
}

type CORS struct {
	// https://github.com/gin-contrib/cors

	Enabled bool `mapstructure:"enabled" json:"enabled" yaml:"enabled"`

	// Enabled==true のとき必須。各要素は
	// * ワイルドカード "*" か
	// * scheme 付き URI（http(s)://...）か
	// * ホスト名（RFC1123）
	AllowOrigins []string `mapstructure:"allow_origins" json:"allow_origins" yaml:"allow_origins" validate:"required_if=Enabled true,dive,hostname_rfc1123|uri|eq=*"`

	// Enabled==true のとき必須。要素は代表的な HTTP メソッドのみ許可
	AllowMethods []string `mapstructure:"allow_methods" json:"allow_methods" yaml:"allow_methods" validate:"required_if=Enabled true,dive,oneof=GET POST PUT PATCH DELETE HEAD OPTIONS"`

	// 任意。ASCII 以外を弾く
	AllowHeaders []string `mapstructure:"allow_headers" json:"allow_headers" yaml:"allow_headers" validate:"omitempty,dive,printascii"`

	// 任意。ASCII 以外を弾く
	ExposeHeaders []string `mapstructure:"expose_headers" json:"expose_headers" yaml:"expose_headers" validate:"omitempty,dive,printascii"`

	AllowCredentials bool `mapstructure:"allow_credentials" json:"allow_credentials" yaml:"allow_credentials" validate:""`

	// 0〜24 時間で制限（CORS 仕様上ブラウザの上限は 24h＝86400 秒）
	MaxAgeHour int32 `mapstructure:"max_age_hour" json:"max_age_hour" yaml:"max_age_hour" validate:"gte=0,lte=24"`
}

type OIDC struct {
	// https://www.keycloak.org/securing-apps/oidc-layers
	Enabled bool `mapstructure:"enabled" json:"enabled" yaml:"enabled"`

	// Well-known configuration endpoint
	WellKnownEndpoint string `mapstructure:"well_known_endpoint" json:"well_known_endpoint" yaml:"well_known_endpoint" validate:"required_if=Enabled true,omitempty,url"`

	// Authorization endpoint
	AuthorizationEndpoint string `mapstructure:"authorization_endpoint" json:"authorization_endpoint" yaml:"authorization_endpoint" validate:"required_if=Enabled true,omitempty,url"`

	// Token endpoint
	TokenEndpoint string `mapstructure:"token_endpoint" json:"token_endpoint" yaml:"token_endpoint" validate:"required_if=Enabled true,omitempty,url"`

	// Userinfo endpoint
	UserinfoEndpoint string `mapstructure:"userinfo_endpoint" json:"userinfo_endpoint" yaml:"userinfo_endpoint" validate:"required_if=Enabled true,omitempty,url"`

	// Logout endpoint
	LogoutEndpoint string `mapstructure:"logout_endpoint" json:"logout_endpoint" yaml:"logout_endpoint" validate:"required_if=Enabled true,omitempty,url"`

	// Certificate endpoint
	CertificateEndpoint string `mapstructure:"certificate_endpoint" json:"certificate_endpoint" yaml:"certificate_endpoint" validate:"required_if=Enabled true,omitempty,url"`

	// Introspection endpoint
	IntrospectionEndpoint string `mapstructure:"introspection_endpoint" json:"introspection_endpoint" yaml:"introspection_endpoint" validate:"required_if=Enabled true,omitempty,url"`

	// Token Revocation endpoint
	TokenRevocationEndpoint string `mapstructure:"token_revocation_endpoint" json:"token_revocation_endpoint" yaml:"token_revocation_endpoint" validate:"required_if=Enabled true,omitempty,url"`

	ClientID     string `mapstructure:"client_id"     json:"client_id"     yaml:"client_id"     validate:"required_if=Enabled true,printascii"`
	ClientSecret string `mapstructure:"client_secret" json:"client_secret" yaml:"client_secret" validate:"required_if=Enabled true,printascii"`
}

type Postgres struct {
	Host     string `mapstructure:"host"     json:"host"     yaml:"host"     validate:"required,hostname|ip"`
	Port     uint   `mapstructure:"port"     json:"port"     yaml:"port"     validate:"required,gt=0,lte=65535"`
	User     string `mapstructure:"user"     json:"user"     yaml:"user"     validate:"required"`
	Password string `mapstructure:"password" json:"password" yaml:"password" validate:"required"`
	Database string `mapstructure:"database" json:"database" yaml:"database" validate:"required"`
	SslMode  string `mapstructure:"sslmode"  json:"sslmode"  yaml:"sslmode"  validate:"required,oneof=disable allow prefer require verify-ca verify-full"`

	MinConns                 int32  `mapstructure:"min_conns"                   json:"min_conns"                   yaml:"min_conns"                   validate:"gte=0"`
	MaxConns                 int32  `mapstructure:"max_conns"                   json:"max_conns"                   yaml:"max_conns"                   validate:"gte=1,gtfield=MinConns"`
	MaxConnLifetimeSeconds   uint64 `mapstructure:"max_conn_lifetime_seconds"   json:"max_conn_lifetime_seconds"   yaml:"max_conn_lifetime_seconds"   validate:"-"`
	HealthCheckPeriodSeconds uint64 `mapstructure:"health_check_period_seconds" json:"health_check_period_seconds" yaml:"health_check_period_seconds" validate:"-"`
}

type Valkey struct {
	Host string `mapstructure:"host" json:"host" yaml:"host" validate:"required,hostname|ip"`
	Port uint   `mapstructure:"port" json:"port" yaml:"port" validate:"required,gt=0,lte=65535"`

	DialConnectTimeoutSeconds uint64 `mapstructure:"dial_connect_timeout_seconds" json:"dial_connect_timeout_seconds" yaml:"dial_connect_timeout_seconds" validate:"gte=0"`
	DialReadTimeoutSeconds    uint64 `mapstructure:"dial_read_timeout_seconds"    json:"dial_read_timeout_seconds"    yaml:"dial_read_timeout_seconds"    validate:"gte=0"`
	DialWriteTimeoutSeconds   uint64 `mapstructure:"dial_write_timeout_seconds"   json:"dial_write_timeout_seconds"   yaml:"dial_write_timeout_seconds"   validate:"gte=0"`
}

type OTLPTrace struct {
	Enabled bool `mapstructure:"enabled" json:"enabled" yaml:"enabled"`

	Host string `mapstructure:"host" json:"host" yaml:"host" validate:"required_if=Enabled true,omitempty,hostname|ip"`
	Port uint   `mapstructure:"port" json:"port" yaml:"port" validate:"required_if=Enabled true,omitempty,gt=0,lte=65535"`

	TimeoutSeconds uint64         `mapstructure:"timeout_seconds" json:"timeout_seconds" yaml:"timeout_seconds" validate:"gte=0"`
	Retry          OTLPTraceRetry `mapstructure:"retry"           json:"retry"           yaml:"retry"`
}

type OTLPTraceRetry struct {
	Enabled bool `mapstructure:"enabled" json:"enabled" yaml:"enabled"`

	InitialIntervalSeconds uint64 `mapstructure:"initial_interval_seconds" json:"initial_interval_seconds" yaml:"initial_interval_seconds" validate:"required_if=Enabled true,omitempty,gt=0"`
	MaxIntervalSeconds     uint64 `mapstructure:"max_interval_seconds"     json:"max_interval_seconds"     yaml:"max_interval_seconds"     validate:"required_if=Enabled true,omitempty,gtefield=InitialIntervalSeconds"`
	MaxElapsedTimeSeconds  uint64 `mapstructure:"max_elapsed_time_seconds" json:"max_elapsed_time_seconds" yaml:"max_elapsed_time_seconds" validate:"required_if=Enabled true,omitempty,gtefield=MaxIntervalSeconds"`
}

type OTLPMetric struct {
	Enabled bool `mapstructure:"enabled" json:"enabled" yaml:"enabled"`

	Host string `mapstructure:"host" json:"host" yaml:"host" validate:"required_if=Enabled true,omitempty,hostname|ip"`
	Port uint   `mapstructure:"port" json:"port" yaml:"port" validate:"required_if=Enabled true,omitempty,gt=0,lte=65535"`
}

type OTLPLog struct {
	Enabled bool `mapstructure:"enabled" json:"enabled" yaml:"enabled"`

	Host string `mapstructure:"host" json:"host" yaml:"host" validate:"required_if=Enabled true,omitempty,hostname|ip"`
	Port uint   `mapstructure:"port" json:"port" yaml:"port" validate:"required_if=Enabled true,omitempty,gt=0,lte=65535"`
}

type Prometheus struct {
	Enabled bool `mapstructure:"enabled" json:"enabled" yaml:"enabled"`

	MetricsPath string `mapstructure:"metrics_path" json:"metrics_path" yaml:"metrics_path"`
}

type Pyroscope struct {
	Enabled bool `mapstructure:"enabled" json:"enabled" yaml:"enabled"`

	Host string `mapstructure:"host" json:"host" yaml:"host" validate:"required_if=Enabled true,omitempty,hostname|ip"`
	Port uint   `mapstructure:"port" json:"port" yaml:"port" validate:"required_if=Enabled true,omitempty,gt=0,lte=65535"`

	TenantID string `mapstructure:"tenant_id" json:"tenant_id" yaml:"tenant_id" validate:"omitempty,printascii"`
}

func NewConfig() Config {
	return Config{
		Server: Server{
			Host: "0.0.0.0", //
			Port: 8080,      //
			CORS: CORS{
				Enabled: false,
			},
			OIDC: OIDC{
				Enabled: false,
			},
		},
		Postgres: Postgres{
			Host:                     "postgres", //
			Port:                     5432,       //
			User:                     "",         //
			Password:                 "",         //
			Database:                 "",         //
			SslMode:                  "disable",  //
			MinConns:                 2,          // 2
			MaxConns:                 10,         // 10
			MaxConnLifetimeSeconds:   3600,       // time.Hour
			HealthCheckPeriodSeconds: 60,         // time.Minute
		},
		Valkey: Valkey{
			Host:                      "valkey", //
			Port:                      6379,     //
			DialConnectTimeoutSeconds: 3,        // 3*time.Second
			DialReadTimeoutSeconds:    3,        // 3*time.Second
			DialWriteTimeoutSeconds:   3,        // 3*time.Second
		},
		OTLPTrace: OTLPTrace{
			Enabled: false,
			Host:    "opentelemetry-collector", //
			Port:    4318,                      //
		},
		OTLPMetric: OTLPMetric{
			Enabled: false,
			Host:    "opentelemetry-collector", //
			Port:    4318,                      //
		},
		OTLPLog: OTLPLog{
			Enabled: false,
			Host:    "opentelemetry-collector", //
			Port:    4318,                      //
		},
		Prometheus: Prometheus{
			Enabled:     false,
			MetricsPath: "/metrics",
		},
		Pyroscope: Pyroscope{
			Enabled: false,
			Host:    "pyroscope", //
			Port:    4040,        //
		},
	}
}
