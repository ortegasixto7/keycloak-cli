# keycloak-cli

## Description
CLI to interact with Keycloak.

## Usage
Run the `./kc.exe` binary to see the general help.

```bash
./kc.exe --help
```

## Build on Windows

- **On Windows (PowerShell or CMD)**
  ```powershell
  go build -o kc.exe .
  ```

- **From macOS/Linux (cross-compile to Windows amd64)**
  ```bash
  GOOS=windows GOARCH=amd64 go build -o kc.exe .
  ```

- **From macOS/Linux (cross-compile to Windows arm64)**
  ```bash
  GOOS=windows GOARCH=arm64 go build -o kc.exe .
  ```

### Global flags
- `--config <path>`
  Configuration file (default: `config.json` next to the binary or in the current directory).
- `--realm <name>`
  Default realm to use.
- `--jira <ticket>`
  Jira ticket identifier used only for display in the boxed command output header.

## Commands and examples

> Note: all commands also accept the global `--jira <ticket>` flag. It only affects the visual header of the boxed output; it does not change the behavior of the command.

### Realms
- **List realms**
  ```bash
  ./kc.exe realms list --jira <TICKET>
  ```

### Roles
- **Create a role in a specific realm**
  ```bash
  ./kc.exe roles create --name <ROLE> --description "<DESCRIPTION>" --realm <REALM> --jira <TICKET>
  ```

- **Create a role using the default realm**
  ```bash
  ./kc.exe roles create --name <ROLE> --description "<DESCRIPTION>" --jira <TICKET>
  ```

- **Create a role in all realms**
  ```bash
  ./kc.exe roles create --name <ROLE> --description "<DESCRIPTION>" --jira <TICKET> --all-realms
  ```

 - **Create multiple roles with a single description (applied to all)**
   ```bash
   ./kc.exe roles create `
     --realm myrealm `
     --name admin `
     --name operator `
     --name auditor `
     --description "Base system roles" `
     --jira <TICKET>
   ```

 - **Create multiple roles with per-role descriptions (ordered)**
   ```bash
   ./kc.exe roles create `
     --realm myrealm `
     --name admin --description "Full access" `
     --name operator --description "Limited operations" `
     --name auditor --description "Read-only" `
     --jira <TICKET>
   ```

 - **Create multiple roles without description**
   ```bash
   ./kc.exe roles create --realm myrealm --name viewer --name reporter --jira <TICKET>
   ```

 - **Create multiple roles in all realms**
   ```bash
   ./kc.exe roles create --all-realms --name viewer --name auditor --description "Global read" --jira <TICKET>
   ```

#### Flags specific to `roles create`
- `--name <ROLE>` Repeatable. You must provide at least one `--name` (required).
- `--description <TEXT>` Repeatable. Optional. Rules:
  - No `--description` → roles are created without a description.
  - A single `--description` → applied to all `--name`.
  - Multiple `--description` → must be exactly one per `--name`, in the same order.
- `--all-realms` Create the role in all realms
- `--realm <REALM>` Target realm (takes precedence over the global one)

#### Target realm resolution
Priority order when you run `roles create` (from highest to lowest):
1. `--realm` flag on the `roles create` command.
2. Global `--realm` flag on the root command.
3. `realm` value in `config.json`.

#### Edit roles: `roles update`
- **Update the description of multiple roles in a realm**
  ```bash
  ./kc.exe roles update --realm myrealm `
    --name admin --name operator `
    --description "New description" `
    --jira <TICKET>
  ```

- **Rename roles by order in multiple realms**
  ```bash
  ./kc.exe roles update `
    --realm myrealm --realm sandbox `
    --name viewer --new-name read_only `
    --name auditor --new-name audit_read `
    --jira <TICKET>
  ```

Flags for `roles update`:
- `--name <ROLE>` Repeatable. Required.
- `--description <TEXT>` Repeatable. Optional; 0, 1 or N (paired by order with `--name`).
- `--new-name <NEW>` Repeatable. Optional; 0, 1 or N (paired by order with `--name`).
- `--realm <REALM>` Target realm. If not provided, uses the default.
- `--all-realms` Applies to all realms.
- `--ignore-missing` If a role does not exist in the realm, skip instead of failing.

