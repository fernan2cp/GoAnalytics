package bootstrap

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"goanalytics/services/worker/internal/application/usecases"
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

	WorkerName                  string
	WorkerConsumerGroup         string
	WorkerBatchSize             int
	WorkerPollInterval          time.Duration
	WorkerPendingClaimEnabled   bool
	WorkerPendingMinIdle        time.Duration
	WorkerPendingClaimInterval  time.Duration
	WorkerPendingClaimBatchSize int
	WorkerMaxDeliveries         int64
	HealthHTTPHost              string
	HealthHTTPPort              int

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
	SemanticDedupRules    []usecases.SemanticDedupRule

	SubjectAnalyticsEnabled       bool
	SubjectBootstrapEnabled       bool
	SubjectProjectionEnabled      bool
	SubjectContextRegistryEnabled bool
	SubjectQueryAPIEnabled        bool
	SubjectSnapshotURL            string
	SubjectSourceToken            string
	SubjectSnapshotTimeout        time.Duration
	SubjectBootstrapPageSize      int
	SubjectProjectionInterval     time.Duration
	SubjectProjectionStaleAfter   time.Duration
	SubjectProjectionBatchSize    int
	SubjectProjectionVersion      int
	SubjectJobsCheckInterval      time.Duration
	SubjectJobsHealthHTTPHost     string
	SubjectJobsHealthHTTPPort     int
	SubjectQueryAPIToken          string
	SubjectQueryHTTPHost          string
	SubjectQueryHTTPPort          int
	SubjectQueryReadTimeout       time.Duration
	SubjectQueryWriteTimeout      time.Duration
	SubjectSuggestionDefaultLimit int
	SubjectSuggestionMaxLimit     int
	SubjectSuggestionContextDays  int
	SubjectSuggestionGlobalDays   int
	SubjectSuggestionStaleAfter   time.Duration
	SubjectSuggestionServeStale   bool
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

		WorkerName:                  getEnv("WORKER_NAME", "go-analytics-worker-1"),
		WorkerConsumerGroup:         getEnv("WORKER_CONSUMER_GROUP", "go-analytics-workers"),
		WorkerBatchSize:             500,
		WorkerPollInterval:          time.Second,
		WorkerPendingClaimEnabled:   true,
		WorkerPendingMinIdle:        60 * time.Second,
		WorkerPendingClaimInterval:  30 * time.Second,
		WorkerPendingClaimBatchSize: 100,
		WorkerMaxDeliveries:         5,
		HealthHTTPHost:              getEnv("WORKER_HEALTH_HTTP_HOST", "0.0.0.0"),

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

		SiteRehydrateCooldown:         300 * time.Second,
		SiteNegativeCacheTTL:          300 * time.Second,
		SiteCacheTTL:                  time.Hour,
		DeduplicationTTL:              24 * time.Hour,
		SubjectSnapshotURL:            getEnv("SUBJECT_SNAPSHOT_URL", ""),
		SubjectSourceToken:            getEnv("SUBJECT_SOURCE_TOKEN", ""),
		SubjectSnapshotTimeout:        5 * time.Second,
		SubjectBootstrapPageSize:      500,
		SubjectProjectionInterval:     24 * time.Hour,
		SubjectProjectionStaleAfter:   48 * time.Hour,
		SubjectProjectionBatchSize:    5000,
		SubjectProjectionVersion:      1,
		SubjectJobsCheckInterval:      time.Minute,
		SubjectJobsHealthHTTPHost:     getEnv("SUBJECT_JOBS_HEALTH_HTTP_HOST", "0.0.0.0"),
		SubjectQueryAPIToken:          getEnv("SUBJECT_QUERY_API_TOKEN", ""),
		SubjectQueryHTTPHost:          getEnv("SUBJECT_QUERY_HTTP_HOST", "0.0.0.0"),
		SubjectQueryReadTimeout:       3 * time.Second,
		SubjectQueryWriteTimeout:      3 * time.Second,
		SubjectSuggestionDefaultLimit: 10,
		SubjectSuggestionMaxLimit:     100,
		SubjectSuggestionContextDays:  30,
		SubjectSuggestionGlobalDays:   90,
		SubjectSuggestionStaleAfter:   48 * time.Hour,
		SubjectSuggestionServeStale:   true,
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
	if config.WorkerPendingClaimEnabled, err = getEnvBool("WORKER_PENDING_CLAIM_ENABLED", config.WorkerPendingClaimEnabled); err != nil {
		return Config{}, err
	}
	if config.WorkerPendingMinIdle, err = getEnvMillis("WORKER_PENDING_MIN_IDLE_MS", config.WorkerPendingMinIdle); err != nil {
		return Config{}, err
	}
	if config.WorkerPendingClaimInterval, err = getEnvMillis("WORKER_PENDING_CLAIM_INTERVAL_MS", config.WorkerPendingClaimInterval); err != nil {
		return Config{}, err
	}
	if config.WorkerPendingClaimBatchSize, err = getEnvInt("WORKER_PENDING_CLAIM_BATCH_SIZE", config.WorkerPendingClaimBatchSize); err != nil {
		return Config{}, err
	}
	maxDeliveries, err := getEnvInt("WORKER_MAX_DELIVERIES", int(config.WorkerMaxDeliveries))
	if err != nil {
		return Config{}, err
	}
	config.WorkerMaxDeliveries = int64(maxDeliveries)
	if config.HealthHTTPPort, err = getEnvInt("WORKER_HEALTH_HTTP_PORT", 8081); err != nil {
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
	if config.SemanticDedupRules, err = getEnvSemanticDedupRules("EVENT_SEMANTIC_DEDUP_RULES_JSON"); err != nil {
		return Config{}, err
	}
	if config.SubjectAnalyticsEnabled, err = getEnvBool("SUBJECT_ANALYTICS_ENABLED", false); err != nil {
		return Config{}, err
	}
	if config.SubjectBootstrapEnabled, err = getEnvBool("SUBJECT_BOOTSTRAP_ENABLED", false); err != nil {
		return Config{}, err
	}
	if config.SubjectProjectionEnabled, err = getEnvBool("SUBJECT_PROJECTION_ENABLED", false); err != nil {
		return Config{}, err
	}
	if config.SubjectContextRegistryEnabled, err = getEnvBool("SUBJECT_CONTEXT_REGISTRY_ENABLED", false); err != nil {
		return Config{}, err
	}
	if config.SubjectQueryAPIEnabled, err = getEnvBool("SUBJECT_QUERY_API_ENABLED", false); err != nil {
		return Config{}, err
	}
	if config.SubjectSnapshotTimeout, err = getEnvMillis("SUBJECT_SNAPSHOT_TIMEOUT_MS", config.SubjectSnapshotTimeout); err != nil {
		return Config{}, err
	}
	if config.SubjectBootstrapPageSize, err = getEnvInt("SUBJECT_BOOTSTRAP_PAGE_SIZE", config.SubjectBootstrapPageSize); err != nil {
		return Config{}, err
	}
	if config.SubjectProjectionInterval, err = getEnvSeconds("SUBJECT_PROJECTION_INTERVAL_SECONDS", config.SubjectProjectionInterval); err != nil {
		return Config{}, err
	}
	if config.SubjectProjectionStaleAfter, err = getEnvSeconds("SUBJECT_PROJECTION_STALE_AFTER_SECONDS", config.SubjectProjectionStaleAfter); err != nil {
		return Config{}, err
	}
	if config.SubjectProjectionBatchSize, err = getEnvInt("SUBJECT_PROJECTION_BATCH_SIZE", config.SubjectProjectionBatchSize); err != nil {
		return Config{}, err
	}
	if config.SubjectProjectionVersion, err = getEnvInt("SUBJECT_PROJECTION_VERSION", config.SubjectProjectionVersion); err != nil {
		return Config{}, err
	}
	if config.SubjectJobsCheckInterval, err = getEnvSeconds("SUBJECT_JOBS_CHECK_INTERVAL_SECONDS", config.SubjectJobsCheckInterval); err != nil {
		return Config{}, err
	}
	if config.SubjectJobsHealthHTTPPort, err = getEnvInt("SUBJECT_JOBS_HEALTH_HTTP_PORT", 8080); err != nil {
		return Config{}, err
	}
	if config.SubjectQueryHTTPPort, err = getEnvInt("SUBJECT_QUERY_HTTP_PORT", 8080); err != nil {
		return Config{}, err
	}
	if config.SubjectQueryReadTimeout, err = getEnvMillis("SUBJECT_QUERY_READ_TIMEOUT_MS", config.SubjectQueryReadTimeout); err != nil {
		return Config{}, err
	}
	if config.SubjectQueryWriteTimeout, err = getEnvMillis("SUBJECT_QUERY_WRITE_TIMEOUT_MS", config.SubjectQueryWriteTimeout); err != nil {
		return Config{}, err
	}
	if config.SubjectSuggestionDefaultLimit, err = getEnvInt("SUBJECT_SUGGESTION_DEFAULT_LIMIT", config.SubjectSuggestionDefaultLimit); err != nil {
		return Config{}, err
	}
	if config.SubjectSuggestionMaxLimit, err = getEnvInt("SUBJECT_SUGGESTION_MAX_LIMIT", config.SubjectSuggestionMaxLimit); err != nil {
		return Config{}, err
	}
	if config.SubjectSuggestionContextDays, err = getEnvInt("SUBJECT_SUGGESTION_CONTEXT_WINDOW_DAYS", config.SubjectSuggestionContextDays); err != nil {
		return Config{}, err
	}
	if config.SubjectSuggestionGlobalDays, err = getEnvInt("SUBJECT_SUGGESTION_GLOBAL_WINDOW_DAYS", config.SubjectSuggestionGlobalDays); err != nil {
		return Config{}, err
	}
	if config.SubjectSuggestionStaleAfter, err = getEnvSeconds("SUBJECT_SUGGESTION_STALE_AFTER_SECONDS", config.SubjectSuggestionStaleAfter); err != nil {
		return Config{}, err
	}
	if config.SubjectSuggestionServeStale, err = getEnvBool("SUBJECT_SUGGESTION_SERVE_STALE", config.SubjectSuggestionServeStale); err != nil {
		return Config{}, err
	}
	if err := config.validateSubjectFeatures(); err != nil {
		return Config{}, err
	}
	return config, nil
}

