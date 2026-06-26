# `mercadona` CLI reference

Exact command surface and output shapes. Data goes to **stdout**, logs/errors to **stderr**,
exit code `1` on error — safe to pipe and parse. Add `--json` to any command for machine
output. Common flags go right after the (sub)command: `--wh mad1`, `--lang es`, `--json`.

## Read commands (anonymous — no login)

| Command | Purpose |
| --- | --- |
| `mercadona search <term...> [--limit N]` | full-text product search (Algolia) |
| `mercadona batch [-f file] [--hits N]` | search many terms in ONE request (~100/call) |
| `mercadona product <id>` | product detail + price |
| `mercadona categories [--id N]` | category tree, or one category's products |

`batch` reads one term per line from `-f <file>` (`-f -` = stdin) or positional args. Lines
starting with `#` are skipped. Default returns the top hit per term (`--hits` for more).

### Search hit shape (`--json`)

```json
{
  "query": "leche entera",
  "nbHits": 15,
  "hits": [
    {
      "id": "10379",
      "display_name": "Leche entera Hacendado",
      "packaging": "Pack-6",
      "share_url": "https://tienda.mercadona.es/product/10379/...",
      "categories": [{ "name": "Huevos, leche y mantequilla" }],
      "price_instructions": {
        "unit_price": "5.76",
        "reference_price": "0.960",
        "reference_format": "L",
        "is_pack": true
      }
    }
  ]
}
```

`id` is the product id you pass to `cart add`/`set`. `unit_price` is the price you pay for the
item as sold; `reference_price`/`reference_format` is the per-unit comparison (e.g. 0.960 €/L).
Ids are **per-warehouse** — an id from a different `--wh` may 404.

## Auth commands (bring-your-own credentials)

| Command | Purpose |
| --- | --- |
| `mercadona set-refresh <token>` (or `--stdin`) | seed a refresh token → headless auto-refresh forever |
| `mercadona import-curl [--file f]` | import a browser session (access token + cookie); **no** refresh |
| `mercadona login [--user E] [--password-stdin] [--save]` | password login — **needs a browser reCAPTCHA**, fails headless |
| `mercadona whoami` | verify the session (prints customer id) |

Password login requires a Google reCAPTCHA Enterprise token, so it can't be done headlessly —
the first login is a browser step. The **refresh token** then renews the session with no
captcha (it rotates on each use), which is the durable, unattended credential.

**set-refresh** writes `[auth] refresh_token` to `~/.mercadona/config.toml` (0600). Get the
token from one browser login (DevTools → the `POST /api/auth/tokens/` response, or local
storage). Use `--stdin` to keep it out of argv. After seeding, any authed call auto-refreshes
on a `401 token_not_valid` and retries.

**import-curl** extracts the Bearer access token, cookie, and customer id from a DevTools
"Copy as cURL" (`--file`, or stdin); prints only lengths + the customer id, never the secrets.
It has **no** refresh token, so it can't auto-renew — the access token lasts ~6 weeks.

**login** posts `{username, password}` but the API also wants a `recaptcha_token` it can't
generate, so headless login fails (412/400). Present only as a browser-assisted fallback;
prefer set-refresh / import-curl.

Auth precedence: flags > env (`MERCADONA_TOKEN`/`COOKIE`/`CUSTOMER`/`USER`/`PASS`) >
`~/.mercadona/config.toml` > cached session (`token.json`). The customer id is decoded from
the token's `customer_uuid` claim — the literal `me` alias is rejected (403).

## Cart commands (auth required)

| Command | Purpose |
| --- | --- |
| `mercadona cart get` | show current cart |
| `mercadona cart add <product_id> <qty>` | add qty to a product's existing quantity |
| `mercadona cart set <product_id> <qty>` | set absolute qty (`0` removes the line) |

`qty` is a float — unit items use `1`, `2`…; weight/bulk items accept fractions (`0.5`).

### Cart shape (`cart get --json`)

Read (GET) nests the product under each line; the CLI normalizes to a flat product id:

```json
{
  "id": "<cart-id>",
  "version": 7,
  "products_count": 3,
  "lines": [
    { "quantity": 2.0, "product": { "id": "10379" } }
  ],
  "summary": { "total": "12.34" }
}
```

`quantity` is a float (the API returns `1.0`). `cart add`/`set` echo the raw PUT response.

## Checkout commands (auth required)

| Command | Purpose |
| --- | --- |
| `mercadona checkout addresses` | list saved delivery addresses |
| `mercadona checkout create` | open a checkout from the cart → id + default address |
| `mercadona checkout slots --address <id>` | delivery slots for an address |
| `mercadona checkout set-delivery --checkout <id> --address <id> --slot <id>` | attach address + slot |
| `mercadona checkout submit --checkout <id> --yes` | **place the order (irreversible)** |

### Flow + shapes

1. `checkout create --json` → `{ "id": "<chk>", "address": { "id": <addr>, ... }, ... }`.
   The default address is embedded; **slots are not** — they hang off the address.
2. `checkout slots --address <addr> --json` →
   `{ "next_page": ..., "results": [ { "id": "<slot>", "start": "...", "end": "...",
   "price": "...", "available": true, "open": true } ] }`.
   Slot `id` is a **string**; address `id` is an **int**.
3. `checkout set-delivery --checkout <chk> --address <addr> --slot <slot>` → reserves the slot;
   the response carries the final price breakdown (subtotal + delivery fee → total).
4. `checkout submit --checkout <chk> --yes` → places the order. Without `--yes` the CLI refuses.
   The **minimum order is 60€** (enforced live; bundle's "50" is wrong); delivery ≈ 8.20€.

## Config & state

State lives in `~/.mercadona/` (override with `MERCADONA_CONFIG_DIR`):

- `config.toml` — `[auth]` username/password/refresh_token, `[defaults]` warehouse/lang (0600).
  Only `refresh_token` actually yields a headless session; stored `username`/`password` can't
  auto-login (the login call omits the reCAPTCHA token the API requires → 412/400).
- `token.json` — cached bearer + refresh + cookie (machine-managed).
- `algolia.json` — cached/auto-refreshed search creds.
