// Every setting is read from an environment variable. A .env file is loaded as
// a developer convenience only: variables that already exist in the real
// environment always win, and a missing .env file is not an error.
//
// # How environment variable names are derived
//
// The name is the section name and the field name joined with an underscore:
// App.Env             -> APP_ENV
// Log.Level           -> LOG_LEVEL
// HTTP.Port           -> HTTP_PORT
// I18N.DefaultLanguage -> I18N_DEFAULT_LANGUAGE
//
// Multi-word fields additionally need the `split_words` struct tag, otherwise
// the underscores are not inserted (HTTP.ReadTimeout would become HTTP_READTIMEOUT).
package config

import (
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
	"go.uber.org/fx"
)

var Module = fx.Module("config", fx.Provide(Load))

type Config struct {
	App   AppConfig
	Log   LogConfig
	HTTP  HTTPConfig
	GRPC  GRPCConfig
	DB    DBConfig
	I18N  I18NConfig
	Users UsersConfig
}

// The available languages are not configured here: they are discovered from the
// catalog files in internal/i18n/locales, so the two cannot drift apart.
type I18NConfig struct {
	// DefaultLanguage is used when a request expresses no usable preference.
	// It must have a catalog.
	DefaultLanguage string `split_words:"true" default:"en" validate:"required"`

	// QueryParam is an explicit per-request override.
	QueryParam string `split_words:"true" default:"lang"`

	// CookieName is an explicit per-user preference,
	// used when the query parameter is absent.
	CookieName string `split_words:"true" default:"lang"`
}

type AppConfig struct {
	// Env is the deployment environment: development, staging or production.
	Env AppEnv `default:"development" validate:"required,validEnv"`

	// Name is used for log fields and the fiber application name.
	Name string `default:"go-fiber-template"`
}

type LogConfig struct {
	// Level is the minimum severity that is logged.
	Level LogLevel `default:"info" validate:"required,validLogLevel"`

	// File is optional. When empty (the default) the process logs to stdout
	// only, which is what container platforms expect. When set, logs are
	// written to that path as rotated JSON in addition to stdout.
	File string `default:""`
}

type HTTPConfig struct {
	Host            string        `default:"0.0.0.0"`
	Port            int           `default:"3000" validate:"required,min=1,max=65535"`
	ReadTimeout     time.Duration `split_words:"true" default:"10s"`
	WriteTimeout    time.Duration `split_words:"true" default:"10s"`
	IdleTimeout     time.Duration `split_words:"true" default:"60s"`
	ShutdownTimeout time.Duration `split_words:"true" default:"10s"`

	// BodyLimit is the maximum accepted request body size in bytes.
	BodyLimit int `split_words:"true" default:"4194304"` // 4 MiB

	// CORSAllowedOrigins is a comma-separated list of allowed origins.
	// "*" allows every origin, which is only appropriate for development.
	CORSAllowedOrigins []string `split_words:"true" default:"*"`
}

// The host:port string the HTTP server should listen on.
func (c HTTPConfig) Addr() string {
	return net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
}

type GRPCConfig struct {
	Enabled           bool          `default:"true"`
	Host              string        `default:"0.0.0.0"`
	Port              int           `default:"9090" validate:"required,min=1,max=65535"`
	ShutdownTimeout   time.Duration `split_words:"true" default:"10s"`
	MaxRecvMsgSize    int           `split_words:"true" default:"4194304"` // 4MB
	MaxSendMsgSize    int           `split_words:"true" default:"4194304"` // 4MB
	ConnectionTimeout time.Duration `split_words:"true" default:"10s"`

	// ReflectionEnabled exposes the service descriptors so tools like grpcurl
	// can discover the API without a local copy of the .proto files. It also
	// advertises the whole surface, so it is usually off in production.
	ReflectionEnabled bool `split_words:"true" default:"true"`
}

// The host:port string the gRPC server should listen on.
func (c GRPCConfig) Addr() string {
	return net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
}

// UsersConfig is where the gateway dials the users service.
type UsersConfig struct {
	Addr string `default:"localhost:9090"`
}

type DBConfig struct {
	// When set, is used verbatim and every field below is ignored.
	URL string `split_words:"true" default:""`

	Host     string  `default:"localhost"`
	Port     int     `default:"5432" validate:"required,min=1,max=65535"`
	User     string  `default:"postgres"`
	Password string  `default:"postgres"`
	Name     string  `default:"app"`
	SSLMode  SSLMode `split_words:"true" default:"disable" validate:"required,validSSLMode"`

	MaxConns        int32         `split_words:"true" default:"10" validate:"required,min=1"`
	MinConns        int32         `split_words:"true" default:"1" validate:"min=0"`
	MaxConnLifetime time.Duration `split_words:"true" default:"1h"`
	MaxConnIdleTime time.Duration `split_words:"true" default:"30m"`
	ConnectTimeout  time.Duration `split_words:"true" default:"5s"`

	// Applies the embedded migrations during startup.
	// Turn it off when migrations are a separate deployment step.
	AutoMigrate bool `split_words:"true" default:"true"`
}

func (c DBConfig) DSN() string {
	if c.URL != "" {
		return c.URL
	}

	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(c.User, c.Password),
		Host:   net.JoinHostPort(c.Host, strconv.Itoa(c.Port)),
		Path:   "/" + c.Name,
	}

	query := u.Query()
	query.Set("sslmode", string(c.SSLMode))
	u.RawQuery = query.Encode()

	return u.String()
}

// DSN with the password removed, for logging.
func (c DBConfig) SafeDSN() string {
	if c.URL != "" {
		parsed, err := url.Parse(c.URL)
		if err != nil {
			return "(unparseable DB_URL)"
		}
		return parsed.Redacted()
	}

	return net.JoinHostPort(c.Host, strconv.Itoa(c.Port)) + "/" + c.Name
}

func Load() (*Config, error) {
	loadDotEnv()

	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, fmt.Errorf("parse configuration from environment: %w", err)
	}

	validate, err := newValidator()
	if err != nil {
		return nil, fmt.Errorf("initialise configuration validator: %w", err)
	}

	if err := validate.Struct(&cfg); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

func loadDotEnv() {
	for _, path := range []string{".env", "backend/.env"} {
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			// Normal outsise of development environment
			continue
		}

		if err := godotenv.Load(path); err != nil {
			log.Printf("config: could not read %s: %v", path, err)
			return
		}

		log.Printf("config: loaded environment from %s", path)
		return
	}
}
