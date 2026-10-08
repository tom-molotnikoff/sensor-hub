---
id: user-management
title: User Management and RBAC
sidebar_position: 9
---

# User Management and RBAC

Sensor Hub includes a role-based access control (RBAC) system that governs what each user can see and do. Users are assigned roles, and roles contain permissions that control access to specific features and API endpoints.

## User accounts

Each user account has:

- A unique username
- An email address
- One or more roles
- An optional "must change password" flag

The first admin user is created on the server with `sensor-hub local admin create` (see [Installation](installation#create-the-first-admin-user)). Additional users are created through the User Management page in the web UI or via the API.

User accounts can be disabled without deletion. A disabled user cannot sign in or use the API, but their account, API keys and data are kept and come back into use when the account is enabled again.

## Creating and managing users

User management requires the `manage_users` permission. From the User Management page, administrators can:

- Create new user accounts with a username, email, password, and assigned roles
- Delete user accounts (you cannot delete your own account)
- Set the "must change password" flag on a user, which forces them to change their password on next login
- Assign or change a user's roles
- Disable or enable a user (see [Disabling a user](#disabling-a-user))

## Disabling a user

Disable a user when they should lose access straight away but you want to keep their account, for example when someone leaves or a password may have leaked. It needs the `manage_users` permission.

In the web UI, open the user's row in the Manage Users card, choose **Edit**, turn on **Disabled** and save. From the command line:

```bash
sensor-hub users disable <id>
sensor-hub users enable <id>
```

Both call `PUT /api/users/{id}/disabled` with `{ "disabled": true }` or `{ "disabled": false }`.

When a user is disabled:

- Every session they hold is deleted in the same request, so they are signed out everywhere at once.
- Every request made with one of their API keys gets 401 Unauthorized. The keys are not revoked, so they work again if the user is enabled.
- Signing in with the right password answers "account disabled". A wrong password answers "invalid credentials", as for anyone else.

Two changes are refused:

- You cannot disable or enable your own account (400 Bad Request).
- You cannot disable the last enabled user holding the admin role (409 Conflict), so the hub always keeps an admin who can sign in.

## API keys

Every role is granted `manage_api_keys`, which lets a user create, list, change the expiry of, revoke and delete their own API keys. Another user's key answers 404 Not Found and is left unchanged, so users cannot see or disturb each other's keys. A user with `manage_users` can change the expiry of, revoke or delete any user's key by its id.

An API key acts as its owner. While the owner must change their password, a request with their key gets the same 403 Forbidden as their browser session on every route except signing in and out, reading their own details and changing their password.

## Password management

- Users can change their own password from the web UI at any time
- Passwords are hashed using bcrypt with a configurable cost factor (see [Configuration Settings](configuration))

## Roles

Sensor Hub includes three built-in roles:

| Role   | Purpose                                        |
|--------|------------------------------------------------|
| admin  | Full administrative access to all features     |
| user   | Standard access for day-to-day use             |
| viewer | Read-only access to sensor data and dashboards |

A user's effective permissions are the union of all permissions granted to their assigned roles. A user can have multiple roles.

## Permissions

Permissions are granular access controls that are assigned to roles. Administrators can view and modify the permissions assigned to each role from the User Management page.

## Permission enforcement

Permissions are enforced at the API level. Each endpoint declares its required permission, and the server validates that the authenticated user's roles include that permission before processing the request. Requests without the required permission receive a 403 Forbidden response.

The web UI also uses permissions to control visibility.