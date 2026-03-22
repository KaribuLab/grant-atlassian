// Package handler_test contiene tests unitarios para AtlassianHandler.
//
// Estos tests verifican:
//   - Comando desconocido retorna error.
//   - get-url genera URL correctamente.
//   - get-token intercambia código por token.
//   - Validación de argumentos requeridos.
//   - Manejo de errores del servicio.
package handler_test

import (
	"net/http"
	"testing"

	grantprovider "github.com/KaribuLab/grant-provider"
	"github.com/KaribuLab/grant-atlassian/internal/atlassian"
	"github.com/KaribuLab/grant-atlassian/internal/handler"
	"github.com/KaribuLab/grant-atlassian/internal/provider"
)

// mockExchangeFetcher implementa grantprovider.ExchangeFetcher para tests.
// Permite inyectar credenciales predefinidas sin llamadas HTTP reales.
type mockExchangeFetcher struct {
	// ClientID es el client_id que se retornará en Execute.
	ClientID string
	// ClientSecret es el client_secret que se retornará en Execute.
	ClientSecret string
	// Error es el error opcional que se retornará en Execute.
	Error error
}

// Execute implementa ExchangeFetcher retornando las credenciales configuradas.
func (m *mockExchangeFetcher) Execute(req grantprovider.ExchangeRequest) (grantprovider.ExchangeReponse, error) {
	if m.Error != nil {
		return grantprovider.ExchangeReponse{}, m.Error
	}
	return grantprovider.ExchangeReponse{
		Message: "ok",
		Data: map[string]interface{}{
			"client_id":     m.ClientID,
			"client_secret": m.ClientSecret,
		},
	}, nil
}

// newTestHandler crea un AtlassianHandler con un mockExchangeFetcher inyectado
// para ser usado en tests unitarios.
//
// Parámetros:
//   - svc: servicio Atlassian con cliente HTTP mockeado.
//   - clientID: client_id simulado que retornará el fetcher.
//   - clientSecret: client_secret simulado que retornará el fetcher.
//
// Retorna:
//   - *handler.AtlassianHandler: handler configurado para tests.
func newTestHandler(svc *atlassian.Service, clientID, clientSecret string) *handler.AtlassianHandler {
	h := handler.NewAtlassianHandler(svc)
	fetcher := &mockExchangeFetcher{
		ClientID:     clientID,
		ClientSecret: clientSecret,
	}
	h.SetCredentialsService(grantprovider.GetClientCredentialsService{
		ExchangeFetcher: fetcher,
		OTT:             "test-ott",
	})
	return h
}

// TestAtlassianHandler_Invoke_UnknownCommand verifica que comandos
// desconocidos retornan error apropiado.
func TestAtlassianHandler_Invoke_UnknownCommand(t *testing.T) {
	// Setup
	mockClient := provider.NewMockHTTPClient()
	svc := atlassian.NewService(mockClient)
	h := newTestHandler(svc, "test-client-id", "")

	cmd := grantprovider.InvokeCommand{
		Command:   "unknown-command",
		Provider:  "atlassian",
		SessionID: "test-session",
		Arguments: &[]grantprovider.CommandArgument{},
	}

	// Execute
	resp, err := h.Invoke(cmd)

	// Assert
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if resp.Result.Success {
		t.Error("Expected success=false for unknown command")
	}
	if resp.Result.Message != "unknown command: unknown-command" {
		t.Errorf("Expected 'unknown command' message, got: %s", resp.Result.Message)
	}
	if len(resp.Result.Errors) == 0 || resp.Result.Errors[0] != "unsupported command" {
		t.Errorf("Expected 'unsupported command' error, got: %v", resp.Result.Errors)
	}
}

