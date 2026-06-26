---
name: mercadona-shop
description: >-
  Do the grocery shop at Mercadona (tienda.mercadona.es) by driving the local `mercadona`
  CLI: turn a shopping list into real priced products, fill the cart, prepare delivery
  checkout, and (only on explicit go-ahead) place the order. Use this whenever the user
  wants to shop, price a grocery list, build a cart, or check out at Mercadona — including
  Spanish phrasings like "hazme la compra", "haz la compra de Mercadona", "compra en
  Mercadona", "añade X al carrito", "pídeme estos productos", "¿cuánto cuesta esta lista?",
  and English ones like "do my Mercadona shop", "price this grocery list at Mercadona",
  "fill my Mercadona cart", "order these groceries". Trigger even when the user just pastes
  a list of groceries and mentions Mercadona without saying "skill" or "CLI". Always confirm
  the resolved products before touching the cart, and never place the order without explicit
  consent. Do NOT use for other supermarkets (Carrefour, Lidl, Amazon) — this is Mercadona-only.
---

# Mercadona shop

Turn a shopping list into a real Mercadona order by driving the `mercadona` CLI — search
prices, fill the cart, set delivery, and place the order. The CLI talks to the same HTTP
endpoints `tienda.mercadona.es` uses; this skill is the playbook for using it safely on a
real account.

## The two safety gates (read first)

This spends real money on a real account, so two points are non-negotiable:

1. **Confirm before touching the cart.** Reading and pricing are free and side-effect-free —
   do them freely. But before the *first* `cart add`/`cart set`, show the user the resolved
   plan (each list item → the exact product you picked, with id, name, size and price) and
   wait for an explicit OK. Product matching is fuzzy (see below), so this is where the user
   catches a wrong brand or size — cheap to fix now, annoying to fix after the cart is full.
2. **Never submit without explicit consent.** `checkout submit` is irreversible and places a
   paid order. Run it only when the user has clearly said "yes, place it" (or "dale") *for
   this specific order*, after you've shown them the full total. The CLI also requires `--yes`
   as a hard backstop — never add `--yes` on the user's behalf to "save a step".

Everything else (search, batch pricing, `cart get`, `checkout create`, `checkout slots`,
`set-delivery`) is reversible and fine to run as you work toward those two checkpoints.

### Spending cap (defense in depth)

On top of the two gates, put a hard euro ceiling on every cart/checkout command with `--max <eur>`,
so a wrong product match or a fat-fingered quantity can't quietly run up a huge order. Choose the cap:

- the user's stated budget if they gave one ("máximo 80€", "no más de 100"); otherwise
- the agreed plan total from Gate 1, rounded up with a small margin.

Any `cart add/set` or `checkout create/set-delivery/submit` whose total would exceed `--max` fails
with `error: BUDGET EXCEEDED …` and a non-zero exit, so you stop instead of overspending. `submit`
fails **closed**: with a cap set, if it can't read the order total it refuses rather than pay. Treat a
budget error as **stop-and-report** — only raise the cap if the user explicitly raises the budget.
(You can also set it once for the run: `MERCADONA_MAX_EUR`, or `[limits].max_eur` in
`~/.mercadona/config.toml`; precedence is flag > env > config.)

## Locate the binary

Run `mercadona` from `PATH`. If it isn't installed, install it (npm `@ivorpad/mercadona`, the
`curl | sh` script, or a GitHub release — see the project README) and confirm with `mercadona
version`. Never hardcode a local path or a build-from-source step: this skill ships to every
CLI user, not one machine.

Pin the warehouse to the user's area — ids and prices are **per-warehouse**. Ask their postal
code and run `mercadona set-postal <cp>` once: it resolves and saves the right warehouse as the
default (no login needed). Without it the default is `mad1` (Madrid); you can still override
per-command with `--wh`/`--lang` (e.g. `--wh bcn1`). Always price and shop in the warehouse
that will actually deliver.

## Authenticate (bring-your-own credentials)

Reads/search need no login. Cart and checkout need a Bearer token. The catch: **password
login requires a Google reCAPTCHA Enterprise token**, so it can't be done headlessly — the
*first* login has to happen in a real browser. The good news: the **refresh token renews the
session headlessly, forever** (the refresh call needs no captcha and rotates the token). So the
durable setup is one browser login, then the CLI runs unattended.

**Durable path (recommended) — one browser login, then headless forever:**
1. The user logs in once at `tienda.mercadona.es` in their **own local browser** (not a
   datacenter/Cloud browser — reCAPTCHA Enterprise scores datacenter IPs low and returns 412).
   Either method works — email/password or "Sign in with Google".
2. Easiest: DevTools → Network → **Export HAR…**, then let the CLI extract the refresh token:
   ```bash
   mercadona import-har --file tienda.mercadona.es.har
   ```
   It pulls the `refresh_token` (works for both login methods) into `~/.mercadona/config.toml` and
   caches the session — never reading the password. (Manual alternative: grab the `refresh_token`
   from the `POST /api/auth/tokens/` response or local storage and seed it with
   `printf '%s' '<refresh_token>' | mercadona set-refresh --stdin`.)
3. From then on every authenticated call auto-refreshes on a `401 token_not_valid` and retries —
   no browser, no captcha.

**Quick one-off — import a browser session:** in a logged-in tab, DevTools → Network → any
`…/api/customers/…` request → Copy → Copy as cURL → save to a file →
`mercadona import-curl --file session.txt`. Extracts the access token + cookie + customer id.
It carries **no refresh token**, so it can't auto-renew — fine for a quick test (~6-week token),
but seed a refresh token for anything ongoing.

