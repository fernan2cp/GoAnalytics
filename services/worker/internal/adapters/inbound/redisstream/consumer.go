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
	"goanalytics/services/worker/internal/domain/rejection"
)

// streamClient limita las operaciones Redis requeridas por el consumer.
//
// Permite usar el cliente real o un doble de pruebas sin exponer comandos que
// no pertenecen al ciclo de consumo. Cada metodo devuelve el comando tipado de
// go-redis y reporta sus fallas mediante Result o Err.
type streamClient interface {
	XGroupCreateMkStream(ctx context.Context, stream string, group string, start string) *goredis.StatusCmd
	XReadGroup(ctx context.Context, args *goredis.XReadGroupArgs) *goredis.XStreamSliceCmd
	XAutoClaim(ctx context.Context, args *goredis.XAutoClaimArgs) *goredis.XAutoClaimCmd
	XPendingExt(ctx context.Context, args *goredis.XPendingExtArgs) *goredis.XPendingExtCmd
	XAck(ctx context.Context, stream string, group string, ids ...string) *goredis.IntCmd
}

// eventProcessor define las operaciones de aplicacion usadas por el adapter.
//
// Procesa eventos ya mapeados y registra rechazos de transporte durables. Las
// implementaciones devuelven error para impedir XACK cuando no hubo resultado
// persistente.
type eventProcessor interface {
	Process(ctx context.Context, events []dto.RawEvent) error
	RecordStreamRejection(ctx context.Context, input dto.StreamRejection) error
}

// Consumer consume eventos desde Redis Stream y ejecuta el worker.
//
// Contiene cliente Redis, configuracion del consumer group, caso de uso y
// logger. Es un adaptador inbound: conoce Redis y orquesta el ciclo de vida,
// pero delega reglas de negocio al ProcessEventsUseCase.
type Consumer struct {
	client                streamClient
	streamName            string
	groupName             string
	consumerName          string
	batchSize             int
	pollInterval          time.Duration
	pendingClaimEnabled   bool
	pendingMinIdle        time.Duration
	pendingClaimInterval  time.Duration
	pendingClaimBatchSize int
	maxDeliveries         int64
	lastPendingClaim      time.Time
	now                   func() time.Time
	process               eventProcessor
	logger                outbound.Logger
}

// Config contiene la configuracion del consumer Redis Stream.
//
// Agrupa nombres de stream, grupo, consumidor, tamano de batch e intervalo de
// espera entre lecturas vacias. Debe construirse desde bootstrap.
type Config struct {
	StreamName            string
	GroupName             string
	ConsumerName          string
	BatchSize             int
	PollInterval          time.Duration
	PendingClaimEnabled   bool
	PendingMinIdle        time.Duration
	PendingClaimInterval  time.Duration
	PendingClaimBatchSize int
	MaxDeliveries         int64
}

// NewConsumer crea un consumer Redis Stream.
//
// Recibe cliente Redis, configuracion, caso de uso y logger. Devuelve error si
// falta alguna dependencia o dato requerido. Normaliza batch e intervalo.
func NewConsumer(
	client streamClient,
	config Config,
	process eventProcessor,
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
	if config.PendingMinIdle <= 0 {
		config.PendingMinIdle = 60 * time.Second
	}
	if config.PendingClaimInterval <= 0 {
		config.PendingClaimInterval = 30 * time.Second
	}
	if config.PendingClaimBatchSize <= 0 {
		config.PendingClaimBatchSize = 100
	}
	if config.MaxDeliveries <= 0 {
		config.MaxDeliveries = 5
	}
	return &Consumer{
		client:                client,
		streamName:            config.StreamName,
		groupName:             config.GroupName,
		consumerName:          config.ConsumerName,
		batchSize:             config.BatchSize,
		pollInterval:          config.PollInterval,
		pendingClaimEnabled:   config.PendingClaimEnabled,
		pendingMinIdle:        config.PendingMinIdle,
		pendingClaimInterval:  config.PendingClaimInterval,
		pendingClaimBatchSize: config.PendingClaimBatchSize,
		maxDeliveries:         config.MaxDeliveries,
		now:                   time.Now,
		process:               process,
		logger:                logger,
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

		if consumer.shouldClaimPending() {
			consumer.lastPendingClaim = consumer.now()
			if err := consumer.claimPending(ctx); err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					return nil
				}
				consumer.logError(ctx, "fallo recuperacion de pendientes redis", map[string]any{"error": err.Error()})
			}
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
		consumer.processMessages(ctx, messages, nil)
	}
}

// shouldClaimPending indica si corresponde ejecutar una recuperacion.
//
// No recibe parametros y usa el reloj del consumer. Devuelve false cuando la
// capacidad esta deshabilitada o aun no vencio el intervalo configurado.
func (consumer *Consumer) shouldClaimPending() bool {
	if !consumer.pendingClaimEnabled {
		return false
	}
	return consumer.lastPendingClaim.IsZero() || consumer.now().Sub(consumer.lastPendingClaim) >= consumer.pendingClaimInterval
}

