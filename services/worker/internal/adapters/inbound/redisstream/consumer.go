package redisstream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"goanalytics/services/worker/internal/application/dto"
	"goanalytics/services/worker/internal/application/ports/outbound"
	"goanalytics/services/worker/internal/application/usecases"
)

// Consumer consume eventos desde Redis Stream y ejecuta el worker.
//
// Contiene cliente Redis, configuracion del consumer group, caso de uso y
// logger. Es un adaptador inbound: conoce Redis y orquesta el ciclo de vida,
// pero delega reglas de negocio al ProcessEventsUseCase.
type Consumer struct {
	client       goredis.UniversalClient
	streamName   string
	groupName    string
	consumerName string
	batchSize    int
	pollInterval time.Duration
	process      *usecases.ProcessEventsUseCase
	logger       outbound.Logger
}

// Config contiene la configuracion del consumer Redis Stream.
//
// Agrupa nombres de stream, grupo, consumidor, tamano de batch e intervalo de
// espera entre lecturas vacias. Debe construirse desde bootstrap.
type Config struct {
	StreamName   string
	GroupName    string
	ConsumerName string
	BatchSize    int
	PollInterval time.Duration
}

// NewConsumer crea un consumer Redis Stream.
//
// Recibe cliente Redis, configuracion, caso de uso y logger. Devuelve error si
// falta alguna dependencia o dato requerido. Normaliza batch e intervalo.
func NewConsumer(
	client goredis.UniversalClient,
	config Config,
	process *usecases.ProcessEventsUseCase,
	logger outbound.Logger,
) (*Consumer, error) {
	if client == nil {
		return nil, fmt.Errorf("cliente redis requerido")
	}
	if strings.TrimSpace(config.StreamName) == "" {
		return nil, fmt.Errorf("stream redis requerido")
	}
	if strings.TrimSpace(config.GroupName) == "" {
		return nil, fmt.Errorf("consumer group requerido")
	}
	if strings.TrimSpace(config.ConsumerName) == "" {
		return nil, fmt.Errorf("consumer name requerido")
	}
	if process == nil {
		return nil, fmt.Errorf("caso de uso requerido")
	}
	if config.BatchSize <= 0 {
		config.BatchSize = 100
	}
	if config.PollInterval <= 0 {
		config.PollInterval = time.Second
	}
	return &Consumer{
		client:       client,
		streamName:   config.StreamName,
		groupName:    config.GroupName,
		consumerName: config.ConsumerName,
		batchSize:    config.BatchSize,
		pollInterval: config.PollInterval,
		process:      process,
		logger:       logger,
	}, nil
}

// Run inicia el bucle de consumo del worker.
//
// Recibe contexto de ciclo de vida. Crea el consumer group si falta, lee por
// batches, procesa eventos y confirma con XACK solo despues de persistencia
// exitosa. Devuelve nil cuando el contexto se cancela.
func (consumer *Consumer) Run(ctx context.Context) error {
	if err := consumer.ensureGroup(ctx); err != nil {
		return err
	}
	consumer.logInfo(ctx, "worker redis stream iniciado", map[string]any{
		"stream": consumer.streamName,
		"group":  consumer.groupName,
		"name":   consumer.consumerName,
	})
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		messages, err := consumer.read(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil
			}
			consumer.logError(ctx, "fallo lectura redis stream", map[string]any{"error": err.Error()})
			time.Sleep(consumer.pollInterval)
			continue
		}
		if len(messages) == 0 {
			continue
		}

		events, ids, err := mapMessages(messages)
		if err != nil {
			consumer.logError(ctx, "fallo mapeo de eventos crudos", map[string]any{"error": err.Error()})
			continue
		}
		if err := consumer.process.Process(ctx, events); err != nil {
			consumer.logError(ctx, "fallo procesamiento de batch", map[string]any{
				"error": err.Error(),
				"count": len(events),
			})
			continue
		}
		if err := consumer.ack(ctx, ids); err != nil {
			consumer.logError(ctx, "fallo confirmacion redis stream", map[string]any{"error": err.Error()})
			continue
		}
		consumer.logInfo(ctx, "batch procesado", map[string]any{"count": len(events)})
	}
}

// ensureGroup crea el consumer group si todavia no existe.
//
// Recibe contexto y devuelve error si Redis falla por una causa distinta a que
// el grupo ya exista.
func (consumer *Consumer) ensureGroup(ctx context.Context) error {
	err := consumer.client.XGroupCreateMkStream(ctx, consumer.streamName, consumer.groupName, "$").Err()
	if err == nil || strings.Contains(err.Error(), "BUSYGROUP") {
		return nil
	}
	return err
}

