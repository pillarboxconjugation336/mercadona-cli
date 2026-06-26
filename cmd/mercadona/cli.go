package main

import (
	"encoding/json"
	"flag"
	"os"
	"strings"

	"github.com/ivorjpc/mercadona/internal/client"
	"github.com/ivorjpc/mercadona/internal/config"
)

// common flags shared by every subcommand.
type common struct {
	wh, lang string
	jsonOut  bool
}

func addCommon(fs *flag.FlagSet) *common {
	c := &common{}
	// Empty defaults so newClient can tell "user passed --wh" from "unset" and
	// fall back to config [defaults] before the built-in mad1/es.
	fs.StringVar(&c.wh, "wh", "", "warehouse code, e.g. mad1 (default: config [defaults].warehouse, else mad1)")
	fs.StringVar(&c.lang, "lang", "", "language: es, en, ca, eu, vai (default: config [defaults].lang, else es)")
	fs.BoolVar(&c.jsonOut, "json", false, "emit raw JSON to stdout")
	return c
}

// newClient builds a client, resolving warehouse/lang by precedence:
// explicit --wh/--lang flag > config.toml [defaults] > the built-in mad1/es from
// client.New(). Product ids and prices are per-warehouse, so honouring the
// config default (set once via `set-postal`) is what keeps every command on the
// catalog that serves the user's address.
func newClient(c *common) *client.Client {
	cl := client.New() // built-in defaults: warehouse mad1, lang es
	cfg, _ := config.LoadConfig()
	if v := firstNonEmpty(c.wh, cfg.Defaults.Warehouse); v != "" {
		cl.Warehouse = v
	}
	if v := firstNonEmpty(c.lang, cfg.Defaults.Lang); v != "" {
		cl.Lang = v
	}
	return cl
}

// reorderArgs lets flags appear anywhere among positional args. The stdlib flag
// parser stops at the first positional, so `cart add 123 5 --max 50` would
// silently drop --max (dangerous for a safety flag); this hoists flags (and their
// values) ahead of a `--` terminator so a normal fs.Parse sees them all. Honours
// bool flags (no value), `--flag=value`, and an explicit `--`.
func reorderArgs(fs *flag.FlagSet, args []string) []string {
	var flags, positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if len(a) > 1 && a[0] == '-' {
			flags = append(flags, a)
			name := strings.TrimLeft(a, "-")
			if strings.IndexByte(name, '=') >= 0 {
				continue // --flag=value: value is in the same token
			}
			if f := fs.Lookup(name); f != nil && !isBoolFlag(f) && i+1 < len(args) {
				flags = append(flags, args[i+1]) // consume this flag's value
				i++
			}
			continue
		}
		positional = append(positional, a)
	}
	out := make([]string, 0, len(flags)+1+len(positional))
	out = append(out, flags...)
	out = append(out, "--") // keep positionals positional even if they start with '-'
	return append(out, positional...)
}

func isBoolFlag(f *flag.Flag) bool {
	bf, ok := f.Value.(interface{ IsBoolFlag() bool })
	return ok && bf.IsBoolFlag()
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func emitJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

func emitRaw(raw json.RawMessage) error {
	var buf any
	if err := json.Unmarshal(raw, &buf); err != nil {
		_, err = os.Stdout.Write(raw)
		return err
	}
	return emitJSON(buf)
}
