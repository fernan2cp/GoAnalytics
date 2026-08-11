package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"goanalytics/services/worker/internal/domain/subject"
)

// ContextRegistryRepository persiste revisiones activas e inválidas.
type ContextRegistryRepository struct{ pool *pgxpool.Pool }

// NewContextRegistryRepository crea el repositorio revisionado.
func NewContextRegistryRepository(pool *pgxpool.Pool) (*ContextRegistryRepository, error) {
	if pool == nil {
		return nil, fmt.Errorf("pool requerido")
	}
	return &ContextRegistryRepository{pool: pool}, nil
}

// LoadActive reconstruye la última revisión válida y sus IDs internos.
func (repository *ContextRegistryRepository) LoadActive(ctx context.Context) (subject.ContextRegistry, bool, error) {
	var registry subject.ContextRegistry
	var payload []byte
	err := repository.pool.QueryRow(ctx, `SELECT id,schema_version,registry_version,registry_revision,payload,payload_checksum FROM analytics_context_registry_revisions WHERE validation_status='valid' AND activated_at IS NOT NULL ORDER BY activated_at DESC,id DESC LIMIT 1`).Scan(&registry.ID, &registry.SchemaVersion, &registry.RegistryVersion, &registry.RegistryRevision, &payload, &registry.PayloadChecksum)
	if err == pgx.ErrNoRows {
		return registry, false, nil
	}
	if err != nil {
		return registry, false, err
	}
	if err = json.Unmarshal(payload, &registry); err != nil {
		return registry, false, err
	}
	rows, err := repository.pool.Query(ctx, `SELECT d.id,d.context_key,d.context_version FROM analytics_context_registry_entries e JOIN analytics_context_definitions d ON d.id=e.context_definition_id WHERE e.registry_revision_id=$1`, registry.ID)
	if err != nil {
		return registry, false, err
	}
	defer rows.Close()
	ids := map[string]int64{}
	for rows.Next() {
		var id int64
		var key string
		var version int
		if err = rows.Scan(&id, &key, &version); err != nil {
			return registry, false, err
		}
		ids[fmt.Sprintf("%s\x00%d", key, version)] = id
	}
	if err = rows.Err(); err != nil {
		return registry, false, err
	}
	for index := range registry.Contexts {
		registry.Contexts[index].ID = ids[fmt.Sprintf("%s\x00%d", registry.Contexts[index].ContextKey, registry.Contexts[index].ContextVersion)]
	}
	return registry, true, nil
}

