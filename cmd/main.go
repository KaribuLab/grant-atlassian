// Package main es el punto de entrada del CLI grant-atlassian.
//
// Este programa implementa un provider OAuth2 para Atlassian usando
// la librería grant-provider. Funciona como invocador por stdin:
// lee un InvokeCommand en formato JSON desde os.Stdin, lo procesa
// y escribe la respuesta como JSON a os.Stdout.
//
// Uso:
//
//	echo '{"command":"get-url","provider":"atlassian","session_id":"s1","arguments":[...]}' | grant-atlassian oauth2 get-url
//	echo '{"command":"get-token","provider":"atlassian","session_id":"s1","arguments":[...]}' | grant-atlassian oauth2 get-token
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/KaribuLab/grant-atlassian/internal/handler"
	grantprovider "github.com/KaribuLab/grant-provider"
)

const (
	// ProviderName es el identificador del provider Atlassian.
	ProviderName = "atlassian"

	// ExitCodeSuccess indica ejecución exitosa.
	ExitCodeSuccess = 0

	// ExitCodeError indica error en la ejecución.
	ExitCodeError = 1
)

func main() {
	if err := execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(ExitCodeError)
	}
	os.Exit(ExitCodeSuccess)
}

func NewExchangeFetcher(command grantprovider.InvokeCommand) grantprovider.ExchangeFetcher {
	return &grantprovider.ExchangeFetcherService{
		Provider:         command.Provider,
		SessionID:        command.SessionID,
		ExchangeEndpoint: command.ExchangeEndpoint,
	}
}

// execute configura el handler y los comandos OAuth2, y ejecuta el CLI.
// Cada subcomando lee el InvokeCommand desde stdin y escribe la respuesta a stdout.
//
// Retorna:
//   - error: error si ocurre durante la ejecución del comando.
func execute() error {
	// Crear handler con dependencias por defecto
	h := handler.NewAtlassianHandlerWithDefaults()

	// Crear subcomandos OAuth2 que leen desde stdin
	tokenCmd := createGetTokenCommand(h)
	urlCmd := createGetURLCommand(h)

	// Construir mapa de comandos requeridos por grant-provider
	oauth2Commands := grantprovider.OAuth2Commands{
		handler.CommandGetToken: tokenCmd,
		handler.CommandGetURL:   urlCmd,
	}

	// Crear comando raíz OAuth2 usando grant-provider
	rootCmd, err := grantprovider.NewOAuth2Command(ProviderName, oauth2Commands)
	if err != nil {
		return fmt.Errorf("failed to create OAuth2 command: %w", err)
	}

	// Configurar versión y descripción
	rootCmd.Version = "1.0.0"
	rootCmd.Long = `Provider OAuth2 para Atlassian (3LO - 3-legged OAuth).

Este CLI funciona como invocador por stdin: recibe un InvokeCommand en formato
JSON y retorna una InvokeResponse en formato JSON.

Comandos disponibles:
  - get-url: Genera la URL de autorización para iniciar el flujo OAuth2.
  - get-token: Intercambia el código de autorización por tokens de acceso.

PKCE (Proof Key for Code Exchange) está soportado opcionalmente para
aplicaciones públicas que no pueden mantener un client_secret confidencial.

Ejemplo de uso:
  echo '{"command":"get-url","provider":"atlassian","session_id":"s1","arguments":[{"name":"response_type","value":"code"},{"name":"client_id","value":"mi-client-id"},{"name":"redirect_uri","value":"https://mi-app.com/callback"},{"name":"scope","value":"read:jira offline_access"},{"name":"state","value":"random-state-123"}]}' | grant-atlassian oauth2 get-url`

	// Ejecutar
	return rootCmd.Execute()
}