#### Delete roles: `roles delete`
- **Delete roles in all realms (skipping non-existent ones)**
  ```bash
  ./kc.exe roles delete --all-realms `
    --name temp_role --name deprecated_role `
    --ignore-missing `
    --jira <TICKET>
  ```

Flags for `roles delete`:
- `--name <ROLE>` Repeatable. Required.
- `--realm <REALM>` Repeatable. Target realms. If not provided, uses the default.
- `--all-realms` Delete in all realms.
- `--ignore-missing` Skip non-existent roles instead of failing.

### Client Roles
- **Create a client role in a specific client and realm**
  ```bash
  ./kc.exe client-roles create `
    --client-id my-app `
    --name app-admin `
    --description "Admin role for my-app" `
    --realm myrealm `
    --jira <TICKET>
  ```

- **Create multiple client roles with a single description (applied to all)**
  ```bash
  ./kc.exe client-roles create `
    --client-id my-app `
    --realm myrealm `
    --name app-admin `
    --name app-user `
    --description "Application roles" `
    --jira <TICKET>
  ```

- **Create multiple client roles in all realms**
  ```bash
  ./kc.exe client-roles create `
    --client-id my-app `
    --all-realms `
    --name app-admin `
    --name app-user `
    --description "Global app roles" `
    --jira <TICKET>
  ```

#### Flags specific to `client-roles create`
- `--client-id <CLIENT_ID>` Target client-id. Required.
- `--name <ROLE>` Repeatable. You must provide at least one `--name` (required).
- `--description <TEXT>` Repeatable. Optional. Rules (same pattern as `roles create`):
  - No `--description` → roles are created without a description.
  - A single `--description` → applied to all `--name`.
  - Multiple `--description` → must be exactly one per `--name`, in the same order.
- `--all-realms` Create the client role(s) in all realms.
- `--realm <REALM>` Target realm (takes precedence over the global one).

### Users
- **Create multiple users in a realm with a single password**
  ```bash
  ./kc.exe users create `
    --realm myrealm `
    --username jdoe --username mjane `
    --password "Str0ng!" `
    --first-name John --first-name Mary `
    --last-name Doe --last-name Jane `
    --email john@acme.com --email mary@acme.com `
    --jira <TICKET>
  ```

- **Create users with per-user passwords and realm roles**
  ```bash
  ./kc.exe users create `
    --realm myrealm `
    --username a --password "Aa!1" --email a@acme.com `
    --username b --password "Bb!2" --email b@acme.com `
    --realm-role viewer --realm-role auditor `
    --jira <TICKET>
  ```

- **Create users in all realms, without email (emailVerified=false)**
  ```bash
  ./kc.exe users create `
    --all-realms `
    --username svc-1 --username svc-2 `
    --enabled=false `
    --jira <TICKET>
  ```

- **Create users in multiple specific realms**
  ```bash
  ./kc.exe users create `
    --realm myrealm --realm sandbox `
    --username test1 --password "Test!123" `
    --jira <TICKET>
  ```

#### Flags specific to `users create`
- `--username <USER>` Repeatable. You must provide at least one `--username` (required).
- `--email <EMAIL>` Repeatable. Optional; 0, 1 or N (paired by order with `--username`). If email is provided, `emailVerified` will be `true`, otherwise `false`.
- `--first-name <FIRST>` Repeatable. Optional; 0, 1 or N.
- `--last-name <LAST>` Repeatable. Optional; 0, 1 or N.
- `--password <PWD>` Repeatable. Optional; 0, 1 or N.
- `--enabled` Boolean. Default `true`. You can disable with `--enabled=false`.
- `--realm <REALM>` Repeatable. Target realms. If omitted and you don't use `--all-realms`, the default realm is used (global flag or `config.json`).
- `--all-realms` Create in all realms.
- `--realm-role <ROLE>` Repeatable. Assign existing realm roles to the created user.
 - `--client-role <ROLE>` Repeatable. Assign existing client roles (from the client given by `--client-id`) to the created user.
 - `--client-id <CLIENT_ID>` Client whose roles will be assigned when using `--client-role`. Required if `--client-role` is provided.

#### Edit users: `users update`
- **Update password and enable multiple users**
  ```bash
  ./kc.exe users update `
    --realm myrealm `
    --username jdoe --username mjane `
    --password "N3wP@ss!" `
    --enabled=true `
    --jira <TICKET>
  ```

- **Update fields per user (ordered)**
  ```bash
  ./kc.exe users update `
    --realm myrealm `
    --username a --email a@acme.com --first-name Ann --last-name A `
    --username b --email b@acme.com --first-name Ben --last-name B `
    --jira <TICKET>
  ```

