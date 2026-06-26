package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/ivorjpc/mercadona/internal/client"
	"github.com/ivorjpc/mercadona/internal/config"
)

// ---- authenticated commands ----

// authedClient builds a client whose auth comes from, in precedence order:
// env vars (MERCADONA_TOKEN/COOKIE/CUSTOMER/USER/PASS), then ~/.mercadona/config.toml,
// then the cached session written by `login`/`import-curl`. If there's no access
// token but there are credentials (or a refresh token), it logs in / refreshes.
func authedClient(cf *common) (*client.Client, error) {
	cl := newClient(cf)
	cl.LoadToken() // cached session: access + refresh + cookie (best-effort)

	cfg, _ := config.LoadConfig() // ~/.mercadona/config.toml (missing = empty)
	if cfg.Auth.Username != "" {
		cl.Username = cfg.Auth.Username
	}
	if cfg.Auth.Password != "" {
		cl.Password = cfg.Auth.Password
	}
	if cfg.Auth.RefreshToken != "" && cl.RefreshToken == "" {
		cl.RefreshToken = cfg.Auth.RefreshToken
	}
	if u := os.Getenv("MERCADONA_USER"); u != "" {
		cl.Username = u
	}
	if p := os.Getenv("MERCADONA_PASS"); p != "" {
		cl.Password = p
	}
	if t := os.Getenv("MERCADONA_TOKEN"); t != "" {
		cl.Token = t
	}
	if ck := os.Getenv("MERCADONA_COOKIE"); ck != "" {
		cl.Cookie = ck
	}
	if cid := os.Getenv("MERCADONA_CUSTOMER"); cid != "" {
		cl.CustomerID = cid
	}

	if cl.Token == "" {
		if !cl.CanReauth() {
			if cl.Cookie != "" {
				return nil, fmt.Errorf("a cookie alone does not authenticate the API — you also need a Bearer token or credentials")
			}
			return nil, fmt.Errorf("not authenticated: `mercadona login` (and --save), set MERCADONA_USER/PASS or [auth] in ~/.mercadona/config.toml, or `mercadona import-curl`")
		}
		if err := cl.EnsureToken(); err != nil { // refresh, else login from creds
			return nil, fmt.Errorf("login/refresh failed: %w", err)
		}
	}
	if err := cl.EnsureCustomer(); err != nil {
		return nil, err
	}
	return cl, nil
}

func cmdWhoami(args []string) error {
	fs := flag.NewFlagSet("whoami", flag.ExitOnError)
	cf := addCommon(fs)
	_ = fs.Parse(reorderArgs(fs, args))
	cl, err := authedClient(cf)
	if err != nil {
		return err
	}
	cu, raw, err := cl.Me()
	if err != nil {
		return err
	}
	if cf.jsonOut {
		return emitRaw(raw)
	}
	fmt.Printf("ok — authenticated. customer id=%s\n", cu.Resolve())
	return nil
}

// import-curl extraction patterns (DevTools → Copy as cURL).
var (
	reBearer   = regexp.MustCompile(`(?i)authorization:\s*bearer\s+([A-Za-z0-9._~+/=\-]+)`)
	reCookieB  = regexp.MustCompile(`(?:^|\s)(?:-b|--cookie)\s+'([^']*)'`)
	reCookieH  = regexp.MustCompile(`(?i)(?:-H|--header)\s+'cookie:\s*([^']*)'`)
	reCustomer = regexp.MustCompile(`/api/customers/([^/'"\s]+)/`)
)