// TestAtlassianHandler_Invoke_GetURL_Success verifica generación exitosa
// de URL de autorización.
func TestAtlassianHandler_Invoke_GetURL_Success(t *testing.T) {
	// Setup
	mockClient := provider.NewMockHTTPClient()
	svc := atlassian.NewService(mockClient)
	h := newTestHandler(svc, "test-client-id", "")

	cmd := grantprovider.InvokeCommand{
		Command:   handler.CommandGetURL,
		Provider:  "atlassian",
		SessionID: "test-session",
		Arguments: &[]grantprovider.CommandArgument{
			{Name: handler.ArgResponseType, Value: "code"},
			{Name: handler.ArgClientID, Value: "test-client-id"},
			{Name: handler.ArgRedirectURI, Value: "https://example.com/callback"},
			{Name: handler.ArgScope, Value: "read:jira write:jira"},
			{Name: handler.ArgState, Value: "random-state-123"},
		},
	}

	// Execute
	resp, err := h.Invoke(cmd)

	// Assert
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if !resp.Result.Success {
		t.Errorf("Expected success, got error: %s", resp.Result.Message)
	}
	if resp.Data == nil {
		t.Fatal("Expected data in response")
	}

	// Verificar tipo de respuesta con type assertion
	data, ok := resp.Data.(handler.GetURLData)
	if !ok {
		t.Fatalf("Expected GetURLData, got type: %T", resp.Data)
	}

	// Verificar que la URL está presente y contiene parámetros esperados
	authURL := data.AuthorizationURL
	if authURL == "" {
		t.Error("Expected non-empty authorization_url")
	}

	// Verificar provider
	if data.Provider != "atlassian" {
		t.Errorf("Expected provider 'atlassian', got: %s", data.Provider)
	}

	// Verificar parámetros clave en la URL
	expectedParams := []string{
		"client_id=test-client-id",
		"redirect_uri=https%3A%2F%2Fexample.com%2Fcallback",
		"scope=read%3Ajira+write%3Ajira",
		"state=random-state-123",
		"response_type=code",
		"prompt=consent",
	}
	for _, param := range expectedParams {
		if !contains(authURL, param) {
			t.Errorf("Expected URL to contain %s", param)
		}
	}
}

// TestAtlassianHandler_Invoke_GetURL_WithPKCE verifica generación de URL
// con parámetros PKCE.
func TestAtlassianHandler_Invoke_GetURL_WithPKCE(t *testing.T) {
	// Setup
	mockClient := provider.NewMockHTTPClient()
	svc := atlassian.NewService(mockClient)
	h := newTestHandler(svc, "test-client-id", "")

	cmd := grantprovider.InvokeCommand{
		Command:   handler.CommandGetURL,
		Provider:  "atlassian",
		SessionID: "test-session",
		Arguments: &[]grantprovider.CommandArgument{
			{Name: handler.ArgResponseType, Value: "code"},
			{Name: handler.ArgClientID, Value: "test-client-id"},
			{Name: handler.ArgRedirectURI, Value: "https://example.com/callback"},
			{Name: handler.ArgScope, Value: "read:jira offline_access"},
			{Name: handler.ArgState, Value: "random-state-123"},
			{Name: handler.ArgCodeChallenge, Value: "challenge-hash-abc123"},
			{Name: handler.ArgCodeChallengeMethod, Value: "S256"},
		},
	}

	// Execute
	resp, err := h.Invoke(cmd)

	// Assert
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if !resp.Result.Success {
		t.Errorf("Expected success, got: %v", resp.Result.Errors)
	}

	// Verificar tipo de respuesta con type assertion
	data, ok := resp.Data.(handler.GetURLData)
	if !ok {
		t.Fatalf("Expected GetURLData, got type: %T", resp.Data)
	}

	// Verificar parámetros PKCE en la URL
	authURL := data.AuthorizationURL
	if !contains(authURL, "code_challenge=challenge-hash-abc123") {
		t.Error("Expected URL to contain code_challenge")
	}
	if !contains(authURL, "code_challenge_method=S256") {
		t.Error("Expected URL to contain code_challenge_method=S256")
	}
}

