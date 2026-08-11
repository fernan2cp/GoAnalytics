package subject

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// ContextDefinition describe una semántica inmutable y su estado en revisión.
type ContextDefinition struct {
	ID                   int64   `json:"-"`
	ContextKey           string  `json:"context_key"`
	ContextVersion       int     `json:"context_version"`
	ContextFamily        string  `json:"context_family"`
	ParentContextKey     *string `json:"parent_context_key"`
	ParentContextVersion *int    `json:"parent_context_version"`
	Status               string  `json:"status"`
}

// ContextRegistry representa una revisión completa y validada.
type ContextRegistry struct {
	ID               int64               `json:"-"`
	SchemaVersion    int                 `json:"schema_version"`
	RegistryVersion  int64               `json:"registry_version"`
	RegistryRevision string              `json:"registry_revision"`
	Contexts         []ContextDefinition `json:"contexts"`
	PayloadChecksum  string              `json:"-"`
}

var contextFamilies = map[string]bool{"search": true, "selection": true, "transaction": true, "service": true, "support": true, "content": true, "navigation": true}

// Validate verifica identidad, padres, ciclos e inmutabilidad interna.
func (registry *ContextRegistry) Validate() error {
	if registry.SchemaVersion != 1 || registry.RegistryVersion < 1 || registry.RegistryRevision == "" || len(registry.Contexts) == 0 {
		return fmt.Errorf("cabecera de registro inválida")
	}
	definitions := map[string]ContextDefinition{}
	for _, definition := range registry.Contexts {
		key := contextIdentity(definition.ContextKey, definition.ContextVersion)
		if definition.ContextKey == "" || definition.ContextVersion < 1 || !contextFamilies[definition.ContextFamily] || (definition.Status != "active" && definition.Status != "inactive") || definitions[key].ContextKey != "" {
			return fmt.Errorf("definición de contexto inválida")
		}
		if (definition.ParentContextKey == nil) != (definition.ParentContextVersion == nil) {
			return fmt.Errorf("padre incompleto")
		}
		definitions[key] = definition
	}
	for key, definition := range definitions {
		visited := map[string]bool{key: true}
		current := definition
		for current.ParentContextKey != nil {
			parentKey := contextIdentity(*current.ParentContextKey, *current.ParentContextVersion)
			parent, exists := definitions[parentKey]
			if !exists {
				return fmt.Errorf("padre inexistente")
			}
			if visited[parentKey] {
				return fmt.Errorf("ciclo de contextos")
			}
			visited[parentKey] = true
			current = parent
		}
	}
	payload, err := json.Marshal(registry)
	if err != nil {
		return err
	}
	checksum := sha256.Sum256(payload)
	registry.PayloadChecksum = hex.EncodeToString(checksum[:])
	return nil
}

func contextIdentity(key string, version int) string { return fmt.Sprintf("%s\x00%d", key, version) }