// PersistValid valida checksum e inmutabilidad y activa todo atómicamente.
func (repository *ContextRegistryRepository) PersistValid(ctx context.Context, registry subject.ContextRegistry) (subject.ContextRegistry, error) {
	if err := registry.Validate(); err != nil {
		return registry, err
	}
	payload, err := json.Marshal(registry)
	if err != nil {
		return registry, err
	}
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return registry, err
	}
	defer tx.Rollback(ctx)
	var existingChecksum string
	var revisionID int64
	err = tx.QueryRow(ctx, `SELECT id,payload_checksum FROM analytics_context_registry_revisions WHERE registry_revision=$1`, registry.RegistryRevision).Scan(&revisionID, &existingChecksum)
	if err == nil && existingChecksum != registry.PayloadChecksum {
		return registry, fmt.Errorf("revisión reutilizada con checksum distinto")
	}
	if err != nil && err != pgx.ErrNoRows {
		return registry, err
	}
	if err == pgx.ErrNoRows {
		err = tx.QueryRow(ctx, `INSERT INTO analytics_context_registry_revisions(schema_version,registry_version,registry_revision,payload,payload_checksum,validation_status,loaded_at) VALUES($1,$2,$3,$4,$5,'valid',now()) RETURNING id`, registry.SchemaVersion, registry.RegistryVersion, registry.RegistryRevision, payload, registry.PayloadChecksum).Scan(&revisionID)
		if err != nil {
			return registry, err
		}
	}
	definitionIDs := map[string]int64{}
	for _, definition := range registry.Contexts {
		var id int64
		var family string
		err = tx.QueryRow(ctx, `INSERT INTO analytics_context_definitions(context_key,context_version,context_family) VALUES($1,$2,$3) ON CONFLICT(context_key,context_version) DO UPDATE SET context_key=EXCLUDED.context_key RETURNING id,context_family`, definition.ContextKey, definition.ContextVersion, definition.ContextFamily).Scan(&id, &family)
		if err != nil {
			return registry, err
		}
		if family != definition.ContextFamily {
			return registry, fmt.Errorf("definición publicada inmutable")
		}
		definitionIDs[fmt.Sprintf("%s\x00%d", definition.ContextKey, definition.ContextVersion)] = id
	}
	for _, definition := range registry.Contexts {
		id := definitionIDs[fmt.Sprintf("%s\x00%d", definition.ContextKey, definition.ContextVersion)]
		var parent any
		if definition.ParentContextKey != nil {
			parent = definitionIDs[fmt.Sprintf("%s\x00%d", *definition.ParentContextKey, *definition.ParentContextVersion)]
		}
		var current *int64
		if err = tx.QueryRow(ctx, `SELECT parent_definition_id FROM analytics_context_definitions WHERE id=$1`, id).Scan(&current); err != nil {
			return registry, err
		}
		if current != nil && parent == nil {
			return registry, fmt.Errorf("padre publicado inmutable")
		}
		if current != nil && parent != nil && *current != parent.(int64) {
			return registry, fmt.Errorf("padre publicado inmutable")
		}
		if current == nil && parent != nil {
			if _, err = tx.Exec(ctx, `UPDATE analytics_context_definitions SET parent_definition_id=$2 WHERE id=$1`, id, parent); err != nil {
				return registry, err
			}
		}
		if _, err = tx.Exec(ctx, `INSERT INTO analytics_context_registry_entries(registry_revision_id,context_definition_id,status) VALUES($1,$2,$3) ON CONFLICT(registry_revision_id,context_definition_id) DO UPDATE SET status=EXCLUDED.status`, revisionID, id, definition.Status); err != nil {
			return registry, err
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE analytics_context_registry_revisions SET activated_at=NULL WHERE validation_status='valid' AND activated_at IS NOT NULL AND id<>$1`, revisionID); err != nil {
		return registry, err
	}
	if _, err = tx.Exec(ctx, `UPDATE analytics_context_registry_revisions SET activated_at=COALESCE(activated_at,now()) WHERE id=$1`, revisionID); err != nil {
		return registry, err
	}
	if err = tx.Commit(ctx); err != nil {
		return registry, err
	}
	registry.ID = revisionID
	for index := range registry.Contexts {
		registry.Contexts[index].ID = definitionIDs[fmt.Sprintf("%s\x00%d", registry.Contexts[index].ContextKey, registry.Contexts[index].ContextVersion)]
	}
	return registry, nil
}

// RecordInvalid conserva diagnóstico sin activar la revisión.
func (repository *ContextRegistryRepository) RecordInvalid(ctx context.Context, registry subject.ContextRegistry, cause string) error {
	if registry.RegistryRevision == "" || registry.SchemaVersion != 1 || registry.RegistryVersion < 1 {
		return nil
	}
	payload, _ := json.Marshal(registry)
	_, err := repository.pool.Exec(ctx, `INSERT INTO analytics_context_registry_revisions(schema_version,registry_version,registry_revision,payload,payload_checksum,validation_status,loaded_at) VALUES($1,$2,$3,$4,$5,'invalid',now()) ON CONFLICT(registry_revision) DO NOTHING`, registry.SchemaVersion, registry.RegistryVersion, registry.RegistryRevision, payload, registry.PayloadChecksum)
	_ = cause
	return err
}
