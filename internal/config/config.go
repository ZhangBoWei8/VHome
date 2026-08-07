package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	App  AppConfig
	HTTP HTTPConfig
	DB   DBConfig
	Auth AuthConfig
}

type AppConfig struct {
	Env string
}

type HTTPConfig struct {
	Addr              string
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
}

type DBConfig struct {
	Host            string
	Port            int
	User            string
	Password        string
	DBName          string
	Location        string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
	ConnectTimeout  time.Duration
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
}

type AuthConfig struct {
	SessionTTL   time.Duration
	CookieName   string
	CookieSecure bool
}

func defaultConfig() Config {
	return Config{
		App: AppConfig{
			Env: "development",
		},
		HTTP: HTTPConfig{
			Addr:              ":8080",
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       15 * time.Second,
			WriteTimeout:      15 * time.Second,
			IdleTimeout:       60 * time.Second,
			ShutdownTimeout:   10 * time.Second,
		},
		DB: DBConfig{
			Host:            "127.0.0.1",
			Port:            3306,
			User:            "vhome",
			Password:        "",
			DBName:          "vhome",
			Location:        "Asia/Shanghai",
			MaxOpenConns:    20,
			MaxIdleConns:    20,
			ConnMaxLifetime: 3 * time.Minute,
			ConnMaxIdleTime: 1 * time.Minute,
			ConnectTimeout:  5 * time.Second,
			ReadTimeout:     5 * time.Second,
			WriteTimeout:    5 * time.Second,
		},
		Auth: AuthConfig{
			SessionTTL:   14 * 24 * time.Hour,
			CookieName:   "vhome_session",
			CookieSecure: false,
		},
	}
}

func loadEnvFile(filename string) error {
	if filename == "" {
		return nil
	}

	if err := godotenv.Load(filename); err != nil {
		return fmt.Errorf(
			"load config file %q: %w",
			filename,
			err,
		)
	}

	return nil
}

func LoadConfig(envFile string) (Config, error) {
	cfg := defaultConfig()

	if err := loadEnvFile(envFile); err != nil {
		return Config{}, err
	}

	var errs []error
	var err error

	cfg.App.Env = envString("VHOME_ENV", cfg.App.Env)
	cfg.HTTP.Addr = envString("VHOME_HTTP_ADDR", cfg.HTTP.Addr)
	cfg.HTTP.ReadHeaderTimeout, err = envDuration(
		"VHOME_HTTP_READ_HEADER_TIMEOUT",
		cfg.HTTP.ReadHeaderTimeout,
	)
	if err != nil {
		errs = append(errs, err)
	}

	cfg.HTTP.ReadTimeout, err = envDuration(
		"VHOME_HTTP_READ_TIMEOUT",
		cfg.HTTP.ReadTimeout,
	)
	if err != nil {
		errs = append(errs, err)
	}

	cfg.HTTP.WriteTimeout, err = envDuration(
		"VHOME_HTTP_WRITE_TIMEOUT",
		cfg.HTTP.WriteTimeout,
	)
	if err != nil {
		errs = append(errs, err)
	}

	cfg.HTTP.IdleTimeout, err = envDuration(
		"VHOME_HTTP_IDLE_TIMEOUT",
		cfg.HTTP.IdleTimeout,
	)
	if err != nil {
		errs = append(errs, err)
	}

	cfg.HTTP.ShutdownTimeout, err = envDuration(
		"VHOME_HTTP_SHUTDOWN_TIMEOUT",
		cfg.HTTP.ShutdownTimeout,
	)
	if err != nil {
		errs = append(errs, err)
	}

	cfg.DB.Host = envString("VHOME_DB_HOST", cfg.DB.Host)
	cfg.DB.User = envString("VHOME_DB_USER", cfg.DB.User)
	cfg.DB.Password = envString("VHOME_DB_PASSWORD", cfg.DB.Password)
	cfg.DB.DBName = envString("VHOME_DB_NAME", cfg.DB.DBName)
	cfg.DB.Location = envString("VHOME_DB_LOCATION", cfg.DB.Location)

	cfg.DB.Port, err = envInt("VHOME_DB_PORT", cfg.DB.Port)
	if err != nil {
		errs = append(errs, err)
	}
	cfg.DB.MaxOpenConns, err = envInt(
		"VHOME_DB_MAX_OPEN_CONNS",
		cfg.DB.MaxOpenConns,
	)
	if err != nil {
		errs = append(errs, err)
	}
	cfg.DB.MaxIdleConns, err = envInt(
		"VHOME_DB_MAX_IDLE_CONNS",
		cfg.DB.MaxIdleConns,
	)
	if err != nil {
		errs = append(errs, err)
	}
	cfg.DB.ConnMaxLifetime, err = envDuration(
		"VHOME_DB_CONN_MAX_LIFETIME",
		cfg.DB.ConnMaxLifetime,
	)
	if err != nil {
		errs = append(errs, err)
	}

	cfg.DB.ConnMaxIdleTime, err = envDuration(
		"VHOME_DB_CONN_MAX_IDLE_TIME",
		cfg.DB.ConnMaxIdleTime,
	)
	if err != nil {
		errs = append(errs, err)
	}

	cfg.DB.ConnectTimeout, err = envDuration(
		"VHOME_DB_CONNECT_TIMEOUT",
		cfg.DB.ConnectTimeout,
	)
	if err != nil {
		errs = append(errs, err)
	}

	cfg.DB.ReadTimeout, err = envDuration(
		"VHOME_DB_READ_TIMEOUT",
		cfg.DB.ReadTimeout,
	)
	if err != nil {
		errs = append(errs, err)
	}

	cfg.DB.WriteTimeout, err = envDuration(
		"VHOME_DB_WRITE_TIMEOUT",
		cfg.DB.WriteTimeout,
	)
	if err != nil {
		errs = append(errs, err)
	}

	cfg.Auth.CookieName = envString(
		"VHOME_AUTH_COOKIE_NAME",
		cfg.Auth.CookieName,
	)

	cfg.Auth.SessionTTL, err = envDuration(
		"VHOME_AUTH_SESSION_TTL",
		cfg.Auth.SessionTTL,
	)
	if err != nil {
		errs = append(errs, err)
	}

	cfg.Auth.CookieSecure, err = envBool(
		"VHOME_AUTH_COOKIE_SECURE",
		cfg.Auth.CookieSecure,
	)
	if err != nil {
		errs = append(errs, err)
	}

	errs = append(errs, validate(cfg)...)

	if len(errs) > 0 {
		return Config{}, fmt.Errorf(
			"load config: %w",
			errors.Join(errs...),
		)
	}

	return cfg, nil
}

