# keycloak-cli

## Descripción
CLI para interactuar con Keycloak.

## Uso
Ejecuta el binario `kc` para ver la ayuda general.

```bash
kc --help
```

## Compilar en Windows

- **En Windows (PowerShell o CMD)**
  ```powershell
  go build -o kc.exe .
  ```

- **Desde macOS/Linux (cross-compile a Windows amd64)**
  ```bash
  GOOS=windows GOARCH=amd64 go build -o kc.exe .
  ```

- **Desde macOS/Linux (cross-compile a Windows arm64)**
  ```bash
  GOOS=windows GOARCH=arm64 go build -o kc.exe .
  ```

### Flags globales
- `--config <ruta>`
  Archivo de configuración (por defecto: `config.json` junto al binario o en el directorio actual).
- `--realm <nombre>`
  Realm por defecto a utilizar.

## Comandos y ejemplos

### Realms
- **Listar realms**
  ```bash
  kc realms list
  ```

### Roles
- **Crear un rol en un realm específico**
  ```bash
  kc roles create --name <ROL> --description "<DESCRIPCION>" --realm <REALM>
  ```

- **Crear un rol usando el realm por defecto**
  ```bash
  kc roles create --name <ROL> --description "<DESCRIPCION>"
  ```

- **Crear un rol en todos los realms**
  ```bash
  kc roles create --name <ROL> --description "<DESCRIPCION>" --all-realms
  ```

#### Flags específicos de `roles create`
- `--name <ROL>` (requerido)
- `--description <TEXTO>`
- `--all-realms` Crea el rol en todos los realms
- `--realm <REALM>` Realm destino (tiene prioridad sobre el global)

#### Resolución del realm destino
Orden de prioridad cuando ejecutas `roles create` (de mayor a menor):
1. Flag `--realm` del comando `roles create`.
2. Flag global `--realm` del comando raíz.
3. Valor `realm` en `config.json`.