package bootstrap

import (
	"bufio"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config contiene la configuracion completa del worker.
//
// Agrupa Redis, PostgreSQL, resolver interno, cache y politica de batches. Se
// construye en bootstrap desde `.env` y variables de entorno para mantener
// application y domain libres de infraestructura.
type Config struct {
	AppEnv   string
	AppName  string
	LogLevel string

	WorkerName          string
	WorkerConsumerGroup string
	WorkerBatchSize     int
	WorkerPollInterval  time.Duration

	RedisAddr     string
	RedisUsername string
	RedisPassword string
	RedisDB       int

	EventStreamName string

	PostgresHost     string
	PostgresPort     int
	PostgresUser     string
	PostgresPassword string
	PostgresDB       string
	PostgresSSLMode  string

	SiteResolverURL     string
	SiteResolverToken   string
	SiteResolverTimeout time.Duration

	SiteRehydrateCooldown time.Duration
	SiteNegativeCacheTTL  time.Duration
	SiteCacheTTL          time.Duration
	DeduplicationTTL      time.Duration
}

// LoadConfig carga configuracion del worker desde dotenv opcional y entorno.
//
// Recibe ruta del archivo dotenv. Si el archivo no existe continua con el
// entorno del proceso. Devuelve Config normalizada o error si algun valor
// numerico, booleano o duracion es invalido.
func LoadConfig(dotenvPath string) (Config, error) {
	if err := loadDotenv(dotenvPath); err != nil {
		return Config{}, err
	}
	config := Config{
		AppEnv:   getEnv("APP_ENV", "development"),
		AppName:  getEnv("APP_NAME", "go-analytics"),
		LogLevel: getEnv("LOG_LEVEL", "info"),

		WorkerName:          getEnv("WORKER_NAME", "go-analytics-worker-1"),
		WorkerConsumerGroup: getEnv("WORKER_CONSUMER_GROUP", "go-analytics-workers"),
		WorkerBatchSize:     500,
		WorkerPollInterval:  time.Second,

		RedisAddr:     getEnv("REDIS_ADDR", "localhost:6379"),
		RedisUsername: getEnv("REDIS_USERNAME", ""),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),

		EventStreamName: getEnv("EVENT_STREAM_NAME", "goanalytics:events:raw"),

		PostgresHost:     getEnv("POSTGRES_HOST", "localhost"),
		PostgresUser:     getEnv("POSTGRES_USER", "analytics"),
		PostgresPassword: getEnv("POSTGRES_PASSWORD", "analytics"),
		PostgresDB:       getEnv("POSTGRES_DB", "analytics"),
		PostgresSSLMode:  getEnv("POSTGRES_SSLMODE", "disable"),

		SiteResolverURL:     getEnv("SITE_RESOLVER_URL", ""),
		SiteResolverToken:   getEnv("SITE_RESOLVER_TOKEN", ""),
		SiteResolverTimeout: 300 * time.Millisecond,

		SiteRehydrateCooldown: 300 * time.Second,
		SiteNegativeCacheTTL:  300 * time.Second,
		SiteCacheTTL:          time.Hour,
		DeduplicationTTL:      24 * time.Hour,
	}

	var err error
	if config.WorkerBatchSize, err = getEnvInt("WORKER_BATCH_SIZE", config.WorkerBatchSize); err != nil {
		return Config{}, err
	}
	if config.RedisDB, err = getEnvInt("REDIS_DB", 0); err != nil {
		return Config{}, err
	}
	if config.PostgresPort, err = getEnvInt("POSTGRES_PORT", 5432); err != nil {
		return Config{}, err
	}
	if config.WorkerPollInterval, err = getEnvMillis("WORKER_POLL_INTERVAL_MS", config.WorkerPollInterval); err != nil {
		return Config{}, err
	}
	if config.SiteResolverTimeout, err = getEnvMillis("SITE_RESOLVER_TIMEOUT_MS", config.SiteResolverTimeout); err != nil {
		return Config{}, err
	}
	if config.SiteRehydrateCooldown, err = getEnvSeconds("SITE_REHYDRATE_COOLDOWN_SECONDS", config.SiteRehydrateCooldown); err != nil {
		return Config{}, err
	}
	if config.SiteNegativeCacheTTL, err = getEnvSeconds("SITE_NEGATIVE_CACHE_TTL_SECONDS", config.SiteNegativeCacheTTL); err != nil {
		return Config{}, err
	}
	if config.SiteCacheTTL, err = getEnvSeconds("SITE_CACHE_TTL_SECONDS", config.SiteCacheTTL); err != nil {
		return Config{}, err
	}
	if config.DeduplicationTTL, err = getEnvSeconds("EVENT_DEDUP_TTL_SECONDS", config.DeduplicationTTL); err != nil {
		return Config{}, err
	}
	return config, nil
}

// PostgresDSN devuelve la cadena de conexion para pgxpool.
//
// No recibe parametros. Devuelve un DSN URL con credenciales escapadas y
// sslmode configurado por entorno.
func (config Config) PostgresDSN() string {
	user := url.QueryEscape(config.PostgresUser)
	password := url.QueryEscape(config.PostgresPassword)
	host := net.JoinHostPort(config.PostgresHost, strconv.Itoa(config.PostgresPort))
	db := strings.TrimLeft(url.PathEscape(config.PostgresDB), "/")
	sslMode := url.QueryEscape(config.PostgresSSLMode)
	return fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=%s", user, password, host, db, sslMode)
}

// loadDotenv carga pares clave-valor simples desde un archivo.
//
// Recibe ruta de archivo. Si no existe no devuelve error. No sobrescribe
// variables ya presentes en el proceso para permitir configuracion externa.
func loadDotenv(path string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if key != "" && os.Getenv(key) == "" {
			_ = os.Setenv(key, value)
		}
	}
	return scanner.Err()
}

func getEnv(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) (int, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s invalido: %w", key, err)
	}
	return parsed, nil
}

func getEnvMillis(key string, fallback time.Duration) (time.Duration, error) {
	value, err := getEnvInt(key, int(fallback/time.Millisecond))
	if err != nil {
		return 0, err
	}
	return time.Duration(value) * time.Millisecond, nil
}

func getEnvSeconds(key string, fallback time.Duration) (time.Duration, error) {
	value, err := getEnvInt(key, int(fallback/time.Second))
	if err != nil {
		return 0, err
	}
	return time.Duration(value) * time.Second, nil
}
