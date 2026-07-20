package config

import (
	"log"
	"os"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/joho/godotenv"
)

type Config struct {
	Env        string  `yaml:"env" env-default:"local"`
	Storage    Storage `yaml:"storage" env-required:"true"`
	HTTPServer `yaml:"http_server"`
	Clients    ClientConfig `yaml:"clients"`
	AppSecret  string       `yaml:"app_secret" env-required:"true" env:"APP_SECRET"`
	Cache      Cache        `yaml:"cache"`
	ClickBatch ClickBatch   `yaml:"click_batch"`
}

type Storage struct {
	Host           string `yaml:"host" env-required:"true" env:"STORAGE_HOST"`
	Port           string `yaml:"port" env-required:"true" env:"STORAGE_PORT"`
	Dbname         string `yaml:"dbname" env-required:"true" env:"POSTGRES_DB"`
	User           string `yaml:"user" env-required:"true" env:"POSTGRES_USER"`
	Password       string `yaml:"password" env-required:"true" env:"POSTGRES_PASSWORD"`
	MigrationsPath string `yaml:"migrations_path" env-required:"true"`
	PoolMaxConns   int32  `yaml:"pool_max_conns" env:"POOL_MAX_CONNS" env-default:"20"`
	PoolMinConns   int32  `yaml:"pool_min_conns" env:"POOL_MIN_CONNS" env-default:"5"`
}

type HTTPServer struct {
	Address           string        `yaml:"address" env:"HTTP_SERVER_ADDRESS" env-default:":8080"`
	Timeout           time.Duration `yaml:"timeout" env:"HTTP_SERVER_TIMEOUT" env-default:"4s"`
	IdleTimeout       time.Duration `yaml:"idle_timeout" env:"HTTP_SERVER_IDLE_TIMEOUT" env-default:"60s"`
	ReadHeaderTimeout time.Duration `yaml:"read_header_timeout" env:"HTTP_SERVER_READ_HEADER_TIMEOUT" env-default:"2s"`
	User              string        `yaml:"user" env-required:"true"`
	Password          string        `yaml:"password" env-required:"true" env:"HTTP_SERVER_PASSWORD"`
}

type Client struct {
	Address      string        `yaml:"address" env:"SSO_ADDRESS"`
	Timeout      time.Duration `yaml:"timeout"`
	RetriesCount int           `yaml:"retriesCount"`
}

type ClientConfig struct {
	SSO Client `yaml:"sso"`
}

type Cache struct {
	Enabled bool          `yaml:"enabled" env:"CACHE_ENABLED" env-default:"true"`
	MaxSize int           `yaml:"max_size" env-default:"10000"`
	TTL     time.Duration `yaml:"ttl" env-default:"5m"`
}

type ClickBatch struct {
	Interval time.Duration `yaml:"interval" env:"CLICK_BATCH_INTERVAL" env-default:"10s"`
}

// MustLoad loads config or panics.
func MustLoad() *Config {
	// loads environment variables from the .env file
	if err := godotenv.Load("config.env"); err != nil {
		log.Fatal("Error loading .env file")
	}
	// get configPath from our new env
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		log.Fatal("CONFIG_PATH is not set")
	}

	// check if the file exist
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		log.Fatal("config file doesn't exist: ", configPath)
	}

	// read config from yaml
	var cfg Config
	if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
		log.Fatal("can't read config ", err)
	}

	return &cfg
}
