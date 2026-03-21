// Package handler implementa el CommandHandler de grant-provider
// para procesar comandos OAuth2 de Atlassian (get-url y get-token).
//
// Este paquete actúa como adaptador entre la interfaz genérica de
// grant-provider y el servicio específico de Atlassian.
package handler

import (
	"context"
	"fmt"

	grantprovider "github.com/KaribuLab/grant-provider"
	"github.com/KaribuLab/grant-atlassian/internal/atlassian"
	"github.com/KaribuLab/grant-atlassian/internal/provider"
)

const (
	// CommandGetURL es el nombre del comando para generar URL de autorización.
	CommandGetURL = "get-url"

	// CommandGetToken es el nombre del comando para intercambiar código por token.
	CommandGetToken = "get-token"

	// ArgResponseType es el nombre del argumento para response_type.
	// Requerido por el validador grant-provider.
	ArgResponseType = "response_type"

	// ArgClientID es el nombre del argumento para client_id.
	ArgClientID = "client_id"

	// ArgRedirectURI es el nombre del argumento para redirect_uri.
	ArgRedirectURI = "redirect_uri"

	// ArgScope es el nombre del argumento para scope.
	ArgScope = "scope"

	// ArgState es el nombre del argumento para state.
	ArgState = "state"

	// ArgCodeChallenge es el nombre del argumento para code_challenge (PKCE).
	ArgCodeChallenge = "code_challenge"

	// ArgCodeChallengeMethod es el nombre del argumento para code_challenge_method.
	ArgCodeChallengeMethod = "code_challenge_method"

	// ArgCode es el nombre del argumento para el código de autorización.
	ArgCode = "code"

	// ArgClientSecret es el nombre del argumento para client_secret.
	ArgClientSecret = "client_secret"

	// ArgCodeVerifier es el nombre del argumento para code_verifier (PKCE).
	ArgCodeVerifier = "code_verifier"
)

// AtlassianHandler implementa grantprovider.CommandHandler para procesar
// comandos OAuth2 de Atlassian. Mantiene una referencia al servicio de
// Atlassian que contiene la lógica de negocio.
type AtlassianHandler struct {
	service *atlassian.Service
}

// NewAtlassianHandler crea un nuevo handler con el servicio de Atlassian inyectado.
//
// Parámetros:
//   - service: instancia del servicio Atlassian que implementa la lógica OAuth2.
//
// Retorna:
//   - *AtlassianHandler: handler configurado y listo para usar.
func NewAtlassianHandler(service *atlassian.Service) *AtlassianHandler {
	return &AtlassianHandler{
		service: service,
	}
}

// NewAtlassianHandlerWithDefaults crea un handler con un cliente HTTP por defecto.
// Es una factory convenience para producción.
//
// Retorna:
//   - *AtlassianHandler: handler con cliente HTTP configurado con timeout de 30s.
func NewAtlassianHandlerWithDefaults() *AtlassianHandler {
	client := provider.NewDefaultHTTPClient(atlassian.DefaultTimeout)
	service := atlassian.NewService(client)
	return NewAtlassianHandler(service)
}

// Invoke procesa un comando invocado y retorna la respuesta apropiada.
// Implementa la interfaz grantprovider.CommandHandler.
//
// Parámetros:
//   - cmd: comando a ejecutar con sus argumentos.
//
// Retorna:
//   - grantprovider.InvokeResponse: respuesta exitosa o error.
//   - error: error si ocurre un problema inesperado (no de validación).
func (h *AtlassianHandler) Invoke(cmd grantprovider.InvokeCommand) (grantprovider.InvokeResponse, error) {
	// Convertir arguments a mapa para fácil acceso
	args := argumentsToMap(cmd.Arguments)

	switch cmd.Command {
	case CommandGetURL:
		return h.handleGetURL(args)
	case CommandGetToken:
		return h.handleGetToken(args)
	default:
		return grantprovider.InvokeResponse{
			Result: grantprovider.Result{
				Success: false,
				Message: fmt.Sprintf("unknown command: %s", cmd.Command),
				Errors:  []string{"unsupported command"},
			},
		}, nil
	}
}

// GetURLData contiene la respuesta del comando get-url.
// Esta estructura es exportada para permitir type assertions en tests.
type GetURLData struct {
	// AuthorizationURL es la URL completa para redirigir al usuario a Atlassian.
	AuthorizationURL string `json:"authorization_url"`

	// Provider identifica el provider OAuth2 (siempre "atlassian" para este handler).
	Provider string `json:"provider"`
}