func validate(cfg Config) []error {
	var errs []error

	switch cfg.App.Env {
	case "development", "test", "production":
	default:
		errs = append(
			errs,
			fmt.Errorf(
				"VHOME_ENV must be development, test or production",
			),
		)
	}

	if strings.TrimSpace(cfg.HTTP.Addr) == "" {
		errs = append(errs, errors.New("VHOME_HTTP_ADDR is required"))
	}

	if strings.TrimSpace(cfg.DB.Host) == "" {
		errs = append(errs, errors.New("VHOME_DB_HOST is required"))
	}

	if cfg.DB.Port < 1 || cfg.DB.Port > 65535 {
		errs = append(
			errs,
			errors.New("VHOME_DB_PORT must be between 1 and 65535"),
		)
	}

	if cfg.DB.User == "" {
		errs = append(errs, errors.New("VHOME_DB_USER is required"))
	}

	if cfg.DB.DBName == "" {
		errs = append(errs, errors.New("VHOME_DB_NAME is required"))
	}

	if cfg.DB.MaxOpenConns <= 0 {
		errs = append(
			errs,
			errors.New("VHOME_DB_MAX_OPEN_CONNS must be greater than 0"),
		)
	}

	if cfg.DB.MaxIdleConns < 0 {
		errs = append(
			errs,
			errors.New("VHOME_DB_MAX_IDLE_CONNS cannot be negative"),
		)
	}

	if cfg.DB.MaxIdleConns > cfg.DB.MaxOpenConns {
		errs = append(
			errs,
			errors.New(
				"VHOME_DB_MAX_IDLE_CONNS cannot exceed VHOME_DB_MAX_OPEN_CONNS",
			),
		)
	}

	if cfg.DB.ConnMaxLifetime <= 0 {
		errs = append(
			errs,
			errors.New("VHOME_DB_CONN_MAX_LIFETIME must be greater than 0"),
		)
	}

	if cfg.DB.ConnectTimeout <= 0 {
		errs = append(
			errs,
			errors.New("VHOME_DB_CONNECT_TIMEOUT must be greater than 0"),
		)
	}

	if _, err := time.LoadLocation(cfg.DB.Location); err != nil {
		errs = append(
			errs,
			fmt.Errorf(
				"invalid VHOME_DB_LOCATION %q: %w",
				cfg.DB.Location,
				err,
			),
		)
	}

	if cfg.Auth.SessionTTL <= 0 {
		errs = append(
			errs,
			errors.New(
				"VHOME_AUTH_SESSION_TTL must be greater than 0",
			),
		)
	}

	if strings.TrimSpace(cfg.Auth.CookieName) == "" {
		errs = append(
			errs,
			errors.New(
				"VHOME_AUTH_COOKIE_NAME is required",
			),
		)
	}

	if cfg.App.Env == "production" && !cfg.Auth.CookieSecure {
		errs = append(
			errs,
			errors.New(
				"VHOME_AUTH_COOKIE_SECURE must be true in production",
			),
		)
	}

	return errs
}
