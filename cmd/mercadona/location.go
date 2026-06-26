package main

import (
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/ivorjpc/mercadona/internal/config"
)

var postalCodeRE = regexp.MustCompile(`^\d{5}$`)

// set-postal resolves a postal code to its Mercadona warehouse and saves both to
// ~/.mercadona/config.toml [defaults]. From then on search/cart/checkout default
// to that warehouse, so prices and product ids match what the user's address
// actually gets (and online checkout won't reject a warehouse/address mismatch).
// Resolution is anonymous, so this works — and is worth running — even before login.
func cmdSetPostal(args []string) error {
	fs := flag.NewFlagSet("set-postal", flag.ExitOnError)
	cf := addCommon(fs)
	_ = fs.Parse(reorderArgs(fs, args))
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: mercadona set-postal <postal_code>   (e.g. mercadona set-postal 28022)")
	}
	pc := strings.TrimSpace(fs.Arg(0))
	if !postalCodeRE.MatchString(pc) {
		return fmt.Errorf("invalid postal code %q — expected 5 digits, e.g. 28022", pc)
	}
	wh, err := newClient(cf).ResolveWarehouse(pc)
	if err != nil {
		return err
	}
	cfg, _ := config.LoadConfig()
	cfg.Defaults.PostalCode = pc
	cfg.Defaults.Warehouse = wh
	if err := config.SaveConfig(cfg); err != nil {
		return err
	}
	if cf.jsonOut {
		return emitJSON(map[string]string{"postal_code": pc, "warehouse": wh})
	}
	fmt.Printf("ok — postal code %s → warehouse %s (saved to ~/.mercadona/config.toml)\n", pc, wh)
	fmt.Fprintln(os.Stderr, "→ search, cart and checkout now default to this warehouse (override per-command with --wh).")
	return nil
}