// handleGetURL procesa el comando get-url para generar la URL de autorización.
func (h *AtlassianHandler) handleGetURL(args map[string]string) (grantprovider.InvokeResponse, error) {
	// Validar argumentos requeridos usando grant-provider
	validationErr, err := grantprovider.ValidateOAuth2GetURL(argumentsFromMap(args))
	if err != nil {
		return grantprovider.InvokeResponse{
			Result: grantprovider.Result{
				Success: false,
				Message: "validation error",
				Errors:  []string{err.Error()},
			},
		}, nil
	}

	if len(validationErr.Violations) > 0 {
		errors := make([]string, 0, len(validationErr.Violations))
		for _, v := range validationErr.Violations {
			errors = append(errors, fmt.Sprintf("%s: %s", v.Field, v.Rule))
		}
		return grantprovider.InvokeResponse{
			Result: grantprovider.Result{
				Success: false,
				Message: "missing required arguments",
				Errors:  errors,
			},
		}, nil
	}

	// Construir parámetros de autorización
	params := atlassian.AuthorizationParams{
		ClientID:            args[ArgClientID],
		RedirectURI:         args[ArgRedirectURI],
		Scope:               args[ArgScope],
		State:               args[ArgState],
		CodeChallenge:       args[ArgCodeChallenge],
		CodeChallengeMethod: args[ArgCodeChallengeMethod],
	}

	authURL, err := h.service.BuildAuthorizationURL(params)
	if err != nil {
		return grantprovider.InvokeResponse{
			Result: grantprovider.Result{
				Success: false,
				Message: "failed to build authorization URL",
				Errors:  []string{err.Error()},
			},
		}, nil
	}

	return grantprovider.InvokeResponse{
		Result: grantprovider.Result{
			Success: true,
			Message: "authorization URL generated successfully",
		},
		Data: GetURLData{
			AuthorizationURL: authURL,
			Provider:         "atlassian",
		},
	}, nil
}

// handleGetToken procesa el comando get-token para intercambiar código por token.
func (h *AtlassianHandler) handleGetToken(args map[string]string) (grantprovider.InvokeResponse, error) {
	// Validar argumentos requeridos usando grant-provider
	validationErr, err := grantprovider.ValidateOAuth2GetToken(argumentsFromMap(args))
	if err != nil {
		return grantprovider.InvokeResponse{
			Result: grantprovider.Result{
				Success: false,
				Message: "validation error",
				Errors:  []string{err.Error()},
			},
		}, nil
	}

	if len(validationErr.Violations) > 0 {
		errors := make([]string, 0, len(validationErr.Violations))
		for _, v := range validationErr.Violations {
			errors = append(errors, fmt.Sprintf("%s: %s", v.Field, v.Rule))
		}
		return grantprovider.InvokeResponse{
			Result: grantprovider.Result{
				Success: false,
				Message: "missing required arguments",
				Errors:  errors,
			},
		}, nil
	}

	// Preparar request de token
	req := atlassian.TokenRequest{
		Code:         args[ArgCode],
		ClientID:     args[ArgClientID],
		ClientSecret: args[ArgClientSecret],
		RedirectURI:  args[ArgRedirectURI],
		CodeVerifier: args[ArgCodeVerifier],
	}

	ctx, cancel := context.WithTimeout(context.Background(), atlassian.DefaultTimeout)
	defer cancel()

	tokenResp, err := h.service.ExchangeCodeForToken(ctx, req)
	if err != nil {
		return grantprovider.InvokeResponse{
			Result: grantprovider.Result{
				Success: false,
				Message: "failed to exchange code for token",
				Errors:  []string{err.Error()},
			},
		}, nil
	}

	// Usar GetAccessTokenData de grant-provider si está disponible,
	// de lo contrario usar estructura local equivalente
	return grantprovider.InvokeResponse{
		Result: grantprovider.Result{
			Success: true,
			Message: "token obtained successfully",
		},
		Data: tokenResp,
	}, nil
}

// argumentsToMap convierte un slice de CommandArgument a un mapa string-string.
func argumentsToMap(args *[]grantprovider.CommandArgument) map[string]string {
	result := make(map[string]string)
	if args == nil {
		return result
	}
	for _, arg := range *args {
		result[arg.Name] = arg.Value
	}
	return result
}

// argumentsFromMap convierte un mapa a un slice de CommandArgument.
func argumentsFromMap(m map[string]string) []grantprovider.CommandArgument {
	result := make([]grantprovider.CommandArgument, 0, len(m))
	for k, v := range m {
		result = append(result, grantprovider.CommandArgument{
			Name:  k,
			Value: v,
		})
	}
	return result
}