// TestAtlassianHandler_Invoke_GetURL_MissingRequiredArgs verifica que
// faltan argumentos requeridos produce error de validación.
func TestAtlassianHandler_Invoke_GetURL_MissingRequiredArgs(t *testing.T) {
	// Setup
	mockClient := provider.NewMockHTTPClient()
	svc := atlassian.NewService(mockClient)
	h := newTestHandler(svc, "test-client-id", "")

	// Probar sin scope y state (argumentos requeridos según grant-provider)
	cmd := grantprovider.InvokeCommand{
		Command:   handler.CommandGetURL,
		Provider:  "atlassian",
		SessionID: "test-session",
		Arguments: &[]grantprovider.CommandArgument{
			{Name: handler.ArgResponseType, Value: "code"},
			{Name: handler.ArgClientID, Value: "test-client-id"},
			{Name: handler.ArgRedirectURI, Value: "https://example.com/callback"},
			// Falta scope y state
		},
	}

	// Execute
	resp, err := h.Invoke(cmd)

	// Assert
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if resp.Result.Success {
		t.Error("Expected validation to fail without required args")
	}
	if len(resp.Result.Errors) == 0 {
		t.Error("Expected validation errors for missing required arguments")
	}
}

// TestAtlassianHandler_Invoke_GetToken_Success verifica intercambio exitoso
// de código por token.
func TestAtlassianHandler_Invoke_GetToken_Success(t *testing.T) {
	// Setup - mock respuesta exitosa del token endpoint
	mockClient := provider.NewMockHTTPClient()
	mockClient.SetJSONResponse(`{
		"access_token": "test-access-token-123",
		"refresh_token": "test-refresh-token-456",
		"expires_in": 3600,
		"token_type": "Bearer",
		"scope": "read:jira write:jira"
	}`)

	svc := atlassian.NewService(mockClient)
	h := newTestHandler(svc, "test-client-id", "")

	cmd := grantprovider.InvokeCommand{
		Command:   handler.CommandGetToken,
		Provider:  "atlassian",
		SessionID: "test-session",
		Arguments: &[]grantprovider.CommandArgument{
			{Name: handler.ArgCode, Value: "auth-code-123"},
			{Name: handler.ArgClientID, Value: "test-client-id"},
			{Name: handler.ArgRedirectURI, Value: "https://example.com/callback"},
		},
	}

	// Execute
	resp, err := h.Invoke(cmd)

	// Assert
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if !resp.Result.Success {
		t.Errorf("Expected success, got: %v", resp.Result.Errors)
	}
	if resp.Data == nil {
		t.Fatal("Expected token data in response")
	}

	// Verificar tipo de respuesta con type assertion
	tokenData, ok := resp.Data.(*atlassian.TokenResponse)
	if !ok {
		t.Fatalf("Expected *atlassian.TokenResponse, got type: %T", resp.Data)
	}

	// Verificar contenido del token
	if tokenData.AccessToken != "test-access-token-123" {
		t.Errorf("Expected access_token 'test-access-token-123', got: %s", tokenData.AccessToken)
	}
	if tokenData.RefreshToken != "test-refresh-token-456" {
		t.Errorf("Expected refresh_token 'test-refresh-token-456', got: %s", tokenData.RefreshToken)
	}
	if tokenData.ExpiresIn != 3600 {
		t.Errorf("Expected expires_in 3600, got: %d", tokenData.ExpiresIn)
	}
	if tokenData.TokenType != "Bearer" {
		t.Errorf("Expected token_type 'Bearer', got: %s", tokenData.TokenType)
	}

	// Verificar que se hizo la petición al endpoint correcto
	if len(mockClient.RequestsCaptured) != 1 {
		t.Fatalf("Expected 1 HTTP request, got %d", len(mockClient.RequestsCaptured))
	}

	req := mockClient.RequestsCaptured[0]
	if req.URL.String() != atlassian.TokenEndpoint {
		t.Errorf("Expected request to %s, got %s", atlassian.TokenEndpoint, req.URL)
	}
	if req.Method != http.MethodPost {
		t.Errorf("Expected POST method, got %s", req.Method)
	}

	// Verificar contenido del body
	if len(mockClient.RequestBodies) != 1 {
		t.Fatal("Expected captured request body")
	}
	body := mockClient.RequestBodies[0]
	expectedFields := []string{
		`"grant_type":"authorization_code"`,
		`"code":"auth-code-123"`,
		`"client_id":"test-client-id"`,
		`"redirect_uri":"https://example.com/callback"`,
	}
	for _, field := range expectedFields {
		if !contains(body, field) {
			t.Errorf("Expected request body to contain %s", field)
		}
	}
}

