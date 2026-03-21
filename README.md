# grant-atlassian

Provider OAuth2 para Atlassian implementado en Go usando la librería [grant-provider](https://github.com/KaribuLab/grant-provider).

Este CLI implementa el flujo OAuth2 3LO (3-legged OAuth) de Atlassian como invocador por stdin. Lee comandos en formato JSON desde stdin y retorna respuestas en formato JSON a stdout.

## Características

- **Invocador por stdin**: Lee `InvokeCommand` JSON desde stdin, escribe `InvokeResponse` JSON a stdout.
- **Comandos OAuth2**: `get-url` y `get-token` para flujo completo de autorización.
- **Soporte PKCE**: Para aplicaciones públicas que no pueden mantener un `client_secret` confidencial.
- **Validación integrada**: Usa los validadores `ValidateOAuth2GetURL` y `ValidateOAuth2GetToken` de grant-provider.

## Instalación

### Desde código fuente

```bash
# Clonar el repositorio
git clone https://github.com/KaribuLab/grant-atlassian.git
cd grant-atlassian

# Compilar para Linux
task build:linux

# O compilar para el sistema actual
task build

# El binario queda en bin/grant-atlassian
```

### Requisitos

- Go 1.25 o superior
- [Task](https://taskfile.dev/installation/) (opcional, para usar Taskfile.yaml)

## Uso

Este CLI funciona como **invocador por stdin**: recibe un `InvokeCommand` en formato JSON y retorna una `InvokeResponse` en formato JSON.

### Formato de entrada

```json
{
  "command": "nombre-del-comando",
  "provider": "atlassian",
  "session_id": "id-de-sesion",
  "arguments": [
    {"name": "nombre-arg", "value": "valor-arg"}
  ]
}
```

### Comando get-url

Genera la URL de autorización OAuth2:

```bash
echo '{
  "command": "get-url",
  "provider": "atlassian",
  "session_id": "session-001",
  "arguments": [
    {"name": "response_type", "value": "code"},
    {"name": "client_id", "value": "TU_CLIENT_ID"},
    {"name": "redirect_uri", "value": "https://tu-app.com/callback"},
    {"name": "scope", "value": "read:jira write:jira offline_access"},
    {"name": "state", "value": "random-state-123"}
  ]
}' | grant-atlassian oauth2 get-url
```

Salida JSON:

```json
{
  "result": {
    "success": true,
    "message": "authorization URL generated successfully"
  },
  "data": {
    "authorization_url": "https://auth.atlassian.com/authorize?client_id=...",
    "provider": "atlassian"
  }
}
```

### Comando get-url con PKCE

Para aplicaciones públicas:

```bash
echo '{
  "command": "get-url",
  "provider": "atlassian",
  "session_id": "session-001",
  "arguments": [
    {"name": "response_type", "value": "code"},
    {"name": "client_id", "value": "TU_CLIENT_ID"},
    {"name": "redirect_uri", "value": "https://tu-app.com/callback"},
    {"name": "scope", "value": "read:jira offline_access"},
    {"name": "state", "value": "random-state-123"},
    {"name": "code_challenge", "value": "HASH_S256_DE_TU_CODE_VERIFIER"},
    {"name": "code_challenge_method", "value": "S256"}
  ]
}' | grant-atlassian oauth2 get-url
```

### Comando get-token

Intercambia el código de autorización por tokens:

```bash
# Para aplicaciones confidenciales (con client_secret)
echo '{
  "command": "get-token",
  "provider": "atlassian",
  "session_id": "session-001",
  "arguments": [
    {"name": "code", "value": "CODIGO_RECIBIDO_EN_CALLBACK"},
    {"name": "client_id", "value": "TU_CLIENT_ID"},
    {"name": "client_secret", "value": "TU_CLIENT_SECRET"},
    {"name": "redirect_uri", "value": "https://tu-app.com/callback"}
  ]
}' | grant-atlassian oauth2 get-token

# Para aplicaciones públicas (con PKCE)
echo '{
  "command": "get-token",
  "provider": "atlassian",
  "session_id": "session-001",
  "arguments": [
    {"name": "code", "value": "CODIGO_RECIBIDO_EN_CALLBACK"},
    {"name": "client_id", "value": "TU_CLIENT_ID"},
    {"name": "redirect_uri", "value": "https://tu-app.com/callback"},
    {"name": "code_verifier", "value": "TU_CODE_VERIFIER_ORIGINAL"}
  ]
}' | grant-atlassian oauth2 get-token
```

Salida JSON:

```json
{
  "result": {
    "success": true,
    "message": "token obtained successfully"
  },
  "data": {
    "access_token": "eyJ...",
    "refresh_token": "eyJ...",
    "expires_in": 3600,
    "token_type": "Bearer",
    "scope": "read:jira write:jira"
  }
}
```

## Flujo OAuth2 completo

### 1. Generar URL de autorización

```bash
# Generar code_verifier para PKCE (ejemplo en bash)
CODE_VERIFIER=$(openssl rand -base64 32 | tr -d '=+/')
CODE_CHALLENGE=$(echo -n "$CODE_VERIFIER" | openssl dgst -sha256 -binary | openssl base64 | tr -d '=+/')

# Crear JSON de entrada
INPUT_JSON=$(cat <<EOF
{
  "command": "get-url",
  "provider": "atlassian",
  "session_id": "session-$(date +%s)",
  "arguments": [
    {"name": "response_type", "value": "code"},
    {"name": "client_id", "value": "TU_CLIENT_ID"},
    {"name": "redirect_uri", "value": "https://tu-app.com/callback"},
    {"name": "scope", "value": "read:jira offline_access"},
    {"name": "state", "value": "state-$(date +%s)"},
    {"name": "code_challenge", "value": "$CODE_CHALLENGE"},
    {"name": "code_challenge_method", "value": "S256"}
  ]
}
EOF
)

# Ejecutar comando
echo "$INPUT_JSON" | grant-atlassian oauth2 get-url
```

### 2. Redirigir al usuario

Extrae la `authorization_url` de la respuesta JSON y redirige al usuario. Después de autorizar, Atlassian redirige a:

```
https://tu-app.com/callback?code=AUTH_CODE&state=state-...
```

### 3. Intercambiar código por token

```bash
# Crear JSON de entrada
INPUT_JSON=$(cat <<EOF
{
  "command": "get-token",
  "provider": "atlassian",
  "session_id": "session-001",
  "arguments": [
    {"name": "code", "value": "AUTH_CODE_RECIBIDO"},
    {"name": "client_id", "value": "TU_CLIENT_ID"},
    {"name": "redirect_uri", "value": "https://tu-app.com/callback"},
    {"name": "code_verifier", "value": "$CODE_VERIFIER"}
  ]
}
EOF
)

# Ejecutar comando
echo "$INPUT_JSON" | grant-atlassian oauth2 get-token
```

## Argumentos

### get-url

| Argumento | Requerido | Descripción |
|-----------|------------|-------------|
| `response_type` | Sí | Siempre `"code"` para flujo de autorización |
| `client_id` | Sí | Client ID de tu aplicación OAuth2 en Atlassian |
| `redirect_uri` | Sí | URI de redirección registrada en la consola de desarrolladores |
| `scope` | Sí | Scopes solicitados separados por espacio. Incluye `offline_access` para obtener refresh token |
| `state` | Sí | Valor único para prevenir ataques CSRF |
| `code_challenge` | No | Hash del code_verifier para flujo PKCE |
| `code_challenge_method` | No | Método del challenge: `S256` o `plain`. Por defecto: `S256` |

### get-token

| Argumento | Requerido | Descripción |
|-----------|------------|-------------|
| `code` | Sí | Código de autorización recibido en el callback |
| `client_id` | Sí | Client ID de tu aplicación |
| `redirect_uri` | Sí | Debe coincidir con el usado en get-url |
| `client_secret` | No | Requerido para apps confidenciales |
| `code_verifier` | No | Requerido si se usó PKCE en get-url |

## Scopes disponibles

Atlassian ofrece varios scopes dependiendo del producto:

- **Jira**: `read:jira`, `write:jira`, `read:jira-work`, `write:jira-work`, `read:jira-user`, `offline_access`
- **Confluence**: `read:confluence`, `write:confluence`, `read:confluence-space.summary`, `offline_access`

Para más información: [OAuth 2.0 scopes](https://developer.atlassian.com/cloud/oauth/scopes/)

## Desarrollo

### Estructura del proyecto

```
grant-atlassian/
├── cmd/
│   └── main.go              # Punto de entrada del CLI (invocador por stdin)
├── internal/
│   ├── atlassian/
│   │   └── service.go       # Lógica OAuth2 de Atlassian
│   ├── handler/
│   │   ├── handler.go       # CommandHandler para grant-provider
│   │   └── handler_test.go  # Tests unitarios
│   └── provider/
│       ├── httpclient.go    # Cliente HTTP desacoplado
│       └── mock.go          # Mock HTTP para tests
├── bin/
│   └── grant-atlassian      # Binario compilado
├── Taskfile.yaml            # Tareas de automatización
├── go.mod                   # Módulo Go
└── README.md                # Esta documentación
```

### Tareas disponibles

```bash
# Ver todas las tareas
task

# Ejecutar tests
task test

# Ejecutar tests con cobertura
task test:coverage

# Compilar para Linux
task build:linux

# Compilar para sistema actual
task build

# Limpiar archivos generados
task clean

# Formatear código
task fmt

# Descargar dependencias
task deps
```

### Ejecutar tests

```bash
# Todos los tests
go test ./... -v

# Tests del handler específicamente
go test ./internal/handler/... -v
```

## Endpoints OAuth2 de Atlassian

- **Authorization URL**: `https://auth.atlassian.com/authorize`
- **Token Endpoint**: `https://auth.atlassian.com/oauth/token`

## Documentación oficial

- [Implementing OAuth 2.0 (3LO)](https://developer.atlassian.com/cloud/oauth/getting-started/implementing-oauth-3lo/)
- [OAuth 2.0 scopes](https://developer.atlassian.com/cloud/oauth/scopes/)
- [Atlassian Developer Console](https://developer.atlassian.com/console/myapps/)

## Licencia

MIT License - ver LICENSE para más detalles.

---

Desarrollado por [KaribuLab](https://github.com/KaribuLab)
