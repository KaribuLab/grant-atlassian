// Package provider proporciona la capa de transporte HTTP desacoplada
// para interactuar con APIs externas. Define interfaces que permiten
// inyectar mocks en tests unitarios.
package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HTTPClient define la interfaz para realizar peticiones HTTP.
// Esta abstracción permite inyectar implementaciones reales o mocks
// para testing.
type HTTPClient interface {
	// Do ejecuta una petición HTTP y retorna la respuesta.
	// El contexto permite controlar timeouts y cancelación.
	Do(ctx context.Context, req *http.Request) (*http.Response, error)
}

// DefaultHTTPClient implementa HTTPClient usando net/http.
// Es la implementación de producción que realiza peticiones reales.
type DefaultHTTPClient struct {
	client *http.Client
}

// NewDefaultHTTPClient crea un cliente HTTP con timeout configurado.
//
// Parámetros:
//   - timeout: duración máxima para cada petición HTTP.
//
// Retorna:
//   - *DefaultHTTPClient: cliente HTTP configurado.
func NewDefaultHTTPClient(timeout time.Duration) *DefaultHTTPClient {
	return &DefaultHTTPClient{
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// Do ejecuta la petición HTTP usando el cliente subyacente.
//
// Parámetros:
//   - ctx: contexto para control de cancelación y timeouts.
//   - req: petición HTTP a ejecutar.
//
// Retorna:
//   - *http.Response: respuesta HTTP (requiere cerrar el body por el llamador).
//   - error: error si ocurre durante la petición.
func (c *DefaultHTTPClient) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	return c.client.Do(req)
}

// JSONResponse contiene una respuesta HTTP deserializada como JSON.
type JSONResponse struct {
	StatusCode int
	Body       []byte
	Data       map[string]interface{}
}

// DoJSON realiza una petición HTTP POST con body JSON y deserializa la respuesta.
//
// Parámetros:
//   - ctx: contexto para timeouts y cancelación.
//   - client: cliente HTTP a usar.
//   - url: endpoint destino.
//   - payload: estructura a serializar como JSON en el body.
//
// Retorna:
//   - *JSONResponse: respuesta con datos deserializados.
//   - error: error si ocurre en la petición o al parsear JSON.
func DoJSON(ctx context.Context, client HTTPClient, url string, payload interface{}) (*JSONResponse, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var data map[string]interface{}
	if len(respBody) > 0 {
		if err := json.Unmarshal(respBody, &data); err != nil {
			// No retornamos error si el JSON es inválido, solo dejamos Data vacío
			data = make(map[string]interface{})
		}
	}

	return &JSONResponse{
		StatusCode: resp.StatusCode,
		Body:       respBody,
		Data:       data,
	}, nil
}