Don't reach for `mercadona login --user/--password` in automation — it posts without the
reCAPTCHA token and will fail; it exists only as a browser-assisted fallback.

Verify either way with `mercadona whoami` → `ok — authenticated. customer id=…`. On
`not authenticated` or `401 token_not_valid`, see `references/troubleshooting.md`.

**Never echo secrets.** Feed tokens/passwords via stdin or env, never as flags/argv; don't
print tokens or cookies back to the user.

## The shopping workflow

### 0. Scope the shop before pricing (ask first)

Don't assume — get the two things that decide the basket before resolving anything:

- **For how many people?** A recipe scales: "paella" for 2 is not the same shop as for 6. Ask
  (e.g. "¿para cuántas personas?") and size every quantity from the answer; never default silently
  to a number.
- **The list itself.** Accept a typed list *or a photo* of one (read it, then confirm what you
  parsed). If the user only names a dish, propose an ingredient list for their headcount and let
  them adjust it before you price.

Treat the resolved plan as editable — users routinely tweak brand, size or quantity. Loop on that
until they're happy, *before* any cart action.

### 1. Resolve the list to real products (free, no login)

Price the whole list in one request with `batch` — one term per line, ~100 items per call:

```bash
printf 'leche entera\nhuevos\nplátano\naceite de oliva\n' | mercadona batch -f -
```

`batch` returns the **top hit per term**, which is fast but only a rough match — Algolia
relevance on a bare word is loose (a search for "carne" can top-rank a pepper paste). So:

- For anything the user was specific about (brand, size, "el barato", "sin lactosa") or where
  the top hit looks off, run `mercadona search "<term>" --limit 5` and pick deliberately.
- When genuinely ambiguous, show the user 2-3 options and let them choose rather than guessing.
- Use `--json` when you need to parse ids/prices reliably; the default output is for humans.

Note quantities: scale them to the headcount from step 0. Many items are by unit, but weight/bulk
items (fruit, deli) price per kg and accept fractional quantity (e.g. `0.5`).

### 2. Present the plan and get the OK (Gate 1)

Before touching the cart, show a compact table: each list item → chosen product (`[id] name —
size — unit price`) and a line quantity. **Compute the total with code, never by hand** — feed the
resolved `<id> <qty>` lines to `mercadona total` and quote its figure (it sums `unit_price × qty`
deterministically, in the right warehouse):

```bash
printf '5044 1\n2779 2\n24181 1\n' | mercadona total -f -   # → per-line subtotals + total
```

Call out anything you guessed. Then wait for the user's go-ahead — and if they want changes, edit
the plan and re-run `total`. This is the confirm-before-cart gate. (The cart/checkout API total is
the authoritative one once items are in the cart; `total` is the pre-cart estimate.)

### 3. Fill the cart (after the OK)

```bash
mercadona cart add 10381 1 --max 80   # add 1× product 10381 (fails if cart total would exceed 80€)
mercadona cart set 30167 2 --max 80   # set absolute qty (0 removes the line)
mercadona cart get                    # verify: lines + total
```

Pass `--max <budget>` on every mutating command (see *Spending cap* above) — the cap is your
backstop against a fuzzy match or wrong quantity ballooning the order.

`cart add` is additive; `cart set` is absolute. After filling, run `cart get` and reconcile
the line set + total against your plan before moving on.

**Write one line at a time and verify before the next.** Each `cart add`/`set` is a
read-modify-write, and the cart PUT carries no version — so two writes fired back-to-back can
both read the *same* pre-change cart (the backend read can lag a just-made write) and the
second silently clobbers the first. Don't batch-fire mutations; issue one, confirm it landed
with `cart get`, then issue the next. The verifying `cart get` also serializes them past that
lag. (Verified live: two chained `set … 0` removals raced and one item reappeared.)

### 4. Prepare delivery checkout (reversible)

```bash
mercadona checkout create --json --max 80   # → checkout id + default address (embedded)
mercadona checkout slots --address <id> --json   # delivery slots for that address
mercadona checkout set-delivery --checkout <chk> --address <addr> --slot <slot> --max 80
```

`checkout create` returns the checkout `id` and the default delivery address (slots are NOT
in this response — they hang off the address). Fetch slots with `checkout slots --address
<id>`, pick one the user wants (each slot has `start`/`end`/`price`/`available`), then attach
it with `set-delivery`. The response carries the final price breakdown.

The **minimum order is 60€** (enforced live; varies a little by warehouse) and delivery adds
a fee (~8.20€). If the cart subtotal is under the minimum, tell the user and offer to add
items rather than letting checkout fail.

### 5. Place the order (Gate 2 — explicit consent only)

Show the final order: line items, subtotal, delivery fee, total, and the chosen slot. Only if
the user explicitly confirms *this* order:

```bash
mercadona checkout submit --checkout <chk> --max 80 --yes   # IRREVERSIBLE — places the order (refuses if total > 80€)
```

If you have any doubt about consent, stop and ask. A prepared-but-unsubmitted checkout is
harmless and can sit until the user decides; a placed order cannot be taken back here.

## Reference material

- `references/cli-reference.md` — every command, its flags, and the exact JSON/field shapes
  (cart lines, checkout, slots). Read it when you need to parse `--json` output precisely.
- `references/troubleshooting.md` — auth expiry, Akamai 403/challenge, warehouse/postal
  mismatch, "no results", minimum-purchase and IP-sensitivity notes.

## Why no all-in-one script

This skill drives the CLI step by step instead of a single "shop it all" wrapper on purpose:
the two safety gates need a human in the loop, and product matching needs judgment. Keep the
control flow here, in the conversation, where the user can steer it.
