package subject

import (
	"errors"
	"testing"
)

func TestValidateIdentity(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, tenantID, externalID string
		typeValue                  Type
		wantErr                    error
	}{
		{name: "customer válido", tenantID: "tenant-1", typeValue: CustomerType, externalID: "customer-1"},
		{name: "tenant vacío", typeValue: CustomerType, externalID: "customer-1", wantErr: ErrInvalidIdentity},
		{name: "id externo vacío", tenantID: "tenant-1", typeValue: CustomerType, wantErr: ErrInvalidIdentity},
		{name: "tipo no soportado", tenantID: "tenant-1", typeValue: "account", externalID: "account-1", wantErr: ErrUnsupportedType},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if err := ValidateIdentity(test.tenantID, test.typeValue, test.externalID); !errors.Is(err, test.wantErr) {
				t.Fatalf("ValidateIdentity() error = %v, esperado %v", err, test.wantErr)
			}
		})
	}
}

func TestValidateLifecycle(t *testing.T) {
	t.Parallel()
	destinationID, sameID := int64(2), int64(1)
	tests := []struct {
		name        string
		status      LifecycleStatus
		mergeTarget *int64
		wantErr     bool
	}{
		{name: "unknown sin destino", status: LifecycleUnknown},
		{name: "active sin destino", status: LifecycleActive},
		{name: "inactive sin destino", status: LifecycleInactive},
		{name: "merged con destino", status: LifecycleMerged, mergeTarget: &destinationID},
		{name: "merged sin destino", status: LifecycleMerged, wantErr: true},
		{name: "merged consigo mismo", status: LifecycleMerged, mergeTarget: &sameID, wantErr: true},
		{name: "active con destino", status: LifecycleActive, mergeTarget: &destinationID, wantErr: true},
		{name: "estado desconocido", status: "deleted", wantErr: true},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateLifecycle(test.status, 1, test.mergeTarget)
			if (err != nil) != test.wantErr {
				t.Fatalf("ValidateLifecycle() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func TestLifecycleEligibilityOnlyAllowsActive(t *testing.T) {
	t.Parallel()

	for _, status := range []LifecycleStatus{LifecycleUnknown, LifecycleInactive, LifecycleMerged} {
		if status.IsEligible() {
			t.Fatalf("estado %q no debería ser elegible", status)
		}
	}
	if !LifecycleActive.IsEligible() {
		t.Fatal("active debería ser elegible")
	}
}
