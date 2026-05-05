package redis

import (
	"context"
	"fmt"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

const rehydrateCooldownPrefix = "goanalytics:rehydrate:last_attempt:"

// RehydrationGate controla cooldown de rehidratacion con Redis.
//
// Contiene un cliente Redis y TTL de cooldown. Se usa desde el resolver HTTP
// para evitar llamadas repetidas al backend interno cuando falta metadata.
type RehydrationGate struct {
	client   goredis.UniversalClient
	cooldown time.Duration
}

// NewRehydrationGate crea un gate de rehidratacion.
//
// Recibe cliente Redis y duracion de cooldown. Devuelve error si falta el
// cliente. Si cooldown no es positivo, Allow siempre permite rehidratar.
func NewRehydrationGate(client goredis.UniversalClient, cooldown time.Duration) (*RehydrationGate, error) {
	if client == nil {
		return nil, fmt.Errorf("cliente redis requerido")
	}
	return &RehydrationGate{client: client, cooldown: cooldown}, nil
}

// Allow registra un intento y devuelve si puede rehidratarse ahora.
//
// Recibe contexto y site_code. Usa SET NX con TTL para aplicar cooldown.
// Devuelve ErrRehydrateCooldown cuando ya hubo un intento reciente.
func (gate *RehydrationGate) Allow(ctx context.Context, siteCode string) error {
	if gate == nil || gate.cooldown <= 0 || strings.TrimSpace(siteCode) == "" {
		return nil
	}
	ok, err := gate.client.SetNX(ctx, rehydrateCooldownKey(siteCode), "1", gate.cooldown).Result()
	if err != nil {
		return err
	}
	if !ok {
		return ErrRehydrateCooldown
	}
	return nil
}

func rehydrateCooldownKey(siteCode string) string {
	return rehydrateCooldownPrefix + strings.TrimSpace(siteCode)
}