// claimPending reclama y procesa mensajes abandonados por otros consumers.
//
// Recibe contexto y recorre XAUTOCLAIM desde 0-0 hasta completar el scan.
// Devuelve error cuando Redis no puede reclamar o consultar entregas. Los
// errores individuales de persistencia se registran y dejan el mensaje
// pendiente sin detener el resto del lote reclamado.
func (consumer *Consumer) claimPending(ctx context.Context) error {
	start := "0-0"
	for {
		messages, next, err := consumer.client.XAutoClaim(ctx, &goredis.XAutoClaimArgs{
			Stream:   consumer.streamName,
			Group:    consumer.groupName,
			Consumer: consumer.consumerName,
			MinIdle:  consumer.pendingMinIdle,
			Start:    start,
			Count:    int64(consumer.pendingClaimBatchSize),
		}).Result()
		if err == goredis.Nil {
			return nil
		}
		if err != nil {
			return err
		}
		if len(messages) > 0 {
			deliveries, err := consumer.pendingDeliveryCounts(ctx, messages)
			if err != nil {
				return err
			}
			consumer.processMessages(ctx, messages, deliveries)
		}
		if next == "0-0" {
			return nil
		}
		start = next
	}
}

// pendingDeliveryCounts consulta la cantidad de entregas de mensajes reclamados.
//
// Recibe contexto y mensajes que ya pertenecen al consumer actual. Devuelve
// un mapa por ID o error cuando XPENDING no puede confirmar el contador.
func (consumer *Consumer) pendingDeliveryCounts(ctx context.Context, messages []goredis.XMessage) (map[string]int64, error) {
	deliveries := make(map[string]int64, len(messages))
	for _, message := range messages {
		pending, err := consumer.client.XPendingExt(ctx, &goredis.XPendingExtArgs{
			Stream: consumer.streamName,
			Group:  consumer.groupName,
			Start:  message.ID,
			End:    message.ID,
			Count:  1,
		}).Result()
		if err != nil {
			return nil, err
		}
		if len(pending) == 0 || pending[0].ID != message.ID {
			return nil, fmt.Errorf("mensaje pendiente %s sin contador de entregas", message.ID)
		}
		deliveries[message.ID] = pending[0].RetryCount
	}
	return deliveries, nil
}

// processMessages procesa y confirma mensajes de forma independiente.
//
// Recibe mensajes y contadores opcionales para recuperados. No devuelve error:
// registra cada fallo y conserva pendiente solo el mensaje afectado.
func (consumer *Consumer) processMessages(ctx context.Context, messages []goredis.XMessage, deliveries map[string]int64) {
	for _, message := range messages {
		deliveryCount := int64(1)
		if count, ok := deliveries[message.ID]; ok {
			deliveryCount = count
		}
		consumer.processMessage(ctx, message, deliveryCount)
	}
}

// processMessage procesa un unico mensaje y confirma solo un resultado durable.
//
// Recibe contexto, mensaje Redis y cantidad de entregas. Los mensajes agotados
// o invalidos se registran como rechazo; los validos se delegan a application.
func (consumer *Consumer) processMessage(ctx context.Context, message goredis.XMessage, deliveryCount int64) {
	if deliveryCount >= consumer.maxDeliveries {
		consumer.rejectAndAck(ctx, message, deliveryCount, rejection.ReasonStreamRetryExhausted)
		return
	}
	event, err := mapMessage(message)
	if err != nil {
		consumer.rejectAndAck(ctx, message, deliveryCount, rejection.ReasonInvalidStreamPayload)
		return
	}
	if err := consumer.process.Process(ctx, []dto.RawEvent{event}); err != nil {
		consumer.logError(ctx, "fallo procesamiento de mensaje", map[string]any{
			"stream_id": message.ID,
			"error":     err.Error(),
		})
		return
	}
	if err := consumer.ack(ctx, []string{message.ID}); err != nil {
		consumer.logError(ctx, "fallo confirmacion redis stream", map[string]any{
			"stream_id": message.ID,
			"error":     err.Error(),
		})
		return
	}
	consumer.logInfo(ctx, "mensaje procesado", map[string]any{"stream_id": message.ID})
}