// validateSubjectFeatures valida dependencias entre capacidades opt-in.
//
// Las capacidades derivadas no pueden iniciar sin sus prerrequisitos. Esta
// validacion ocurre antes de abrir conexiones para detectar despliegues
// incompletos sin alterar el worker legacy cuando todos los flags estan
// apagados.
func (config Config) validateSubjectFeatures() error {
	if config.SubjectBootstrapEnabled && !config.SubjectAnalyticsEnabled {
		return fmt.Errorf("SUBJECT_BOOTSTRAP_ENABLED requiere SUBJECT_ANALYTICS_ENABLED")
	}
	if config.SubjectBootstrapEnabled && (strings.TrimSpace(config.SubjectSnapshotURL) == "" || strings.TrimSpace(config.SubjectSourceToken) == "") {
		return fmt.Errorf("SUBJECT_BOOTSTRAP_ENABLED requiere SUBJECT_SNAPSHOT_URL y SUBJECT_SOURCE_TOKEN")
	}
	if config.SubjectBootstrapPageSize < 1 || config.SubjectBootstrapPageSize > 500 {
		return fmt.Errorf("SUBJECT_BOOTSTRAP_PAGE_SIZE debe estar entre 1 y 500")
	}
	if config.SubjectProjectionBatchSize < 1 || config.SubjectProjectionVersion < 1 || config.SubjectProjectionInterval <= 0 || config.SubjectProjectionStaleAfter <= 0 || config.SubjectJobsCheckInterval <= 0 {
		return fmt.Errorf("configuración de proyección inválida")
	}
	if config.SubjectProjectionEnabled && !config.SubjectAnalyticsEnabled {
		return fmt.Errorf("SUBJECT_PROJECTION_ENABLED requiere SUBJECT_ANALYTICS_ENABLED")
	}
	if config.SubjectContextRegistryEnabled && !config.SubjectAnalyticsEnabled {
		return fmt.Errorf("SUBJECT_CONTEXT_REGISTRY_ENABLED requiere SUBJECT_ANALYTICS_ENABLED")
	}
	if config.SubjectQueryAPIEnabled && !config.SubjectProjectionEnabled {
		return fmt.Errorf("SUBJECT_QUERY_API_ENABLED requiere SUBJECT_PROJECTION_ENABLED")
	}
	if config.SubjectQueryAPIEnabled && strings.TrimSpace(config.SubjectQueryAPIToken) == "" {
		return fmt.Errorf("SUBJECT_QUERY_API_ENABLED requiere SUBJECT_QUERY_API_TOKEN")
	}
	if config.SubjectSuggestionDefaultLimit < 1 || config.SubjectSuggestionMaxLimit > 100 || config.SubjectSuggestionDefaultLimit > config.SubjectSuggestionMaxLimit || config.SubjectSuggestionContextDays < 1 || config.SubjectSuggestionGlobalDays < 1 || config.SubjectSuggestionStaleAfter <= 0 {
		return fmt.Errorf("configuración de sugerencias inválida")
	}
	return nil
}