// read obtiene un batch desde Redis Stream.
//
// Recibe contexto y devuelve mensajes Redis sin mapear. Usa `>` para recibir
// solo mensajes nuevos del consumer group.
func (consumer *Consumer) read(ctx context.Context) ([]goredis.XMessage, error) {
	streams, err := consumer.client.XReadGroup(ctx, &goredis.XReadGroupArgs{
		Group:    consumer.groupName,
		Consumer: consumer.consumerName,
		Streams:  []string{consumer.streamName, ">"},
		Count:    int64(consumer.batchSize),
		Block:    consumer.pollInterval,
	}).Result()
	if err == goredis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(streams) == 0 {
		return nil, nil
	}
	return streams[0].Messages, nil
}

// ack confirma mensajes procesados en Redis Stream.
//
// Recibe contexto e IDs de stream. Devuelve error si Redis rechaza XACK.
func (consumer *Consumer) ack(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return consumer.client.XAck(ctx, consumer.streamName, consumer.groupName, ids...).Err()
}

func (consumer *Consumer) logInfo(ctx context.Context, message string, attrs map[string]any) {
	if consumer.logger != nil {
		consumer.logger.Info(ctx, message, attrs)
	}
}

func (consumer *Consumer) logError(ctx context.Context, message string, attrs map[string]any) {
	if consumer.logger != nil {
		consumer.logger.Error(ctx, message, attrs)
	}
}

// mapMessages convierte mensajes Redis a DTOs de aplicacion.
//
// Recibe mensajes XREADGROUP y devuelve eventos crudos junto con sus IDs de
// stream. Devuelve error si falta payload o si el JSON no cumple el contrato.
func mapMessages(messages []goredis.XMessage) ([]dto.RawEvent, []string, error) {
	events := make([]dto.RawEvent, 0, len(messages))
	ids := make([]string, 0, len(messages))
	for _, message := range messages {
		payloadValue, ok := message.Values["payload"]
		if !ok {
			return nil, nil, fmt.Errorf("mensaje %s sin payload", message.ID)
		}
		raw, err := decodeRawEvent(fmt.Sprint(payloadValue))
		if err != nil {
			return nil, nil, fmt.Errorf("mensaje %s invalido: %w", message.ID, err)
		}
		events = append(events, raw)
		ids = append(ids, message.ID)
	}
	return events, ids, nil
}

// decodeRawEvent parsea el JSON publicado por la ingesta.
//
// Recibe payload JSON y devuelve dto.RawEvent. Usa una estructura con tags
// explicitos porque el DTO de application no expone detalles de transporte.
func decodeRawEvent(payload string) (dto.RawEvent, error) {
	var raw streamRawEvent
	if err := json.Unmarshal([]byte(payload), &raw); err != nil {
		return dto.RawEvent{}, err
	}
	return dto.RawEvent{
		EventID:      raw.EventID,
		SiteCode:     raw.SiteCode,
		Environment:  raw.Environment,
		TokenVersion: raw.TokenVersion,
		JWTID:        raw.JWTID,
		EventName:    raw.EventName,
		EventVersion: normalizeEventVersion(raw.EventVersion),
		EventTime:    raw.EventTime,
		ReceivedAt:   raw.ReceivedAt,
		AnonymousID:  raw.AnonymousID,
		SessionID:    raw.SessionID,
		UserID:       raw.UserID,
		Origin:       raw.Origin,
		URL:          raw.URL,
		Path:         raw.Path,
		Referrer:     raw.Referrer,
		UserAgent:    raw.UserAgent,
		IPHash:       raw.IPHash,
		SDKName:      raw.SDKName,
		SDKVersion:   raw.SDKVersion,
		Properties:   raw.Properties,
		Context:      raw.Context,
	}, nil
}

func normalizeEventVersion(version int) int {
	if version <= 0 {
		return 1
	}
	return version
}

type streamRawEvent struct {
	EventID      string         `json:"event_id"`
	SiteCode     string         `json:"site_code"`
	Environment  string         `json:"env"`
	TokenVersion int            `json:"token_version"`
	JWTID        string         `json:"jwt_id"`
	EventName    string         `json:"event_name"`
	EventVersion int            `json:"event_version"`
	EventTime    time.Time      `json:"event_time"`
	ReceivedAt   time.Time      `json:"received_at"`
	AnonymousID  string         `json:"anonymous_id"`
	SessionID    string         `json:"session_id"`
	UserID       *string        `json:"user_id"`
	Origin       string         `json:"origin"`
	URL          string         `json:"url"`
	Path         string         `json:"path"`
	Referrer     string         `json:"referrer"`
	UserAgent    string         `json:"user_agent"`
	IPHash       string         `json:"ip_hash"`
	SDKName      string         `json:"sdk_name"`
	SDKVersion   string         `json:"sdk_version"`
	Properties   map[string]any `json:"properties"`
	Context      map[string]any `json:"context"`
}