// rejectAndAck persiste un rechazo de transporte antes de confirmar Redis.
//
// Recibe mensaje, entregas y motivo estable. Si application o XACK fallan,
// registra el error y conserva el mensaje pendiente para un intento posterior.
func (consumer *Consumer) rejectAndAck(ctx context.Context, message goredis.XMessage, deliveryCount int64, reason string) {
	payload := messagePayload(message)
	eventID, siteCode, environment := extractStreamIdentity(payload)
	if err := consumer.process.RecordStreamRejection(ctx, dto.StreamRejection{
		StreamID:      message.ID,
		EventID:       eventID,
		SiteCode:      siteCode,
		Environment:   environment,
		Reason:        reason,
		Payload:       payload,
		DeliveryCount: deliveryCount,
	}); err != nil {
		consumer.logError(ctx, "fallo persistencia de rechazo de stream", map[string]any{
			"stream_id": message.ID,
			"reason":    reason,
			"error":     err.Error(),
		})
		return
	}
	if err := consumer.ack(ctx, []string{message.ID}); err != nil {
		consumer.logError(ctx, "fallo confirmacion de rechazo de stream", map[string]any{
			"stream_id": message.ID,
			"reason":    reason,
			"error":     err.Error(),
		})
		return
	}
	consumer.logInfo(ctx, "mensaje rechazado y confirmado", map[string]any{
		"stream_id":      message.ID,
		"reason":         reason,
		"delivery_count": deliveryCount,
	})
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

// mapMessage convierte un mensaje Redis individual a DTO de aplicacion.
//
// Recibe XMessage y devuelve RawEvent. Devuelve error con el ID de stream si
// falta payload o el JSON no cumple el contrato de transporte.
func mapMessage(message goredis.XMessage) (dto.RawEvent, error) {
	payloadValue, ok := message.Values["payload"]
	if !ok {
		return dto.RawEvent{}, fmt.Errorf("mensaje %s sin payload", message.ID)
	}
	raw, err := decodeRawEvent(fmt.Sprint(payloadValue))
	if err != nil {
		return dto.RawEvent{}, fmt.Errorf("mensaje %s invalido: %w", message.ID, err)
	}
	return raw, nil
}

// messagePayload obtiene el payload textual disponible en el mensaje.
//
// Recibe XMessage y devuelve cadena vacia cuando el campo no existe. No
// interpreta ni valida el contenido.
func messagePayload(message goredis.XMessage) string {
	payload, ok := message.Values["payload"]
	if !ok {
		return ""
	}
	return fmt.Sprint(payload)
}

// extractStreamIdentity recupera identificadores no sensibles de un JSON.
//
// Recibe payload textual y devuelve event_id, site_code y env cuando pueden
// decodificarse como strings. Un JSON invalido produce valores vacios.
func extractStreamIdentity(payload string) (string, string, string) {
	var decoded map[string]any
	if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
		return "", "", ""
	}
	stringValue := func(key string) string {
		value, _ := decoded[key].(string)
		return value
	}
	return stringValue("event_id"), stringValue("site_code"), stringValue("env")
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
		EventID:                raw.EventID,
		LogicalEventID:         raw.LogicalEventID,
		IdempotencyKey:         raw.IdempotencyKey,
		TabID:                  raw.TabID,
		Sequence:               raw.Sequence,
		PreviousLogicalEventID: raw.PreviousLogicalEventID,
		SiteCode:               raw.SiteCode,
		Environment:            raw.Environment,
		TokenVersion:           raw.TokenVersion,
		JWTID:                  raw.JWTID,
		EventName:              raw.EventName,
		EventVersion:           normalizeEventVersion(raw.EventVersion),
		EventTime:              raw.EventTime,
		ReceivedAt:             raw.ReceivedAt,
		AnonymousID:            raw.AnonymousID,
		SessionID:              raw.SessionID,
		UserID:                 raw.UserID,
		Origin:                 raw.Origin,
		URL:                    raw.URL,
		Path:                   raw.Path,
		Referrer:               raw.Referrer,
		UserAgent:              raw.UserAgent,
		IPHash:                 raw.IPHash,
		SDKName:                raw.SDKName,
		SDKVersion:             raw.SDKVersion,
		Properties:             raw.Properties,
		Context:                raw.Context,
	}, nil
}

func normalizeEventVersion(version int) int {
	if version <= 0 {
		return 1
	}
	return version
}

type streamRawEvent struct {
	EventID                string         `json:"event_id"`
	LogicalEventID         string         `json:"logical_event_id"`
	IdempotencyKey         string         `json:"idempotency_key"`
	TabID                  string         `json:"tab_id"`
	Sequence               int64          `json:"sequence"`
	PreviousLogicalEventID string         `json:"previous_logical_event_id"`
	SiteCode               string         `json:"site_code"`
	Environment            string         `json:"env"`
	TokenVersion           int            `json:"token_version"`
	JWTID                  string         `json:"jwt_id"`
	EventName              string         `json:"event_name"`
	EventVersion           int            `json:"event_version"`
	EventTime              time.Time      `json:"event_time"`
	ReceivedAt             time.Time      `json:"received_at"`
	AnonymousID            string         `json:"anonymous_id"`
	SessionID              string         `json:"session_id"`
	UserID                 *string        `json:"user_id"`
	Origin                 string         `json:"origin"`
	URL                    string         `json:"url"`
	Path                   string         `json:"path"`
	Referrer               string         `json:"referrer"`
	UserAgent              string         `json:"user_agent"`
	IPHash                 string         `json:"ip_hash"`
	SDKName                string         `json:"sdk_name"`
	SDKVersion             string         `json:"sdk_version"`
	Properties             map[string]any `json:"properties"`
	Context                map[string]any `json:"context"`
}
