# Authentication and Authorisation

This guide explains how authentication, session management, CSRF protection,
and role-based access control work across the full stack.

## Authentication Methods

Sensor Hub supports two authentication methods, checked in this order:

1. **API Key** — `X-API-Key` header, used by the CLI and programmatic clients
2. **Session Cookie** — `sensor_hub_session` HTTP-only cookie, used by the React UI

### Session-Based Auth (Browser)

```
Browser                         Sensor Hub
  │                                │
  ├─ POST /api/auth/login ────────►│
  │  {username, password}          │ Validate credentials
  │                                │ Create session (token_hash + csrf_token)
  │◄── Set-Cookie: sensor_hub_session=<token>
  │    Body: {csrf_token, must_change_password}
  │                                │
  ├─ GET /api/sensors/ ───────────►│
  │  Cookie: sensor_hub_session    │ Validate session
  │                                │ Set currentUser in context
  │◄── 200 OK [{...}]             │
  │                                │
  ├─ POST /api/sensors/ ──────────►│
  │  Cookie: sensor_hub_session    │ Validate CSRF token
  │  X-CSRF-Token: <token>         │ Validate session
  │  {name, type, url}             │ Check manage_sensors permission
  │◄── 201 Created                │
```

The session token is hashed before storage. The CSRF token is returned in the
login response and must be sent as `X-CSRF-Token` on all state-changing
requests. The frontend stores it in-memory and sends it automatically. CSRF
validation is skipped for safe HTTP methods (GET, HEAD, OPTIONS), API key
requests, and the login/logout endpoints. The token can be refreshed via
`GET /api/auth/me`.

### API Key Auth (CLI / Programmatic)

```
CLI                             Sensor Hub
  │                                │
  ├─ GET /api/sensors/ ───────────►│
  │  X-API-Key: shk_abc123...     │ Hash key, look up in api_keys table
  │                                │ Check not expired or revoked
  │                                │ Load owning user and permissions
  │◄── 200 OK [{...}]             │
```

API key authentication bypasses CSRF checks (API keys are not automatically
sent by the browser).

API keys are hashed before storage. 

## Must Change Password

When a user is created through the API, `must_change_password` is set to
`true`. The first admin created with `local admin create` gets it only with
`--must-change-password`. This forces the user to change their password on first login. During this time,
All other endpoints return 403 Forbidden. The flag is cleared after the user
changes their password.

## Role-Based Access Control

### Roles

Three roles are created by the initial migration:

| Role | Description |
|------|-------------|
| `admin` | Full access to all features |
| `user` | Standard access (read + some write) |
| `viewer` | Read-only access |

### Permissions

There are permissions defined in the database The admin role is granted all permissions 
by default.

### Adding a New Permission

1. Create a new migration that inserts into the `permissions` table
2. Grant it to the admin role via `role_permissions`
3. Use `middleware.RequirePermission("your_permission")` on the relevant routes

## Login Rate Limiting

Failed login attempts are tracked. The system applies exponential backoff when 
the number of failures within a configurable window exceeds a threshold:

Rate limiting is applied per-username and per-IP-address independently.

## OAuth (Gmail SMTP)

OAuth 2.0 is used specifically for Gmail SMTP integration (sending alert
emails). It is not a user-facing login method.