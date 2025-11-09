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

 - **Crear múltiples roles con una sola descripción (se aplica a todos)**
   ```bash
   kc roles create \
     --realm myrealm \
     --name admin \
     --name operator \
     --name auditor \
     --description "Roles base del sistema"
   ```

 - **Crear múltiples roles con descripciones por rol (ordenadas)**
   ```bash
   kc roles create \
     --realm myrealm \
     --name admin --description "Acceso total" \
     --name operator --description "Operaciones limitadas" \
     --name auditor --description "Solo lectura"
   ```

 - **Crear múltiples roles sin descripción**
   ```bash
   kc roles create --realm myrealm --name viewer --name reporter
   ```

 - **Crear múltiples roles en todos los realms**
   ```bash
   kc roles create --all-realms --name viewer --name auditor --description "Lectura global"
   ```

#### Flags específicos de `roles create`
- `--name <ROL>` Repetible. Debes proporcionar al menos un `--name` (requerido).
- `--description <TEXTO>` Repetible. Opcional. Reglas:
  - Sin `--description` → se crean sin descripción.
  - Un solo `--description` → se aplica a todos los `--name`.
  - Varios `--description` → deben ser exactamente uno por cada `--name`, en el mismo orden.
- `--all-realms` Crea el rol en todos los realms
- `--realm <REALM>` Realm destino (tiene prioridad sobre el global)

#### Resolución del realm destino
Orden de prioridad cuando ejecutas `roles create` (de mayor a menor):
1. Flag `--realm` del comando `roles create`.
2. Flag global `--realm` del comando raíz.
3. Valor `realm` en `config.json`.

#### Editar roles: `roles update`
- **Actualizar descripción de varios roles en un realm**
  ```bash
  kc roles update --realm myrealm \
    --name admin --name operator \
    --description "Nueva descripción"
  ```

- **Renombrar roles por orden en múltiples realms**
  ```bash
  kc roles update \
    --realm myrealm --realm sandbox \
    --name viewer --new-name read_only \
    --name auditor --new-name audit_read
  ```

Flags de `roles update`:
- `--name <ROL>` Repetible. Requerido.
- `--description <TEXTO>` Repetible. Opcional; 0, 1 o N (se empareja por orden con `--name`).
- `--new-name <NUEVO>` Repetible. Opcional; 0, 1 o N (se empareja por orden con `--name`).
- `--realm <REALM>` Realm destino. Si no se indica, usa el por defecto.
- `--all-realms` Aplica en todos los realms.
- `--ignore-missing` Si un rol no existe en el realm, omite en lugar de fallar.

#### Eliminar roles: `roles delete`
- **Eliminar roles en todos los realms (saltando los inexistentes)**
  ```bash
  kc roles delete --all-realms \
    --name temp_role --name deprecated_role \
    --ignore-missing
  ```

Flags de `roles delete`:
- `--name <ROL>` Repetible. Requerido.
- `--realm <REALM>` Realm destino. Si no se indica, usa el por defecto.
- `--all-realms` Elimina en todos los realms.
- `--ignore-missing` Si un rol no existe en el realm, omite en lugar de fallar.

### Users
- **Crear múltiples usuarios en un realm con una sola contraseña**
  ```bash
  kc users create \
    --realm myrealm \
    --username jdoe --username mjane \
    --password "S3guro!" \
    --first-name John --first-name Mary \
    --last-name Doe --last-name Jane \
    --email john@acme.com --email mary@acme.com
  ```

- **Crear usuarios con contraseñas por usuario y roles de realm**
  ```bash
  kc users create \
    --realm myrealm \
    --username a --password "Aa!1" --email a@acme.com \
    --username b --password "Bb!2" --email b@acme.com \
    --realm-role viewer --realm-role auditor
  ```

- **Crear usuarios en todos los realms, sin email (emailVerified=false)**
  ```bash
  kc users create \
    --all-realms \
    --username svc-1 --username svc-2 \
    --enabled=false
  ```

- **Crear usuarios en múltiples realms específicos**
  ```bash
  kc users create \
    --realm myrealm --realm sandbox \
    --username test1 --password "Test!123"
  ```

#### Flags específicos de `users create`
- `--username <USER>` Repetible. Debes proporcionar al menos un `--username` (requerido).
- `--email <EMAIL>` Repetible. Opcional; 0, 1 o N (se empareja por orden con `--username`). Si se especifica email, `emailVerified` será `true`, si no, `false`.
- `--first-name <NOMBRE>` Repetible. Opcional; 0, 1 o N.
- `--last-name <APELLIDO>` Repetible. Opcional; 0, 1 o N.
- `--password <PWD>` Repetible. Opcional; 0, 1 o N.
- `--enabled` Booleano. Por defecto `true`. Puedes deshabilitar con `--enabled=false`.
- `--realm <REALM>` Repetible. Realms destino. Si se omite y no usas `--all-realms`, se usa el realm por defecto (flag global o `config.json`).
- `--all-realms` Crea en todos los realms.
- `--realm-role <ROL>` Repetible. Asigna roles de realm existentes al usuario creado.