// TestAtlassianHandler_Invoke_GetToken_WithPKCE verifica intercambio
// de código con PKCE (code_verifier).
func TestAtlassianHandler_Invoke_GetToken_WithPKCE(t *testing.T) {
	// Setup
	mockClient := provider.NewMockHTTPClient()
	mockClient.SetJSONResponse(`{
		"access_token": "test-access-token",
		"expires_in": 3600,
		"token_type": "Bearer"
	}`)

	svc := atlassian.NewService(mockClient)
	h := newTestHandler(svc, "test-client-id", "")

	cmd := grantprovider.InvokeCommand{
		Command:   handler.CommandGetToken,
		Provider:  "atlassian",
		SessionID: "test-session",
		Arguments: &[]grantprovider.CommandArgument{
			{Name: handler.ArgCode, Value: "auth-code-123"},
			{Name: handler.ArgClientID, Value: "test-client-id"},
			{Name: handler.ArgRedirectURI, Value: "https://example.com/callback"},
			{Name: handler.ArgCodeVerifier, Value: "my-secret-verifier"},
		},
	}

	// Execute
	resp, err := h.Invoke(cmd)

	// Assert
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if !resp.Result.Success {
		t.Errorf("Expected success, got: %v", resp.Result.Errors)
	}

	// Verificar tipo de respuesta
	tokenData, ok := resp.Data.(*atlassian.TokenResponse)
	if !ok {
		t.Fatalf("Expected *atlassian.TokenResponse, got type: %T", resp.Data)
	}
	if tokenData.AccessToken != "test-access-token" {
		t.Errorf("Expected access_token 'test-access-token', got: %s", tokenData.AccessToken)
	}

	// Verificar code_verifier en el body
	if len(mockClient.RequestBodies) != 1 {
		t.Fatal("Expected captured request body")
	}
	body := mockClient.RequestBodies[0]
	if !contains(body, `"code_verifier":"my-secret-verifier"`) {
		t.Errorf("Expected request body to contain code_verifier, got: %s", body)
	}
}

// TestAtlassianHandler_Invoke_GetToken_WithClientSecret verifica
// intercambio con client_secret para apps confidenciales.
func TestAtlassianHandler_Invoke_GetToken_WithClientSecret(t *testing.T) {
	// Setup
	mockClient := provider.NewMockHTTPClient()
	mockClient.SetJSONResponse(`{
		"access_token": "test-access-token",
		"expires_in": 3600,
		"token_type": "Bearer"
	}`)

	svc := atlassian.NewService(mockClient)
	h := newTestHandler(svc, "test-client-id", "super-secret")

	cmd := grantprovider.InvokeCommand{
		Command:   handler.CommandGetToken,
		Provider:  "atlassian",
		SessionID: "test-session",
		Arguments: &[]grantprovider.CommandArgument{
			{Name: handler.ArgCode, Value: "auth-code-123"},
			{Name: handler.ArgClientID, Value: "test-client-id"},
			{Name: handler.ArgClientSecret, Value: "super-secret"},
			{Name: handler.ArgRedirectURI, Value: "https://example.com/callback"},
		},
	}

	// Execute
	resp, err := h.Invoke(cmd)

	// Assert
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if !resp.Result.Success {
		t.Errorf("Expected success, got: %v", resp.Result.Errors)
	}

	// Verificar tipo de respuesta
	tokenData, ok := resp.Data.(*atlassian.TokenResponse)
	if !ok {
		t.Fatalf("Expected *atlassian.TokenResponse, got type: %T", resp.Data)
	}
	if tokenData.AccessToken != "test-access-token" {
		t.Errorf("Expected access_token 'test-access-token', got: %s", tokenData.AccessToken)
	}

	// Verificar client_secret en el body
	if len(mockClient.RequestBodies) != 1 {
		t.Fatal("Expected captured request body")
	}
	body := mockClient.RequestBodies[0]
	if !contains(body, `"client_secret":"super-secret"`) {
		t.Errorf("Expected request body to contain client_secret, got: %s", body)
	}
}