func cmdImportCurl(args []string) error {
	fs := flag.NewFlagSet("import-curl", flag.ExitOnError)
	file := fs.String("file", "-", "file with a DevTools 'Copy as cURL' command ('-' = stdin)")
	_ = fs.Parse(reorderArgs(fs, args))
	var (
		data []byte
		err  error
	)
	if *file == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(*file)
	}
	if err != nil {
		return err
	}
	s := string(data)
	token := firstSubmatch(reBearer, s)
	cookie := firstSubmatch(reCookieB, s)
	if cookie == "" {
		cookie = strings.TrimSpace(firstSubmatch(reCookieH, s))
	}
	customer := firstSubmatch(reCustomer, s)
	if customer == "me" {
		customer = "" // let the "me" default handle it
	}
	if token == "" && cookie == "" {
		return fmt.Errorf("no 'authorization: Bearer ...' or cookie found in the input")
	}
	if err := client.SaveSession(token, cookie, customer); err != nil {
		return err
	}
	// Never echo secrets — report only lengths + the (non-secret) customer id.
	fmt.Fprintf(os.Stderr, "imported session: token=%s, cookie=%s, customer=%s\n",
		present(token), present(cookie), orDefault(customer, "me"))
	fmt.Fprintln(os.Stderr, "→ verify with: mercadona whoami")
	return nil
}

// import-har extracts a session from a browser HAR export (DevTools → Network →
// "Export HAR…"). Unlike import-curl it also captures the durable refresh token
// and, by default, seeds it into ~/.mercadona/config.toml so the CLI re-auths
// headlessly forever — works for both email/password and Google-login accounts.
func cmdImportHar(args []string) error {
	fs := flag.NewFlagSet("import-har", flag.ExitOnError)
	file := fs.String("file", "", "path to a .har file ('-' or omitted = stdin; may also be passed positionally)")
	save := fs.Bool("save", true, "seed the refresh token into ~/.mercadona/config.toml for headless auto-renew")
	_ = fs.Parse(reorderArgs(fs, args))

	src := *file
	if src == "" && fs.NArg() == 1 {
		src = fs.Arg(0)
	}
	var (
		data []byte
		err  error
	)
	if src == "" || src == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(src)
	}
	if err != nil {
		return err
	}

	sess, err := client.ParseHAR(data)
	if err != nil {
		return err
	}
	if err := client.SaveHARSession(sess); err != nil {
		return err
	}

	seeded := false
	if *save {
		cfg, _ := config.LoadConfig()
		changed := false
		if sess.RefreshToken != "" {
			cfg.Auth.RefreshToken = sess.RefreshToken
			seeded, changed = true, true
		}
		// The session's wh is the warehouse Mercadona assigned to this account's
		// address — authoritative, so adopt it as the default. Lang is only a
		// UI preference, so don't clobber an explicit one already in config.
		if sess.Warehouse != "" {
			cfg.Defaults.Warehouse = sess.Warehouse
			changed = true
		}
		if sess.Lang != "" && cfg.Defaults.Lang == "" {
			cfg.Defaults.Lang = sess.Lang
			changed = true
		}
		if changed {
			if err := config.SaveConfig(cfg); err != nil {
				return fmt.Errorf("session cached but writing config.toml failed: %w", err)
			}
		}
	}

	// Never echo secrets — report only lengths + the (non-secret) customer id.
	fmt.Fprintf(os.Stderr, "imported HAR session (%s login): access=%s, refresh=%s, cookie=%s, customer=%s\n",
		orDefault(sess.LoginKind, "unknown"), present(sess.AccessToken), present(sess.RefreshToken),
		present(sess.Cookie), orDefault(sess.CustomerID, "(from JWT)"))
	switch {
	case seeded:
		fmt.Fprintln(os.Stderr, "→ refresh token seeded into ~/.mercadona/config.toml (0600): the CLI now auto-renews headlessly — always authenticated.")
	case sess.RefreshToken == "":
		fmt.Fprintln(os.Stderr, "⚠ no refresh token in this HAR — cached the access token only (expires ~6 wk, no auto-renew). Re-export a HAR that includes the login response, or use `set-refresh`.")
	}
	if sess.Warehouse != "" {
		if *save {
			fmt.Fprintf(os.Stderr, "→ warehouse %s detected from the HAR and saved as your default (override with --wh).\n", sess.Warehouse)
		} else {
			fmt.Fprintf(os.Stderr, "→ warehouse %s detected from the HAR (not saved: --save=false).\n", sess.Warehouse)
		}
	}
	fmt.Fprintln(os.Stderr, "→ verify with: mercadona whoami")
	return nil
}

