package subject

import "testing"

func TestContextRegistryValidateChecksumAndParents(t *testing.T) {
	key := "root"
	version := 1
	registry := ContextRegistry{SchemaVersion: 1, RegistryVersion: 1, RegistryRevision: "r1", Contexts: []ContextDefinition{{ContextKey: "root", ContextVersion: 1, ContextFamily: "search", Status: "active"}, {ContextKey: "child", ContextVersion: 1, ContextFamily: "selection", ParentContextKey: &key, ParentContextVersion: &version, Status: "active"}}}
	if err := registry.Validate(); err != nil {
		t.Fatal(err)
	}
	if len(registry.PayloadChecksum) != 64 {
		t.Fatalf("checksum=%q", registry.PayloadChecksum)
	}
}
func TestContextRegistryRejectsCycle(t *testing.T) {
	a, b := "a", "b"
	version := 1
	registry := ContextRegistry{SchemaVersion: 1, RegistryVersion: 1, RegistryRevision: "r1", Contexts: []ContextDefinition{{ContextKey: "a", ContextVersion: 1, ContextFamily: "search", ParentContextKey: &b, ParentContextVersion: &version, Status: "active"}, {ContextKey: "b", ContextVersion: 1, ContextFamily: "search", ParentContextKey: &a, ParentContextVersion: &version, Status: "active"}}}
	if err := registry.Validate(); err == nil {
		t.Fatal("debería rechazar ciclo")
	}
}
