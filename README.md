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
- Una aplicación OAuth 2.0 creada en la [Consola de Desarrolladores de Atlassian](https://developer.atlassian.com/console/myapps/)

## Obtener client_id y client_secret

Para usar este provider necesitas crear una aplicación OAuth 2.0 en Atlassian y obtener tus credenciales. Sigue estos pasos:

### 1. Acceder a la Consola de Desarrolladores

1. Ve a [developer.atlassian.com](https://developer.atlassian.com)
2. Haz clic en tu icono de perfil (esquina superior derecha)
3. Selecciona **"Developer console"** del menú desplegable

### 2. Crear una nueva aplicación

1. En la Consola de Desarrolladores, haz clic en **"Create"** o **"Crear"**
2. Selecciona **"OAuth 2.0 integration"** (Integración OAuth 2.0)
3. Ingresa un nombre para tu aplicación (ej: "Mi Integración Atlassian")
4. Haz clic en **"Create"** para crear la aplicación

### 3. Configurar OAuth 2.0 (3LO)

1. En el menú lateral, selecciona **"Authorization"**
2. Haz clic en **"Configure"** junto a **"OAuth 2.0 (3LO)"**
3. Ingresa tu **Callback URL** (debe coincidir con el `redirect_uri` que uses en las llamadas)
   - Ejemplo: `https://mi-app.com/callback` o `http://localhost:8080/callback`
4. Guarda la configuración

### 4. Configurar Permisos (Scopes)

1. En el menú lateral, selecciona **"Permissions"**
2. Agrega las APIs que necesites:
   - **Jira Cloud**: Para acceder a Jira
   - **Confluence Cloud**: Para acceder a Confluence
3. Configura los scopes necesarios:
   - `read:jira` - Leer datos de Jira
   - `write:jira` - Escribir datos de Jira
   - `offline_access` - Para obtener refresh tokens
4. Haz clic en **"Save changes"**

### 5. Obtener tus Credenciales

1. En el menú lateral, selecciona **"Settings"**
2. Aquí encontrarás:
   - **Client ID**: El identificador de tu aplicación (se usa en `client_id`)
   - **Secret**: El secreto de tu aplicación (se usa en `client_secret`)

```bash
# Ejemplo de credenciales
client_id: "abc123def456ghi789"
client_secret: "s3cr3t_k3y_xyz789"
```

**Nota importante**: El `client_secret` solo se muestra una vez al crear la aplicación. Si lo pierdes, deberás generar uno nuevo en la sección Settings.

### Distribución de la App (Opcional)

Si deseas que otros usuarios puedan usar tu aplicación:

1. Ve a **"Distribution"** en el menú lateral
2. Completa la información requerida (descripción, política de privacidad, etc.)
3. Cambia el estado a **"Sharing"**
4. Atlassian revisará tu aplicación antes de aprobarla

Para uso interno o desarrollo, puedes dejarla en modo **"Not sharing"**.

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
    {"name": "redirect_uri", "value": "http://127.0.0.1:1215/callback/atlassian"},
    {"name": "scope", "value": "read:jira write:jira offline_access"},
    {"name": "state", "value": "session-001"}
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
    {"name": "redirect_uri", "value": "http://127.0.0.1:1215/callback/atlassian"},
    {"name": "scope", "value": "read:jira offline_access"},
    {"name": "state", "value": "session-001"},
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
    {"name": "redirect_uri", "value": "http://127.0.0.1:1215/callback/atlassian"}
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
    {"name": "redirect_uri", "value": "http://127.0.0.1:1215/callback/atlassian"},
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
    {"name": "redirect_uri", "value": "http://127.0.0.1:1215/callback/atlassian"},
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
http://127.0.0.1:1215/callback/atlassian?code=AUTH_CODE&state=state-...
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
    {"name": "redirect_uri", "value": "http://127.0.0.1:1215/callback/atlassian"},
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

### Guías de Atlassian

- [Implementing OAuth 2.0 (3LO)](https://developer.atlassian.com/cloud/oauth/getting-started/implementing-oauth-3lo/) - Guía completa del flujo de autorización
- [Managing your OAuth 2.0 apps](https://developer.atlassian.com/cloud/oauth/getting-started/managing-oauth-apps/) - Cómo gestionar tus aplicaciones OAuth
- [Enabling OAuth 2.0 3LO](https://developer.atlassian.com/cloud/oauth/getting-started/enabling-oauth-3lo/) - Habilitar OAuth 2.0 en tu app
- [OAuth 2.0 scopes](https://developer.atlassian.com/cloud/oauth/scopes/) - Lista completa de scopes disponibles

### Consola y Credenciales

- [Atlassian Developer Console](https://developer.atlassian.com/console/myapps/) - Crear y gestionar aplicaciones
- [Create OAuth 2.0 credential](https://support.atlassian.com/user-management/docs/create-oauth-2-0-credential-for-service-accounts/) - Crear credenciales OAuth 2.0

## Licencia

MIT License - ver LICENSE para más detalles.

---

Desarrollado por [KaribuLab](https://github.com/KaribuLab)
