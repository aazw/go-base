package config

type Config struct {
	Host      string    `mapstructure:"host"      json:"host"      yaml:"host"      validate:"required,hostname"`
	Port      uint      `mapstructure:"port"      json:"port"      yaml:"port"      validate:"required,port"`
	Postgres  Postgres  `mapstructure:"postgres"  json:"postgres"  yaml:"postgres"  validate:"required"`
	Valkey    Valkey    `mapstructure:"valkey"    json:"valkey"    yaml:"valkey"    validate:""`
	Tempo     Tempo     `mapstructure:"tempo"     json:"tempo"     yaml:"tempo"     validate:""`
	Pyroscope Pyroscope `mapstructure:"pyroscope" json:"pyroscope" yaml:"pyroscope" validate:""`
}

type Postgres struct {
	Host     string `mapstructure:"host"     json:"host"     yaml:"host"     validate:"required,hostname"`
	Port     uint   `mapstructure:"port"     json:"port"     yaml:"port"     validate:"required,port"`
	User     string `mapstructure:"user"     json:"user"     yaml:"user"     validate:"required"`
	Password string `mapstructure:"password" json:"password" yaml:"password" validate:"required"`
	Database string `mapstructure:"database" json:"database" yaml:"database" validate:"required"`
	SslMode  string `mapstructure:"sslmode"  json:"sslmode"  yaml:"sslmode"  validate:"required,oneof=disable allow prefer require verify-ca verify-full"`

	MinConns                 int32  `mapstructure:"min_conns"                   json:"min_conns"                   yaml:"min_conns"                   validate:"-"`
	MaxConns                 int32  `mapstructure:"max_conns"                   json:"max_conns"                   yaml:"max_conns"                   validate:"-"`
	MaxConnLifetimeSeconds   uint64 `mapstructure:"max_conn_lifetime_seconds"   json:"max_conn_lifetime_seconds"   yaml:"max_conn_lifetime_seconds"   validate:"-"`
	HealthCheckPeriodSeconds uint64 `mapstructure:"health_check_period_seconds" json:"health_check_period_seconds" yaml:"health_check_period_seconds" validate:"-"`
}

type Valkey struct {
	Host string `mapstructure:"host" json:"host" yaml:"host" validate:"required,hostname"`
	Port uint   `mapstructure:"port" json:"port" yaml:"port" validate:"required,port"`

	DialConnectTimeoutSeconds uint64 `mapstructure:"dial_connect_timeout_seconds" json:"dial_connect_timeout_seconds" yaml:"dial_connect_timeout_seconds" validate:"-"`
	DialReadTimeoutSeconds    uint64 `mapstructure:"dial_read_timeout_seconds"    json:"dial_read_timeout_seconds"    yaml:"dial_read_timeout_seconds"    validate:"-"`
	DialWriteTimeoutSeconds   uint64 `mapstructure:"dial_write_timeout_seconds"   json:"dial_write_timeout_seconds"   yaml:"dial_write_timeout_seconds"   validate:"-"`
}

type Tempo struct {
	Host string `mapstructure:"host" json:"host" yaml:"host" validate:"required,hostname"`
	Port uint   `mapstructure:"port" json:"port" yaml:"port" validate:"required,port"`

	TimeoutSeconds uint64     `mapstructure:"timeout_seconds" json:"timeout_seconds" yaml:"timeout_seconds" validate:"-"`
	Retry          TempoRetry `mapstructure:"retry"           json:"retry"           yaml:"retry"           validate:"-"`
}

type TempoRetry struct {
	Enabled                bool   `mapstructure:"enabled"                  json:"enabled"                  yaml:"enabled"                  validate:"-"`
	InitialIntervalSeconds uint64 `mapstructure:"initial_interval_seconds" json:"initial_interval_seconds" yaml:"initial_interval_seconds" validate:"-"`
	MaxIntervalSeconds     uint64 `mapstructure:"max_interval_seconds"     json:"max_interval_seconds"     yaml:"max_interval_seconds"     validate:"-"`
	MaxElapsedTimeSeconds  uint64 `mapstructure:"max_elapsed_time_seconds" json:"max_elapsed_time_seconds" yaml:"max_elapsed_time_seconds" validate:"-"`
}

type Pyroscope struct {
	Host string `mapstructure:"host" json:"host" yaml:"host" validate:"required,hostname"`
	Port uint   `mapstructure:"port" json:"port" yaml:"port" validate:"required,port"`

	TenantID string `mapstructure:"tenant_id" json:"tenant_id" yaml:"tenant_id" validate:"-"`
}

func NewConfig() Config {
	return Config{
		Host: "localhost", //
		Port: 8080,        //
		Postgres: Postgres{
			Host:                     "localhost", //
			Port:                     5432,        //
			User:                     "",          //
			Password:                 "",          //
			Database:                 "",          //
			SslMode:                  "disable",   //
			MinConns:                 2,           // 2
			MaxConns:                 10,          // 10
			MaxConnLifetimeSeconds:   3600,        // time.Hour
			HealthCheckPeriodSeconds: 60,          // time.Minute
		},
		Valkey: Valkey{
			Host:                      "localhost", //
			Port:                      6379,        //
			DialConnectTimeoutSeconds: 3,           // 3*time.Second
			DialReadTimeoutSeconds:    3,           // 3*time.Second
			DialWriteTimeoutSeconds:   3,           // 3*time.Second
		},
		Tempo: Tempo{
			Host: "localhost", //
			Port: 4318,        //
		},
		Pyroscope: Pyroscope{
			Host: "localhost", //
			Port: 4040,        //
		},
	}
}