Flags for `users update`:
- `--username <USER>` Repeatable. Required.
- `--email <EMAIL>` Repeatable. 0, 1 or N (paired by order). If specified, `emailVerified=true`.
- `--first-name <FIRST>` Repeatable. 0, 1 or N.
- `--last-name <LAST>` Repeatable. 0, 1 or N.
- `--password <PWD>` Repeatable. 0, 1 or N.
- `--enabled` Boolean. If the flag is included, apply the value to the target users.
- `--realm <REALM>` Repeatable. Target realms.
- `--all-realms` Applies to all realms.
- `--ignore-missing` Skip non-existent users instead of failing.

#### Delete users: `users delete`
- **Delete users in multiple realms, ignoring non-existent ones**
  ```bash
  ./kc.exe users delete `
    --realm myrealm --realm sandbox `
    --username olduser1 --username olduser2 `
    --ignore-missing `
    --jira <TICKET>
  ```

Flags for `users delete`:
- `--username <USER>` Repeatable. Required.
- `--realm <REALM>` Repeatable. Target realms.
- `--all-realms` Delete in all realms.
- `--ignore-missing` Skip non-existent users instead of failing.

### Clients
- **Create client(s)**
  ```bash
  ./kc.exe clients create `
    --realm myrealm `
    --client-id app-frontend `
    --name "App Frontend" `
    --public=true `
    --redirect-uri https://app.example.com/callback `
    --web-origin https://app.example.com `
    --jira <TICKET>
  ```

- **Update client(s)**
  ```bash
  ./kc.exe clients update `
    --realm myrealm `
    --client-id app-frontend `
    --name "App Frontend v2" `
    --root-url https://app.example.com `
    --base-url / `
    --jira <TICKET>
  ```

- **Delete client(s)**
  ```bash
  ./kc.exe clients delete --realm myrealm --client-id app-frontend --ignore-missing --jira <TICKET>
  ```

- **List clients**
  ```bash
  ./kc.exe clients list --realm myrealm --jira <TICKET>
  ```

Flags para `clients` (principales):
- `--client-id <ID>` Repeatable en create/update/delete. Requerido para create/update/delete.
- `--name`, `--public`, `--enabled`, `--protocol`, `--root-url`, `--base-url`.
- `--redirect-uri`, `--web-origin` (lista aplicada a todos los seleccionados cuando se usa en update/create).
- `--standard-flow`, `--direct-access`, `--implicit-flow`, `--service-accounts` (bool 0/1/N).
- `--new-client-id` para renombrar en `update` (0/1/N).
- `--realm` (0/1/N) o `--all-realms`.
- `--ignore-missing` en `update/delete` para omitir inexistentes.

Nota:
- El seteo explícito de `--secret` no está soportado por la librería usada; el comando emitirá un warning y lo omitirá.

#### Asignar scopes a un client
- **Asignar scopes**
  ```bash
  ./kc.exe clients scopes assign `
    --realm myrealm `
    --client-id app-frontend `
    --type default `
    --scope profile --scope email `
    --jira <TICKET>
  ```

- **Remover scopes**
  ```bash
  ./kc.exe clients scopes remove `
    --realm myrealm `
    --client-id app-frontend `
    --type optional `
    --scope address --ignore-missing `
    --jira <TICKET>
  ```

Flags:
- `--client-id <ID>` Requerido.
- `--scope <NAME>` Repeatable. Requerido.
- `--type default|optional` (default: `default`).
- `--realm` requerido (o global), o `--all-realms` en assign/remove si deseas aplicar a múltiples realms.
- `--ignore-missing` en remove para omitir scopes no asignados.

### Client Scopes
- **Crear client scopes**
  ```bash
  ./kc.exe client-scopes create `
    --realm myrealm `
    --name profile --description "Standard profile" --protocol openid-connect `
    --jira <TICKET>
  ```

- **Actualizar client scopes**
  ```bash
  ./kc.exe client-scopes update `
    --realm myrealm `
    --name profile --new-name profile_v2 --description "Updated" `
    --jira <TICKET>
  ```

- **Eliminar client scopes**
  ```bash
  ./kc.exe client-scopes delete --realm myrealm --name profile --ignore-missing --jira <TICKET>
  ```

- **Listar client scopes**
  ```bash
  ./kc.exe client-scopes list --realm myrealm --jira <TICKET>
  ```

Flags para `client-scopes`:
- `--name <NAME>` Repeatable. Requerido en create/update/delete.
- `--description`, `--protocol` (0/1/N). `protocol` por defecto: `openid-connect`.
- `--new-name` en update (0/1/N).
- `--realm` o `--all-realms`.
- `--ignore-missing` en update/delete para omitir inexistentes.

## Logging
- Toda la salida estándar y de error se duplica en `kc.log` (en el directorio de ejecución o según `--log-file`).
- Cada comando imprime marcas de tiempo `START`/`END` y errores con su duración.