// SubjectJobsHealthAddress devuelve host:port del proceso subject-jobs.
func (config Config) SubjectJobsHealthAddress() string {
	return net.JoinHostPort(config.SubjectJobsHealthHTTPHost, strconv.Itoa(config.SubjectJobsHealthHTTPPort))
}

// SubjectQueryAddress devuelve host:port del proceso query-api.
func (config Config) SubjectQueryAddress() string {
	return net.JoinHostPort(config.SubjectQueryHTTPHost, strconv.Itoa(config.SubjectQueryHTTPPort))
}

// HealthAddress devuelve la direccion HTTP operativa host:port del worker.
//
// No recibe parametros y combina los campos HealthHTTPHost y HealthHTTPPort de
// Config. Se usa solo para endpoints de health/readiness.
func (config Config) HealthAddress() string {
	return net.JoinHostPort(config.HealthHTTPHost, strconv.Itoa(config.HealthHTTPPort))
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

// getEnvBool obtiene una variable booleana con fallback.
//
// Recibe nombre y valor por defecto. Devuelve el booleano interpretado por
// strconv.ParseBool o error cuando el contenido configurado no es valido.
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

type semanticDedupRuleConfig struct {
	EventName    string   `json:"event_name"`
	WindowMillis int      `json:"window_ms"`
	Fields       []string `json:"fields"`
}

// getEnvSemanticDedupRules obtiene reglas semanticas declaradas en JSON.
//
// Recibe el nombre de la variable de entorno. Devuelve una lista vacia cuando
// no esta configurada. Las ventanas se expresan en milisegundos para permitir
// valores bajos como 100ms a 300ms.
func getEnvSemanticDedupRules(key string) ([]usecases.SemanticDedupRule, error) {
	value := os.Getenv(key)
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	var decoded []semanticDedupRuleConfig
	if err := json.Unmarshal([]byte(value), &decoded); err != nil {
		return nil, fmt.Errorf("%s invalido: %w", key, err)
	}
	rules := make([]usecases.SemanticDedupRule, 0, len(decoded))
	for _, item := range decoded {
		if strings.TrimSpace(item.EventName) == "" || item.WindowMillis <= 0 {
			continue
		}
		rules = append(rules, usecases.SemanticDedupRule{
			EventName: strings.TrimSpace(item.EventName),
			Window:    time.Duration(item.WindowMillis) * time.Millisecond,
			Fields:    item.Fields,
		})
	}
	return rules, nil
}
