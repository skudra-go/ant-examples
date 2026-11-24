package app

import (
	"net"
	"net/url"
	"runtime/debug"
	"strconv"

	"azugo.io/azugo/config"
	coreconfig "azugo.io/core/config"
	"azugo.io/core/validation"
	"github.com/spf13/viper"
)

const DefaultAppName = "playground"

// ==== app config ====
type AppConfig struct {
	Name    string `mapstructure:"name" validate:"required"`
	Version string `mapstructure:"version" validate:"required"`
}

func (c *AppConfig) Bind(prefix string, v *viper.Viper) {
	v.SetDefault(prefix+".name", "playground")
	v.SetDefault(prefix+".version", getVersion())

	_ = v.BindEnv(prefix+".name", "APP_NAME")
	_ = v.BindEnv(prefix+".version", "APP_VERSION")
}

func getVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		return info.Main.Version
	}
	return "dev"
}

func (c *AppConfig) Validate(valid *validation.Validate) error {
	return valid.Struct(c)
}

// ==== postgres config ====

type PostgresConfig struct {
	Host     string `mapstructure:"host" validate:"required"`
	Port     int    `mapstructure:"port" validate:"required,min=1,max=65535"`
	User     string `mapstructure:"user" validate:"required"`
	Password string `mapstructure:"password"`
	Database string `mapstructure:"database" validate:"required"`
	SSLMode  string `mapstructure:"sslmode" validate:"required,oneof=disable allow prefer require verify-ca verify-full"`
}

func (c *PostgresConfig) Bind(prefix string, v *viper.Viper) {
	psw, _ := coreconfig.LoadRemoteSecret("POSTGRES_PASSWORD")

	v.SetDefault(prefix+".host", "localhost")
	v.SetDefault(prefix+".port", 5432)
	v.SetDefault(prefix+".user", "postgres")
	v.SetDefault(prefix+".password", psw)
	v.SetDefault(prefix+".database", "postgres")
	v.SetDefault(prefix+".sslmode", "disable")

	_ = v.BindEnv(prefix+".host", "POSTGRES_HOST")
	_ = v.BindEnv(prefix+".port", "POSTGRES_PORT")
	_ = v.BindEnv(prefix+".user", "POSTGRES_USER")
	_ = v.BindEnv(prefix+".password", "POSTGRES_PASSWORD")
	_ = v.BindEnv(prefix+".database", "POSTGRES_DB")
	_ = v.BindEnv(prefix+".sslmode", "POSTGRES_SSLMODE")
}

func (c *PostgresConfig) Validate(valid *validation.Validate) error {
	return valid.Struct(c)
}

func (c *PostgresConfig) DSN() string {
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(c.User, c.Password),
		Host:     net.JoinHostPort(c.Host, strconv.Itoa(c.Port)),
		Path:     "/" + c.Database,
		RawQuery: url.Values{"sslmode": {c.SSLMode}}.Encode(),
	}

	return u.String()
}

// ==== main config ====

type Config struct {
	*config.Configuration `mapstructure:",squash"`

	App      *AppConfig      `mapstructure:"app"`
	Postgres *PostgresConfig `mapstructure:"postgres"`

	Test string `mapstructure:"test"`
}

func NewConfig() *Config {
	return &Config{
		Configuration: config.New(),
	}
}

func (c *Config) Bind(_ string, v *viper.Viper) {
	c.Configuration.Bind("", v)

	c.App = config.Bind(c.App, "app", v)
	c.Postgres = config.Bind(c.Postgres, "postgres", v)

	v.SetDefault("test", "Default value")

	_ = v.BindEnv("test", "TEST")
}

func (c *Config) Validate(valid *validation.Validate) error {
	if err := c.Configuration.Validate(valid); err != nil {
		return err
	}

	if err := c.App.Validate(valid); err != nil {
		return err
	}

	return c.Postgres.Validate(valid)
}
