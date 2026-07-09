# Auth.js Token Refresh - Implementation Plan

## Background

The Auth.js session stores the Keycloak access token in the session JWT cookie. The
token is only set once, at initial login. When it expires (Keycloak default: 5 minutes),
`isTokenExpired` in hooks.server.ts detects this and redirects to login.

The quick fix (patching the base64url decode bug) restores working behaviour. But the
underlying architecture is fragile: every session older than 5 minutes is treated as
expired, forcing a full login redirect. Keycloak auto-authenticates via SSO in most
cases, so users rarely see the Keycloak login form - but it's one more round trip, one
more cookie cycle, and one more point of failure.

The proper fix is to implement token refresh in Auth.js: when the access token is near
expiry, silently exchange the refresh token for a new one. The user's session stays alive
indefinitely as long as they visit within the refresh token lifetime (days to weeks in
Keycloak defaults).

## Changes required

### `web/src/auth.ts`

Add a `refreshAccessToken` helper and extend the `jwt` callback.

**On initial sign-in** (`account` present), store three fields instead of one:

```typescript
if (account) {
    return {
        ...token,
        accessToken: account.access_token,
        refreshToken: account.refresh_token,
        accessTokenExpires: account.expires_at
            ? account.expires_at * 1000
            : Date.now() + ((account.expires_in as number ?? 300) * 1000),
    };
}
```

**On subsequent calls**, check expiry with a 30-second buffer and refresh if needed:

```typescript
if (Date.now() < (token.accessTokenExpires as number) - 30_000) {
    return token;
}
return refreshAccessToken(token);
```

**`refreshAccessToken` helper** - calls the Keycloak token endpoint directly:

```typescript
async function refreshAccessToken(token: any) {
    try {
        const issuer = process.env.AUTH_KEYCLOAK_ISSUER!;
        const res = await fetch(`${issuer}/protocol/openid-connect/token`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
            body: new URLSearchParams({
                grant_type: 'refresh_token',
                client_id: process.env.AUTH_KEYCLOAK_ID!,
                client_secret: process.env.AUTH_KEYCLOAK_SECRET!,
                refresh_token: token.refreshToken,
            }),
        });
        const tokens = await res.json();
        if (!res.ok) throw tokens;
        return {
            ...token,
            accessToken: tokens.access_token,
            refreshToken: tokens.refresh_token ?? token.refreshToken,
            accessTokenExpires: Date.now() + (tokens.expires_in * 1000),
            error: undefined,
        };
    } catch (err) {
        console.error('[auth] token refresh failed:', err);
        return { ...token, error: 'RefreshAccessTokenError' };
    }
}
```

**`session` callback** - also expose the error flag:

```typescript
async session({ session, token }) {
    (session as any).accessToken = token.accessToken;
    (session as any).error = token.error;
    return session;
}
```

### `web/src/hooks.server.ts`

Remove `isTokenExpired` entirely, including its function block and the blank line after it. Update `getAccessToken` to check the error flag
instead:

```typescript
async function getAccessToken(event: any): Promise<string | null> {
    try {
        const session = await event.locals.auth?.();
        if (!session) return null;
        if ((session as any).error === 'RefreshAccessTokenError') return null;
        return (session as any).accessToken ?? null;
    } catch {
        return null;
    }
}
```

The cookie clearing block in the outer wrapper stays unchanged. It handles the
`RefreshAccessTokenError` case: refresh token expired (weeks of inactivity) triggers a
redirect to login, the outer wrapper clears the stale session cookie, and the user gets a
single clean Keycloak login prompt.

Also remove the two `console.log` lines in the outer wrapper - they were added for
debugging the redirect loop and are no longer useful.

Note on error handling: a network failure reaching Keycloak during refresh is treated
identically to an expired refresh token - both produce `RefreshAccessTokenError` and
trigger a logout redirect. This is intentional. The `console.error` log in
`refreshAccessToken` lets ops distinguish the two cases via logs.

### `web/src/routes/+layout.server.ts`

No changes needed. The hook redirects to login before `resolve()` is called when
`getAccessToken` returns null, so the layout never runs with a bad session.

## Known limitation: parallel refresh calls

When multiple server-side requests fire in parallel (layout load, page load, and `/api/*`
proxy calls each go through the hook independently), all of them may call `auth()` within
the same tick and each independently trigger a refresh against Keycloak. At a 300-second
token lifetime this race window is a fraction of a second every ~4.5 minutes - benign in
practice, but measurable at scale.

The fix is a module-level promise cache in `auth.ts` keyed by refresh token, so parallel
requests share one Keycloak call. Roughly 20-30 lines. Deferred until production logs
show it as an actual problem.

## What this changes for the user

- Sessions no longer expire every 5 minutes from the user's perspective
- Keycloak role/membership changes (new troop, removed role) propagate within one refresh
  cycle (typically 5 minutes) without requiring a logout
- Re-login is only needed when the Keycloak refresh token itself expires (days to weeks
  depending on Keycloak configuration)

## Testing

- Log in, wait longer than `expires_in` (check Keycloak admin for the value, typically
  300 seconds), verify the session is still active and API calls work
- Verify `docker compose logs web` shows no redirect-to-login entries during normal use
- To test the refresh-token-expired path: manually delete the `refreshToken` from the
  stored session (or shorten the Keycloak refresh token lifetime in admin), verify a
  single clean redirect to login occurs with no loop
- Note: rotating `AUTH_SECRET` invalidates all existing sessions immediately, forcing
  every active user to re-login. Plan rotations for low-traffic windows.