func firstSubmatch(re *regexp.Regexp, s string) string {
	if m := re.FindStringSubmatch(s); m != nil {
		return m[1]
	}
	return ""
}

func present(s string) string {
	if s == "" {
		return "(none)"
	}
	return fmt.Sprintf("%d chars", len(s))
}

func orDefault(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

func cmdLogin(args []string) error {
	fs := flag.NewFlagSet("login", flag.ExitOnError)
	cf := addCommon(fs)
	user := fs.String("user", "", "username/email (or env MERCADONA_USER, or config.toml)")
	pass := fs.String("password", "", "password (prefer --password-stdin / env / config)")
	passAlias := fs.String("pass", "", "alias of --password")
	passStdin := fs.Bool("password-stdin", false, "read the password from stdin")
	save := fs.Bool("save", false, "save credentials to ~/.mercadona/config.toml for auto-relogin")
	_ = fs.Parse(reorderArgs(fs, args))

	cfg, _ := config.LoadConfig()
	user2 := firstNonEmpty(*user, os.Getenv("MERCADONA_USER"), cfg.Auth.Username)
	if user2 == "" {
		return fmt.Errorf("missing username: pass --user, set MERCADONA_USER, or [auth].username in config.toml")
	}
	pass2 := firstNonEmpty(*pass, *passAlias, os.Getenv("MERCADONA_PASS"), cfg.Auth.Password)
	if *passStdin {
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		pass2 = strings.TrimRight(string(b), "\r\n")
	}
	if pass2 == "" {
		return fmt.Errorf("missing password: --password, --password-stdin, MERCADONA_PASS, or config.toml")
	}

	cl := newClient(cf)
	tok, err := cl.Login(user2, pass2)
	if err != nil {
		return err
	}
	if *save {
		cfg.Auth.Username = user2
		cfg.Auth.Password = pass2
		if err := config.SaveConfig(cfg); err != nil {
			return fmt.Errorf("login ok but saving config.toml failed: %w", err)
		}
		fmt.Fprintln(os.Stderr, "credentials saved to ~/.mercadona/config.toml (0600); future runs auto-relogin")
	}
	if cf.jsonOut {
		return emitJSON(map[string]string{"status": "ok", "customer_id": tok.CustomerID.String()})
	}
	fmt.Printf("logged in — customer_id=%s (session cached)\n", tok.CustomerID.String())
	return nil
}

// set-refresh seeds the durable refresh token into ~/.mercadona/config.toml so
// the CLI can renew the session headlessly. Get the token from one browser login
// (DevTools → the POST /api/auth/tokens/ response, or local storage).
func cmdSetRefresh(args []string) error {
	fs := flag.NewFlagSet("set-refresh", flag.ExitOnError)
	stdin := fs.Bool("stdin", false, "read the refresh token from stdin (keeps it out of argv)")
	_ = fs.Parse(reorderArgs(fs, args))
	var rt string
	switch {
	case *stdin:
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		rt = strings.TrimSpace(string(b))
	case fs.NArg() == 1:
		rt = strings.TrimSpace(fs.Arg(0))
	default:
		return fmt.Errorf("usage: mercadona set-refresh <refresh_token>   (or: ... set-refresh --stdin)")
	}
	if rt == "" {
		return fmt.Errorf("empty refresh token")
	}
	cfg, _ := config.LoadConfig()
	cfg.Auth.RefreshToken = rt
	if err := config.SaveConfig(cfg); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "refresh token saved to ~/.mercadona/config.toml (%d chars, 0600).\n", len(rt))
	fmt.Fprintln(os.Stderr, "→ the CLI now auto-renews headlessly. test: mercadona whoami")
	return nil
}
