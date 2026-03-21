// Package atlassian implementa la lógica de dominio específica para
// la integración OAuth2 con Atlassian (3LO - 3-legged OAuth).
//
// Incluye soporte para:
//   - Generación de URLs de autorización OAuth2.
//   - Intercambio de código por token de acceso.
//   - PKCE (Proof Key for Code Exchange) para flujos seguros sin secret.
package atlassian

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/KaribuLab/grant-atlassian/internal/provider"
)

const (
	// AuthorizationEndpoint es la URL de autorización OAuth2 de Atlassian.
	AuthorizationEndpoint = "https://auth.atlassian.com/authorize"

	// TokenEndpoint es la URL para intercambiar códigos por tokens.
	TokenEndpoint = "https://auth.atlassian.com/oauth/token"

	// DefaultTimeout es el timeout por defecto para peticiones HTTP.
	DefaultTimeout = 30 * time.Second
)

// Service encapsula la lógica de negocio para interactuar con
// la API OAuth2 de Atlassian. No realiza peticiones HTTP directamente,
// sino que usa un HTTPClient inyectable para facilitar el testing.
type Service struct {
	client provider.HTTPClient
}

// NewService crea un nuevo servicio Atlassian con el cliente HTTP proporcionado.
//
// Parámetros:
//   - client: implementación de HTTPClient (puede ser DefaultHTTPClient o mock).
//
// Retorna:
//   - *Service: servicio configurado listo para usar.
func NewService(client provider.HTTPClient) *Service {
	return &Service{
		client: client,
	}
}

// AuthorizationParams contiene los parámetros necesarios para construir
// la URL de autorización OAuth2 de Atlassian.
type AuthorizationParams struct {
	// ClientID es el identificador de la aplicación OAuth2 (requerido).
	ClientID string

	// RedirectURI es la URL de callback registrada (requerido).
	RedirectURI string

	// Scope es la lista de permisos solicitados, separados por espacios (requerido).
	// Para obtener refresh token, incluir "offline_access".
	Scope string

	// State es un valor único para prevenir ataques CSRF (requerido).
	State string

	// CodeChallenge es el hash S256 del code_verifier para flujo PKCE (opcional).
	CodeChallenge string

	// CodeChallengeMethod es el método usado para generar el challenge.
	// Si CodeChallenge está presente y este campo es vacío, se asume "S256".
	CodeChallengeMethod string
}

// BuildAuthorizationURL construye la URL completa para redirigir al usuario
// a la página de autorización de Atlassian.
//
// Parámetros:
//   - params: estructura con todos los parámetros de autorización.
//
// Retorna:
//   - string: URL completa lista para redireccionar al usuario.
//   - error: error si faltan parámetros requeridos.
func (s *Service) BuildAuthorizationURL(params AuthorizationParams) (string, error) {
	if err := validateAuthorizationParams(params); err != nil {
		return "", err
	}

	u, err := url.Parse(AuthorizationEndpoint)
	if err != nil {
		return "", fmt.Errorf("failed to parse authorization endpoint: %w", err)
	}

	q := u.Query()
	q.Set("client_id", params.ClientID)
	q.Set("redirect_uri", params.RedirectURI)
	q.Set("scope", params.Scope)
	q.Set("state", params.State)
	q.Set("response_type", "code")
	q.Set("prompt", "consent")

	// Soporte PKCE
	if params.CodeChallenge != "" {
		q.Set("code_challenge", params.CodeChallenge)
		method := params.CodeChallengeMethod
		if method == "" {
			method = "S256"
		}
		q.Set("code_challenge_method", method)
	}

	u.RawQuery = q.Encode()
	return u.String(), nil
}

