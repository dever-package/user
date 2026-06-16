---
name: dever-user
description: This skill should be used when editing the Dever user package, including user account, auth, API key, credential, identity, benefit, point, token, middleware, user APIs, user services, user page JSON, permissions, secrets, and migration behavior.
version: 0.1.0
---

# User Package

Use this component skill together with `shemic-dever`. Read `shemic-dever` first for framework rules, then apply these user-specific boundaries.

## Source Of Truth

- Package source: `backend/package/user`
- Component metadata: `backend/package/user/dever.json`
- Models: `model`
- Services: `service`
- APIs: `api`
- Auth context: `authctx`
- Middleware: `middleware`
- Pages: `front/page`

## Hard Rules

- Do not expose account, token, credential, API key, identity, point, or benefit mutations through generic unsafe actions.
- Do not log secrets, tokens, passwords, API keys, or credential payloads.
- Do not place real secrets in templates, page JSON, or source.
- API handlers stay thin; auth, token, identity, point, benefit, and API-key rules belong in services.
- Middleware must remain focused on authentication/context injection and avoid project-specific business rules.
- Ordinary admin list/update pages should use package/front and model metadata when safe.

## Service/API Rules

- `service/auth`: login, token, session, credential behavior.
- `service/api_key`: API key creation, validation, masking, rotation.
- `service/identity`: identity and level behavior.
- `service/point`: point accounting and logs.
- `service/benefit`: benefit grant/issue behavior.
- `authctx`: request actor/API-key context only.

Use transactions or idempotency for point, benefit, and identity changes.

## Common Checks

- Permission errors: inspect site access, user middleware, and page/action auth before changing model/action rules.
- Credential bugs: check secret hashing/masking paths before returning data to frontend.
- API key bugs: never return full secret after creation unless existing service explicitly supports one-time display.
