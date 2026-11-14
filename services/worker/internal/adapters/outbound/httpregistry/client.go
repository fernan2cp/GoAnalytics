package httpregistry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"goanalytics/services/worker/internal/domain/subject"
)

// Client obtiene el registro completo con Bearer dedicado.
type Client struct {
	url, token string
	http       *http.Client
}

// NewClient crea el cliente de fuente con timeout acotado.
func NewClient(url, token string, timeout time.Duration) (*Client, error) {
	if strings.TrimSpace(url) == "" || strings.TrimSpace(token) == "" || timeout <= 0 {
		return nil, fmt.Errorf("configuración de registro incompleta")
	}
	return &Client{url: url, token: token, http: &http.Client{Timeout: timeout}}, nil
}

// Fetch decodifica el contrato estricto y calcula checksum en dominio.
func (client *Client) Fetch(ctx context.Context) (subject.ContextRegistry, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, client.url, nil)
	if err != nil {
		return subject.ContextRegistry{}, err
	}
	request.Header.Set("Authorization", "Bearer "+client.token)
	response, err := client.http.Do(request)
	if err != nil {
		return subject.ContextRegistry{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return subject.ContextRegistry{}, fmt.Errorf("registro respondió estado %d", response.StatusCode)
	}
	var registry subject.ContextRegistry
	decoder := json.NewDecoder(response.Body)
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&registry); err != nil {
		return registry, err
	}
	return registry, nil
}
