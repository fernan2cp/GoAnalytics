package bootstrap

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config contiene la configuracion completa del servicio de ingesta.
//
// Agrupa valores de HTTP, JWT, Redis, stream, rate limit y limites de payload.
// Se construye en bootstrap desde `.env` y variables de entorno para mantener
// application y domain libres de infraestructura.
type Config struct {
	AppEnv   string
	AppName  string
	LogLevel string

	HTTPHost string
	HTTPPort int

	JWTAlgorithm         string
	GoAnalyticsJWTSecret string
	JWTIssuer            string
	JWTAudience          string
	JWTExpirationMinutes int
	HideAuthFailures     bool

	RedisAddr     string
	RedisUsername string
	RedisPassword string
	RedisDB       int

	EventStreamName   string
	EventStreamMaxLen int64

	RateLimitEnabled             bool
	RateLimitEventsPerMinuteSite int
	RateLimitEventsPerMinuteIP   int

	MaxEventsPerBatch    int
	MaxEventPayloadBytes int64

	AllowedOrigins []string
}

// LoadConfig carga configuracion desde un archivo `.env` opcional y entorno.
//
// Recibe la ruta del archivo dotenv. Si el archivo no existe continua con las
// variables del proceso. Devuelve Config normalizada o error cuando algun valor
// numerico o booleano es invalido.
func LoadConfig(dotenvPath string) (Config, error) {
	if err := loadDotenv(dotenvPath); err != nil {
		return Config{}, err
	}
	config := Config{
		AppEnv:   getEnv("APP_ENV", "development"),
		AppName:  getEnv("APP_NAME", "go-analytics"),
		LogLevel: getEnv("LOG_LEVEL", "info"),

		HTTPHost: getEnv("INGEST_HTTP_HOST", "0.0.0.0"),

		JWTAlgorithm:         getEnv("JWT_ALGORITHM", "HS256"),
		GoAnalyticsJWTSecret: getEnv("GO_ANALYTICS_JWT_SECRET", ""),
		JWTIssuer:            getEnv("JWT_ISSUER", "main-backend"),
		JWTAudience:          getEnv("JWT_AUDIENCE", "analytics-ingest"),
		JWTExpirationMinutes: 30,

		RedisAddr:     getEnv("REDIS_ADDR", "localhost:6379"),
		RedisUsername: getEnv("REDIS_USERNAME", ""),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),

		EventStreamName:   getEnv("EVENT_STREAM_NAME", "goanalytics:events:raw"),
		EventStreamMaxLen: 1000000,

		RateLimitEnabled:             true,
		RateLimitEventsPerMinuteSite: 3000,
		RateLimitEventsPerMinuteIP:   1000,

		MaxEventsPerBatch:    50,
		MaxEventPayloadBytes: 65536,
	}

	allowedOriginsStr := getEnv("CORS_ALLOWED_ORIGINS", "")
	if allowedOriginsStr != "" {
		config.AllowedOrigins = strings.Split(allowedOriginsStr, ",")
		for i, origin := range config.AllowedOrigins {
			config.AllowedOrigins[i] = strings.TrimSpace(origin)
		}
	} else if config.AppEnv == "development" {
		config.AllowedOrigins = []string{"*"}
	}

	var err error
	if config.HTTPPort, err = getEnvInt("INGEST_HTTP_PORT", 8080); err != nil {
		return Config{}, err
	}
	if config.RedisDB, err = getEnvInt("REDIS_DB", 0); err != nil {
		return Config{}, err
	}
	if config.JWTExpirationMinutes, err = getEnvInt("JWT_EXPIRATION_MINUTES", config.JWTExpirationMinutes); err != nil {
		return Config{}, err
	}
	if config.EventStreamMaxLen, err = getEnvInt64("EVENT_STREAM_MAXLEN", config.EventStreamMaxLen); err != nil {
		return Config{}, err
	}
	if config.RateLimitEnabled, err = getEnvBool("RATE_LIMIT_ENABLED", config.RateLimitEnabled); err != nil {
		return Config{}, err
	}
	if config.RateLimitEventsPerMinuteSite, err = getEnvInt("RATE_LIMIT_EVENTS_PER_MINUTE_PER_SITE", config.RateLimitEventsPerMinuteSite); err != nil {
		return Config{}, err
	}
	if config.RateLimitEventsPerMinuteIP, err = getEnvInt("RATE_LIMIT_EVENTS_PER_MINUTE_PER_IP", config.RateLimitEventsPerMinuteIP); err != nil {
		return Config{}, err
	}
	if config.MaxEventsPerBatch, err = getEnvInt("MAX_EVENTS_PER_BATCH", config.MaxEventsPerBatch); err != nil {
		return Config{}, err
	}
	if config.MaxEventPayloadBytes, err = getEnvInt64("MAX_EVENT_PAYLOAD_BYTES", config.MaxEventPayloadBytes); err != nil {
		return Config{}, err
	}
	if config.HideAuthFailures, err = getEnvBool("INGEST_HIDE_AUTH_FAILURES", false); err != nil {
		return Config{}, err
	}

	if strings.ToUpper(config.JWTAlgorithm) != "HS256" {
		return Config{}, fmt.Errorf("JWT_ALGORITHM no soportado en Fase 2: %s", config.JWTAlgorithm)
	}
	return config, nil
}

// Address devuelve la direccion HTTP host:port.
//
// No recibe parametros y combina los campos HTTPHost y HTTPPort de Config.
func (config Config) Address() string {
	return fmt.Sprintf("%s:%d", config.HTTPHost, config.HTTPPort)
}

// JWTMaxLifetime devuelve la vida maxima permitida del token.
//
// No recibe parametros. Devuelve cero cuando la configuracion no define un
// valor positivo, lo que desactiva esa validacion en el adaptador.
func (config Config) JWTMaxLifetime() time.Duration {
	if config.JWTExpirationMinutes <= 0 {
		return 0
	}
	return time.Duration(config.JWTExpirationMinutes) * time.Minute
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

// getEnv obtiene una variable de entorno textual con fallback.
//
// Recibe nombre de variable y valor por defecto. Devuelve el valor del proceso
// cuando existe, o el fallback cuando esta ausente.
func getEnv(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// getEnvInt obtiene una variable entera con fallback.
//
// Recibe nombre de variable y valor por defecto. Devuelve error si la variable
// existe pero no puede convertirse a int.
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

// getEnvInt64 obtiene una variable entera de 64 bits con fallback.
//
// Recibe nombre de variable y valor por defecto. Devuelve error si la variable
// existe pero no puede convertirse a int64.
func getEnvInt64(key string, fallback int64) (int64, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s invalido: %w", key, err)
	}
	return parsed, nil
}

// getEnvBool obtiene una variable booleana con fallback.
//
// Recibe nombre de variable y valor por defecto. Acepta los formatos de
// strconv.ParseBool y devuelve error si la variable existe pero es invalida.
func getEnvBool(key string, fallback bool) (bool, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s invalido: %w", key, err)
	}
	return parsed, nil
}