// TestAtlassianHandler_Invoke_GetToken_MissingCode verifica que falta
// el código produce error de validación.
func TestAtlassianHandler_Invoke_GetToken_MissingCode(t *testing.T) {
	// Setup
	mockClient := provider.NewMockHTTPClient()
	svc := atlassian.NewService(mockClient)
	h := newTestHandler(svc, "test-client-id", "")

	// Probar sin código (requerido)
	cmd := grantprovider.InvokeCommand{
		Command:   handler.CommandGetToken,
		Provider:  "atlassian",
		SessionID: "test-session",
		Arguments: &[]grantprovider.CommandArgument{
			{Name: handler.ArgClientID, Value: "test-client-id"},
			{Name: handler.ArgRedirectURI, Value: "https://example.com/callback"},
		},
	}

	// Execute
	resp, err := h.Invoke(cmd)

	// Assert
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if resp.Result.Success {
		t.Error("Expected validation to fail without code")
	}
	if len(resp.Result.Errors) == 0 {
		t.Error("Expected validation errors for missing code")
	}
}

// TestAtlassianHandler_Invoke_GetToken_APIError verifica manejo de errores
// del API de Atlassian.
func TestAtlassianHandler_Invoke_GetToken_APIError(t *testing.T) {
	// Setup - simular error 400 del API
	mockClient := provider.NewMockHTTPClient()
	mockClient.SetErrorResponse(http.StatusBadRequest, `{
		"error": "invalid_grant",
		"error_description": "The provided authorization grant is invalid"
	}`)

	svc := atlassian.NewService(mockClient)
	h := newTestHandler(svc, "test-client-id", "")

	cmd := grantprovider.InvokeCommand{
		Command:   handler.CommandGetToken,
		Provider:  "atlassian",
		SessionID: "test-session",
		Arguments: &[]grantprovider.CommandArgument{
			{Name: handler.ArgCode, Value: "invalid-code"},
			{Name: handler.ArgClientID, Value: "test-client-id"},
			{Name: handler.ArgRedirectURI, Value: "https://example.com/callback"},
		},
	}

	// Execute
	resp, err := h.Invoke(cmd)

	// Assert
	if err != nil {
		t.Errorf("Expected no error from handler, got: %v", err)
	}
	if resp.Result.Success {
		t.Error("Expected error response for API failure")
	}
	if !contains(resp.Result.Message, "failed to exchange code") {
		t.Errorf("Expected 'failed to exchange code' in message, got: %s", resp.Result.Message)
	}
}

// TestAtlassianHandler_Invoke_GetToken_NetworkError verifica manejo de
// errores de red.
func TestAtlassianHandler_Invoke_GetToken_NetworkError(t *testing.T) {
	// Setup - simular error de red
	mockClient := provider.NewMockHTTPClient()
	mockClient.Error = http.ErrHandlerTimeout

	svc := atlassian.NewService(mockClient)
	h := newTestHandler(svc, "test-client-id", "")

	cmd := grantprovider.InvokeCommand{
		Command:   handler.CommandGetToken,
		Provider:  "atlassian",
		SessionID: "test-session",
		Arguments: &[]grantprovider.CommandArgument{
			{Name: handler.ArgCode, Value: "auth-code-123"},
			{Name: handler.ArgClientID, Value: "test-client-id"},
			{Name: handler.ArgRedirectURI, Value: "https://example.com/callback"},
		},
	}

	// Execute
	resp, err := h.Invoke(cmd)

	// Assert
	if err != nil {
		t.Errorf("Expected no error from handler, got: %v", err)
	}
	if resp.Result.Success {
		t.Error("Expected error response for network failure")
	}
	if len(resp.Result.Errors) == 0 {
		t.Error("Expected error details in response")
	}
}

// TestNewAtlassianHandlerWithDefaults verifica la factory convenience.
func TestNewAtlassianHandlerWithDefaults(t *testing.T) {
	h := handler.NewAtlassianHandlerWithDefaults()
	if h == nil {
		t.Fatal("Expected non-nil handler")
	}
}

// contains es un helper para verificar substring.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
