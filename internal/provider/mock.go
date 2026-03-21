// Package provider proporciona utilidades de testing para mocks HTTP.
//
// Este archivo contiene MockHTTPClient que facilita los tests unitarios
// sin realizar peticiones de red reales.
package provider

import (
	"bytes"
	"context"
	"io"
	"net/http"
)

// MockHTTPClient implementa HTTPClient para tests unitarios.
// Permite configurar respuestas predefinidas y capturar peticiones realizadas.
type MockHTTPClient struct {
	// Response es la respuesta HTTP que se retornará en Do.
	Response *http.Response

	// Error es el error que se retornará en Do (tiene prioridad sobre Response).
	Error error

	// RequestsCaptured almacena todas las peticiones realizadas vía Do.
	RequestsCaptured []*http.Request

	// RequestBodies almacena los cuerpos de las peticiones como strings.
	RequestBodies []string
}

// NewMockHTTPClient crea un mock HTTP client con valores por defecto.
//
// Retorna:
//   - *MockHTTPClient: mock configurado con response 200 OK vacío.
func NewMockHTTPClient() *MockHTTPClient {
	return &MockHTTPClient{
		Response: &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader([]byte("{}"))),
			Header:     make(http.Header),
		},
		RequestsCaptured: make([]*http.Request, 0),
		RequestBodies:    make([]string, 0),
	}
}

// Do implementa HTTPClient guardando la petición y retornando el mock configurado.
//
// Parámetros:
//   - ctx: contexto (ignorado en el mock).
//   - req: petición a capturar.
//
// Retorna:
//   - *http.Response: respuesta configurada en MockHTTPClient.Response.
//   - error: error configurado en MockHTTPClient.Error.
func (m *MockHTTPClient) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	// Capturar request
	m.RequestsCaptured = append(m.RequestsCaptured, req)

	// Capturar body si existe
	if req.Body != nil {
		body, _ := io.ReadAll(req.Body)
		m.RequestBodies = append(m.RequestBodies, string(body))
		// Restaurar body para que pueda ser leído de nuevo
		req.Body = io.NopCloser(bytes.NewReader(body))
	}

	if m.Error != nil {
		return nil, m.Error
	}

	return m.Response, nil
}

// SetJSONResponse configura una respuesta HTTP 200 OK con body JSON.
//
// Parámetros:
//   - jsonBody: string JSON a retornar como cuerpo de respuesta.
func (m *MockHTTPClient) SetJSONResponse(jsonBody string) {
	m.Response = &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader([]byte(jsonBody))),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}

// SetErrorResponse configura una respuesta HTTP con código de error.
//
// Parámetros:
//   - statusCode: código HTTP de error (ej: 400, 401, 500).
//   - body: cuerpo de la respuesta de error.
func (m *MockHTTPClient) SetErrorResponse(statusCode int, body string) {
	m.Response = &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(bytes.NewReader([]byte(body))),
		Header:     make(http.Header),
	}
}
