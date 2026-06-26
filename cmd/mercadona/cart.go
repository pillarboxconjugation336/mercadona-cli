package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"strconv"
)

// fmtQty renders a quantity without a trailing ".0" (1.0 → "1", 0.5 → "0.5").
func fmtQty(q float64) string {
	return strconv.FormatFloat(q, 'f', -1, 64)
}

func cmdCart(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: mercadona cart <get|add|set> [flags] [args]")
	}
	sub, rest := args[0], args[1:]
	fs := flag.NewFlagSet("cart", flag.ExitOnError)
	cf := addCommon(fs)
	maxFlag := fs.Float64("max", 0, "refuse if the resulting cart total exceeds this many € (0 = env/config)")
	_ = fs.Parse(reorderArgs(fs, rest))
	cl, err := authedClient(cf)
	if err != nil {
		return err
	}
	switch sub {
	case "get":
		cart, raw, err := cl.GetCart()
		if err != nil {
			return err
		}
		if cf.jsonOut {
			return emitRaw(raw)
		}
		fmt.Printf("cart %s  (v%d, %d productos, total %s€)\n", cart.ID, cart.Version, cart.ProductsCount, cart.Summary.Total)
		for _, l := range cart.Lines {
			fmt.Printf("  %s× product %s\n", fmtQty(l.Quantity), l.ProductID)
		}
		return nil
	case "add", "set":
		a := fs.Args()
		if len(a) != 2 {
			return fmt.Errorf("usage: mercadona cart %s <product_id> <qty>", sub)
		}
		qty, err := strconv.ParseFloat(a[1], 64)
		if err != nil {
			return fmt.Errorf("invalid qty %q", a[1])
		}
		var raw json.RawMessage
		if sub == "add" {
			raw, err = cl.AddLine(a[0], qty)
		} else {
			raw, err = cl.SetLine(a[0], qty)
		}
		if err != nil {
			return err
		}
		if bErr := budgetCheckRaw(raw, *maxFlag, "cart"); bErr != nil {
			return bErr
		}
		return emitRaw(raw)
	default:
		return fmt.Errorf("unknown cart subcommand %q (get|add|set)", sub)
	}
}