// validateAuthorizationParams verifica que los parámetros requeridos estén presentes.
func validateAuthorizationParams(params AuthorizationParams) error {
	if strings.TrimSpace(params.ClientID) == "" {
		return fmt.Errorf("client_id is required")
	}
	if strings.TrimSpace(params.RedirectURI) == "" {
		return fmt.Errorf("redirect_uri is required")
	}
	if strings.TrimSpace(params.Scope) == "" {
		return fmt.Errorf("scope is required")
	}
	if strings.TrimSpace(params.State) == "" {
		return fmt.Errorf("state is required")
	}
	return nil
}

// TokenRequest contiene los parámetros para intercambiar un código de autorización
// por tokens de acceso.
type TokenRequest struct {
	// Code es el código de autorización recibido en el callback (requerido).
	Code string

	// ClientID es el identificador de la aplicación (requerido).
	ClientID string

	// ClientSecret es el secreto de la aplicación (requerido para apps confidenciales).
	ClientSecret string

	// RedirectURI debe coincidir con el usado en la URL de autorización (requerido).
	RedirectURI string

	// CodeVerifier es el valor original usado para generar el code_challenge en PKCE.
	// Requerido si se usó PKCE en la URL de autorización.
	CodeVerifier string
}

// TokenResponse contiene los tokens devueltos por Atlassian.
type TokenResponse struct {
	// AccessToken es el token para autenticar peticiones a la API.
	AccessToken string `json:"access_token"`

	// RefreshToken se devuelve si scope incluye "offline_access".
	RefreshToken string `json:"refresh_token,omitempty"`

	// ExpiresIn es la duración en segundos del access_token.
	ExpiresIn int `json:"expires_in"`

	// TokenType es siempre "Bearer".
	TokenType string `json:"token_type"`

	// Scope son los scopes efectivamente otorgados.
	Scope string `json:"scope,omitempty"`
}

// ExchangeCodeForToken intercambia un código de autorización por tokens de acceso.
//
// Parámetros:
//   - ctx: contexto para control de timeouts y cancelación.
//   - req: estructura con los parámetros del intercambio.
//
// Retorna:
//   - *TokenResponse: tokens obtenidos del servidor de autorización.
//   - error: error si la petición falla o el servidor retorna error.
func (s *Service) ExchangeCodeForToken(ctx context.Context, req TokenRequest) (*TokenResponse, error) {
	if err := validateTokenRequest(req); err != nil {
		return nil, err
	}

	payload := map[string]string{
		"grant_type":    "authorization_code",
		"code":          req.Code,
		"client_id":     req.ClientID,
		"redirect_uri":  req.RedirectURI,
	}

	if req.ClientSecret != "" {
		payload["client_secret"] = req.ClientSecret
	}

	if req.CodeVerifier != "" {
		payload["code_verifier"] = req.CodeVerifier
	}

	resp, err := provider.DoJSON(ctx, s.client, TokenEndpoint, payload)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token endpoint returned status %d: %s", resp.StatusCode, string(resp.Body))
	}

	tokenResp := &TokenResponse{
		AccessToken:  getString(resp.Data, "access_token"),
		RefreshToken: getString(resp.Data, "refresh_token"),
		TokenType:    getString(resp.Data, "token_type"),
		Scope:        getString(resp.Data, "scope"),
	}

	if expires, ok := resp.Data["expires_in"]; ok {
		switch v := expires.(type) {
		case float64:
			tokenResp.ExpiresIn = int(v)
		case int:
			tokenResp.ExpiresIn = v
		}
	}

	return tokenResp, nil
}

// validateTokenRequest verifica que los parámetros requeridos estén presentes.
func validateTokenRequest(req TokenRequest) error {
	if strings.TrimSpace(req.Code) == "" {
		return fmt.Errorf("code is required")
	}
	if strings.TrimSpace(req.ClientID) == "" {
		return fmt.Errorf("client_id is required")
	}
	if strings.TrimSpace(req.RedirectURI) == "" {
		return fmt.Errorf("redirect_uri is required")
	}
	return nil
}

// getString extrae un string de un mapa, retornando vacío si no existe o no es string.
func getString(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return ""
}
