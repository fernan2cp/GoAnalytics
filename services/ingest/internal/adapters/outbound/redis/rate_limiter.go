package redis

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// RateLimiter aplica limites por ventana usando contadores Redis.
//
// Contiene cliente Redis y prefijo de claves. Implementa el puerto RateLimiter
// con INCR y EXPIRE, suficiente para la ventana por minuto de Fase 2.
type RateLimiter struct {
	client redis.UniversalClient
	prefix string
}

// NewRateLimiter crea un limitador basado en Redis.
//
// Recibe cliente Redis y prefijo de claves. Devuelve error si falta cliente.
// Si el prefijo viene vacio usa `goanalytics:ratelimit`.
func NewRateLimiter(client redis.UniversalClient, prefix string) (*RateLimiter, error) {
	if client == nil {
		return nil, fmt.Errorf("cliente redis requerido")
	}
	if strings.TrimSpace(prefix) == "" {
		prefix = "goanalytics:ratelimit"
	}
	return &RateLimiter{client: client, prefix: strings.TrimRight(prefix, ":")}, nil
}

// Allow incrementa el contador de la clave y decide si queda dentro del limite.
//
// Recibe clave logica, limite numerico y ventana. Devuelve true cuando el
// contador no supera el limite. Devuelve error si Redis falla.
func (limiter *RateLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	if limit <= 0 {
		return true, nil
	}
	if window <= 0 {
		window = time.Minute
	}
	redisKey := limiter.keyFor(key, window)
	count, err := limiter.client.Incr(ctx, redisKey).Result()
	if err != nil {
		return false, err
	}
	if count == 1 {
		if err := limiter.client.Expire(ctx, redisKey, window+time.Minute).Err(); err != nil {
			return false, err
		}
	}
	return count <= int64(limit), nil
}

// keyFor arma una clave estable para la ventana actual.
//
// Recibe clave logica y ventana, y devuelve una clave Redis con bucket temporal
// en segundos Unix. Mantiene el formato `goanalytics:ratelimit:*`.
func (limiter *RateLimiter) keyFor(key string, window time.Duration) string {
	bucket := time.Now().UTC().Unix() / int64(window.Seconds())
	safeKey := strings.Trim(strings.ReplaceAll(key, " ", "_"), ":")
	return fmt.Sprintf("%s:%s:%d", limiter.prefix, safeKey, bucket)
}
