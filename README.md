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

## Commands and examples

### Realms
- **List realms**
  ```bash
  ./kc.exe realms list
  ```

### Roles
- **Create a role in a specific realm**
  ```bash
  ./kc.exe roles create --name <ROLE> --description "<DESCRIPTION>" --realm <REALM>
  ```

- **Create a role using the default realm**
  ```bash
  ./kc.exe roles create --name <ROLE> --description "<DESCRIPTION>"
  ```

- **Create a role in all realms**
  ```bash
  ./kc.exe roles create --name <ROLE> --description "<DESCRIPTION>" --all-realms
  ```

 - **Create multiple roles with a single description (applied to all)**
   ```bash
   ./kc.exe roles create \
     --realm myrealm \
     --name admin \
     --name operator \
     --name auditor \
     --description "Base system roles"
   ```

 - **Create multiple roles with per-role descriptions (ordered)**
   ```bash
   ./kc.exe roles create \
     --realm myrealm \
     --name admin --description "Full access" \
     --name operator --description "Limited operations" \
     --name auditor --description "Read-only"
   ```

 - **Create multiple roles without description**
   ```bash
   ./kc.exe roles create --realm myrealm --name viewer --name reporter
   ```

 - **Create multiple roles in all realms**
   ```bash
   ./kc.exe roles create --all-realms --name viewer --name auditor --description "Global read"
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
  ./kc.exe roles update --realm myrealm \
    --name admin --name operator \
    --description "New description"
  ```

- **Rename roles by order in multiple realms**
  ```bash
  ./kc.exe roles update \
    --realm myrealm --realm sandbox \
    --name viewer --new-name read_only \
    --name auditor --new-name audit_read
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
  ./kc.exe roles delete --all-realms \
    --name temp_role --name deprecated_role \
    --ignore-missing
  ```

Flags for `roles delete`:
- `--name <ROLE>` Repeatable. Required.
- `--realm <REALM>` Repeatable. Target realms. If not provided, uses the default.
- `--all-realms` Delete in all realms.
- `--ignore-missing` Skip non-existent roles instead of failing.

### Users
- **Create multiple users in a realm with a single password**
  ```bash
  ./kc.exe users create \
    --realm myrealm \
    --username jdoe --username mjane \
    --password "Str0ng!" \
    --first-name John --first-name Mary \
    --last-name Doe --last-name Jane \
    --email john@acme.com --email mary@acme.com
  ```

- **Create users with per-user passwords and realm roles**
  ```bash
  ./kc.exe users create \
    --realm myrealm \
    --username a --password "Aa!1" --email a@acme.com \
    --username b --password "Bb!2" --email b@acme.com \
    --realm-role viewer --realm-role auditor
  ```

- **Create users in all realms, without email (emailVerified=false)**
  ```bash
  ./kc.exe users create \
    --all-realms \
    --username svc-1 --username svc-2 \
    --enabled=false
  ```

- **Create users in multiple specific realms**
  ```bash
  ./kc.exe users create \
    --realm myrealm --realm sandbox \
    --username test1 --password "Test!123"
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

#### Edit users: `users update`
- **Update password and enable multiple users**
  ```bash
  ./kc.exe users update \
    --realm myrealm \
    --username jdoe --username mjane \
    --password "N3wP@ss!" \
    --enabled=true
  ```

- **Update fields per user (ordered)**
  ```bash
  ./kc.exe users update \
    --realm myrealm \
    --username a --email a@acme.com --first-name Ann --last-name A \
    --username b --email b@acme.com --first-name Ben --last-name B
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
  ./kc.exe users delete \
    --realm myrealm --realm sandbox \
    --username olduser1 --username olduser2 \
    --ignore-missing
  ```

Flags for `users delete`:
- `--username <USER>` Repeatable. Required.
- `--realm <REALM>` Repeatable. Target realms.
- `--all-realms` Delete in all realms.
- `--ignore-missing` Skip non-existent users instead of failing.