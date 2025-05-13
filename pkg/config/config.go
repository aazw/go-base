package config

type Config struct {
	Host      string    `mapstructure:"host"      validate:"required,hostname"`
	Port      uint      `mapstructure:"port"      validate:"required,port"`
	Postgres  Postgres  `mapstructure:"postgres"  validate:"required"`
	Valkey    Valkey    `mapstructure:"valkey"    validate:""`
	Tempo     Tempo     `mapstructure:"tempo"     validate:""`
	Pyroscope Pyroscope `mapstructure:"pyroscope" validate:""`
}

type Postgres struct {
	Host     string `mapstructure:"host"     validate:"required,hostname"`
	Port     uint   `mapstructure:"port"     validate:"required,port"`
	User     string `mapstructure:"user"     validate:"required"`
	Password string `mapstructure:"password" validate:"required"`
	Database string `mapstructure:"database" validate:"required"`
	SslMode  string `mapstructure:"sslmode"  validate:"required,oneof=disable allow prefer require verify-ca verify-full"`
}

type Valkey struct {
	Host string `mapstructure:"host" validate:"required,hostname"`
	Port uint   `mapstructure:"port" validate:"required,port"`
}

type Tempo struct {
	Host string `mapstructure:"host" validate:"required,hostname"`
	Port uint   `mapstructure:"port" validate:"required,port"`
}

type Pyroscope struct {
	Host string `mapstructure:"host" validate:"required,hostname"`
	Port uint   `mapstructure:"port" validate:"required,port"`
}

func NewConfig() Config {
	return Config{
		Host: "localhost",
		Port: 8080,
		Postgres: Postgres{
			Host:     "localhost",
			Port:     5432,
			User:     "",
			Password: "",
			Database: "",
			SslMode:  "disable",
		},
		Valkey: Valkey{
			Host: "localhost",
			Port: 6379,
		},
		Tempo: Tempo{
			Host: "localhost",
			Port: 4318,
		},
		Pyroscope: Pyroscope{
			Host: "localhost",
			Port: 4040,
		},
	}
}
