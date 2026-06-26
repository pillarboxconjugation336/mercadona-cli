# mercadona

Unofficial, agent-friendly CLI for `tienda.mercadona.es` — search the catalog, read
prices, and (soon) build a cart and check out. Single static Go binary, no runtime
deps, structured `--json` output for programmatic/agent use.

> Unofficial. Mercadona has no public API. Bring your own credentials; use at a
> sane request rate. This talks to the same HTTP endpoints the website does.

## Build

```bash
go build -o mercadona .
```

## Commands

### Read core (anonymous — no login)

```bash
mercadona search queso                      # full-text product search
mercadona search --limit 5 --json mayonesa  # structured output for agents
mercadona batch -f lista.txt                 # many terms in ONE request (≈100 items / call)
printf 'queso\ncarne\nmayonesa\n' | mercadona batch -f -
mercadona product 13406                      # detail + price
mercadona categories                         # category tree
mercadona categories --id 112 --json         # one category's products (raw JSON)
```

Common flags: `--wh mad1` (warehouse), `--lang es`, `--json`.
Data goes to **stdout**, logs/errors to **stderr**, exit code `1` on error — friendly to scripts and agents.

Example:

```
$ mercadona batch -f lista.txt
• queso            → [51110] Queso rallado mozzarella pizza-Roma Hacendado — 1.60€ (8.000€/kg)
• carne            → [34157] Carne de pimiento choricero Hacendado — 1.55€ (11.072€/kg)
• mayonesa         → [13406] Mayonesa Hacendado — 1.20€ (2.400€/L)
```

### Authenticated: `login` / `import-curl`, `whoami`, `cart`, `checkout`

The API authenticates with a **Bearer token** (a SimpleJWT). **Password login requires a
Google reCAPTCHA Enterprise token**, so it can't be done headlessly — the first login must
happen in a browser. After that, the **refresh token renews the session headlessly, forever**
(`POST /api/auth/tokens/ {refresh_token}` needs no captcha, and rotates the token). Verified.

**Recommended for automation — one browser login, then headless auto-refresh:**
1. Log in once at tienda.mercadona.es. In DevTools, grab the **`refresh_token`** from the
   `POST /api/auth/tokens/` response (Network) or from local storage.
2. Put it in `~/.mercadona/config.toml` (0600):
   ```toml
   [auth]
   refresh_token = "<your refresh token>"   # the durable, headless-renewable credential
   [defaults]
   warehouse = "mad1"
   ```
3. Done. On every `401 token_not_valid` the CLI refreshes and retries automatically — no
   browser, no captcha, unattended. (`MERCADONA_TOKEN`/`MERCADONA_COOKIE`/`MERCADONA_CUSTOMER`
   env vars also work for one-off runs.)

**Quick one-off (no refresh):** `mercadona import-curl --file s.txt` from a DevTools
"Copy as cURL" of any `…/api/customers/…` request extracts the Bearer token + cookie + customer
id. It has no refresh token, so it can't auto-renew — re-import (or re-seed the refresh token)
when the access token expires.

> `mercadona login --user … --password …` exists but will fail without a `recaptcha_token`
> (browser-only); prefer the refresh-token flow above for automation.

The customer id is read automatically from the token's `customer_uuid` claim, so
you never pass it (the literal `me` alias is rejected with 403). Token/cookie/
customer can also come from `MERCADONA_TOKEN` / `MERCADONA_COOKIE` /
`MERCADONA_CUSTOMER`. Secrets are read from env/files, never taken as flags.

#### Test the checkout flow with your session

```bash
mercadona import-curl --file session.txt   # from DevTools "Copy as cURL"
mercadona whoami                           # → "ok — customer id=…"  (confirms auth)
mercadona cart get --json                  # inspect current cart
mercadona cart add 51110 2                 # add 2× a product
mercadona checkout create --json           # open checkout → returns id + delivery slots
mercadona checkout set-delivery --checkout <id> --address <id> --slot <id>
mercadona checkout submit   --checkout <id> --yes   # IRREVERSIBLE — places the order
```

The access token (a SimpleJWT) lasts ~6 weeks; when `whoami` starts returning
`401 token_not_valid`, re-import a fresh `Copy as cURL` (or use `login`).

## Design / reliability

Three layers, by IP-sensitivity:

1. **Search → Algolia.** Not behind Mercadona's Akamai at all; works from any IP.
   The public app-id **rotates** (`7UZJKL1GNI` → `7UZJKL1DJ0` …), so the CLI never
   relies on a hardcoded value: it ships a last-known-good fallback and, on a stale-creds
   signal (DNS failure / 401 / 403 / 404), **re-discovers the app-id, key and index from
   the live SPA bundle**, caches them, and retries. Survives rotation automatically.
2. **Catalog reads (`/api/...`)** — Akamai-fronted but served to anonymous GETs at
   human-paced volume. Sends web-app-like headers to stay in monitor mode.
3. **Auth + cart + checkout** — the only IP-sensitive part. Run from a **residential IP**
   (local, or a box on your own network — not a flagged datacenter / serverless egress),
   log in once and cache the token. A real browser is only needed as a *fallback* to mint
   Akamai clearance if a challenge ever appears.

## Config

State lives under the OS config dir (`~/Library/Application Support/mercadona` on macOS,
`~/.config/mercadona` elsewhere). Override with `MERCADONA_CONFIG_DIR`.

- `algolia.json` — cached/refreshed search credentials.
- `token.json` — cached bearer token (added with the auth commands).

## Claude skill

This repo bundles a Claude Code skill, **`mercadona-shop`** (`.claude/skills/mercadona-shop/`),
that drives this CLI to do the grocery shop: turn a list into priced products, fill the cart,
prepare delivery checkout, and place the order only on explicit user consent. Install it where
your Claude reads skills (symlink or copy `.claude/skills/mercadona-shop` into `~/.claude/skills/`);
it points back at this binary, so build the CLI first.

## Status

Read core (`search`, `batch`, `product`, `categories`) + authenticated leg
(`login`, `import-curl`, `whoami`, `cart`, `checkout`) implemented. Reads, Algolia
self-refresh, uTLS fingerprint, and the auth plumbing are verified live; the
cart/checkout bodies await a real-session run (`import-curl` → `whoami` → `cart get`).
`checkout submit` is gated behind `--yes`.