// createGetTokenCommand crea el subcomando get-token.
// Este comando lee un InvokeCommand desde stdin, lo procesa mediante
// CommandInvoker y escribe la respuesta JSON a stdout.
//
// Parámetros:
//   - h: handler que procesará el comando.
//
// Retorna:
//   - *cobra.Command: comando configurado listo para registrar.
func createGetTokenCommand(h *handler.AtlassianHandler) *cobra.Command {
	return &cobra.Command{
		Use:   handler.CommandGetToken,
		Short: "Intercambia código de autorización por tokens de acceso",
		Long: `Intercambia el código de autorización recibido en el callback
por tokens de acceso y refresh (si se solicitó offline_access).

Lee el InvokeCommand desde stdin y escribe la InvokeResponse a stdout.

Ejemplo de entrada JSON:
{
  "command": "get-token",
  "provider": "atlassian",
  "session_id": "session-123",
  "arguments": [
    {"name": "code", "value": "auth-code-abc123"},
    {"name": "client_id", "value": "mi-client-id"},
    {"name": "client_secret", "value": "mi-secret"},
    {"name": "redirect_uri", "value": "https://mi-app.com/callback"},
    {"name": "code_verifier", "value": "verifier-para-pkce"}
  ]
}`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Crear invocador con el handler
			invoker := grantprovider.NewOAuth2CommandInvoker(h, NewExchangeFetcher)

			// Leer comando desde stdin y ejecutar
			resp, err := invoker.Run(os.Stdin)
			if err != nil {
				// Si hay error de validación o ejecución, lo retornamos
				// La respuesta ya contiene los detalles del error
				if resp.Result.Success {
					// Error inesperado, no de validación
					return err
				}
			}

			// Escribir respuesta como JSON a stdout
			if err := grantprovider.ToJSON(os.Stdout, resp); err != nil {
				return fmt.Errorf("failed to serialize response: %w", err)
			}

			// Retornar exit code apropiado basado en el resultado
			if !resp.Result.Success {
				os.Exit(ExitCodeError)
			}
			return nil
		},
	}
}

// createGetURLCommand crea el subcomando get-url.
// Este comando lee un InvokeCommand desde stdin, lo procesa mediante
// CommandInvoker y escribe la respuesta JSON a stdout.
//
// Parámetros:
//   - h: handler que procesará el comando.
//
// Retorna:
//   - *cobra.Command: comando configurado listo para registrar.
func createGetURLCommand(h *handler.AtlassianHandler) *cobra.Command {
	return &cobra.Command{
		Use:   handler.CommandGetURL,
		Short: "Genera la URL de autorización OAuth2",
		Long: `Genera la URL de autorización para iniciar el flujo OAuth2 con Atlassian.

La URL redirige al usuario a Atlassian para autorizar la aplicación.
Incluye soporte opcional para PKCE (code_challenge, code_challenge_method).

Lee el InvokeCommand desde stdin y escribe la InvokeResponse a stdout.

Ejemplo de entrada JSON:
{
  "command": "get-url",
  "provider": "atlassian",
  "session_id": "session-123",
  "arguments": [
    {"name": "response_type", "value": "code"},
    {"name": "client_id", "value": "mi-client-id"},
    {"name": "redirect_uri", "value": "https://mi-app.com/callback"},
    {"name": "scope", "value": "read:jira offline_access"},
    {"name": "state", "value": "random-state-123"},
    {"name": "code_challenge", "value": "hash-s256-del-verifier"},
    {"name": "code_challenge_method", "value": "S256"}
  ]
}`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Crear invocador con el handler
			invoker := grantprovider.NewCommandInvoker(h)

			// Leer comando desde stdin y ejecutar
			resp, err := invoker.Run(os.Stdin)
			if err != nil {
				// Si hay error de validación o ejecución, lo retornamos
				// La respuesta ya contiene los detalles del error
				if resp.Result.Success {
					// Error inesperado, no de validación
					return err
				}
			}

			// Escribir respuesta como JSON a stdout
			if err := grantprovider.ToJSON(os.Stdout, resp); err != nil {
				return fmt.Errorf("failed to serialize response: %w", err)
			}

			// Retornar exit code apropiado basado en el resultado
			if !resp.Result.Success {
				os.Exit(ExitCodeError)
			}
			return nil
		},
	}
}
