package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ivorjpc/mercadona/internal/client"
)

func cmdSearch(args []string) error {
	fs := flag.NewFlagSet("search", flag.ExitOnError)
	cf := addCommon(fs)
	limit := fs.Int("limit", 24, "max results")
	_ = fs.Parse(reorderArgs(fs, args))
	if fs.NArg() == 0 {
		return fmt.Errorf("usage: mercadona search [flags] <term...>")
	}
	cl := newClient(cf)
	res, err := cl.Search(strings.Join(fs.Args(), " "), *limit)
	if err != nil {
		return err
	}
	if cf.jsonOut {
		return emitJSON(res)
	}
	fmt.Printf("query=%q  nbHits=%d  (index=%s)\n", res.Query, res.NbHits, cl.IndexName())
	for _, h := range res.Hits {
		printHit("  ", h)
	}
	return nil
}

func cmdBatch(args []string) error {
	fs := flag.NewFlagSet("batch", flag.ExitOnError)
	cf := addCommon(fs)
	file := fs.String("f", "", "file with one term per line ('-' for stdin); else terms are positional args")
	hits := fs.Int("hits", 1, "results per term")
	_ = fs.Parse(reorderArgs(fs, args))
	terms, err := collectTerms(*file, fs.Args())
	if err != nil {
		return err
	}
	if len(terms) == 0 {
		return fmt.Errorf("no terms given (use -f file, stdin, or positional args)")
	}
	cl := newClient(cf)
	results, err := cl.Batch(terms, *hits)
	if err != nil {
		return err
	}
	if cf.jsonOut {
		return emitJSON(results)
	}
	for i, r := range results {
		term := r.Query
		if term == "" && i < len(terms) {
			term = terms[i]
		}
		if len(r.Hits) == 0 {
			fmt.Printf("• %-24s → (sin resultados)\n", term)
			continue
		}
		h := r.Hits[0]
		fmt.Printf("• %-24s → [%s] %s — %s€ %s\n", term, h.ID, h.DisplayName, h.Price.UnitPrice, refFormat(h.Price))
	}
	return nil
}

func cmdProduct(args []string) error {
	fs := flag.NewFlagSet("product", flag.ExitOnError)
	cf := addCommon(fs)
	_ = fs.Parse(reorderArgs(fs, args))
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: mercadona product [flags] <id>")
	}
	cl := newClient(cf)
	pv, raw, err := cl.Product(fs.Arg(0))
	if err != nil {
		return err
	}
	if cf.jsonOut {
		return emitRaw(raw)
	}
	fmt.Printf("[%s] %s\n", pv.ID, pv.DisplayName)
	fmt.Printf("  precio: %s€  (%s %s)\n", pv.Price.UnitPrice, pv.Price.ReferencePrice, pv.Price.ReferenceFormat)
	if pv.Packaging != "" {
		fmt.Printf("  formato: %s\n", pv.Packaging)
	}
	if pv.ShareURL != "" {
		fmt.Printf("  url: %s\n", pv.ShareURL)
	}
	return nil
}

func cmdCategories(args []string) error {
	fs := flag.NewFlagSet("categories", flag.ExitOnError)
	cf := addCommon(fs)
	id := fs.String("id", "", "fetch a single category (with products) by id")
	_ = fs.Parse(reorderArgs(fs, args))
	cl := newClient(cf)
	var raw json.RawMessage
	var err error
	if *id != "" {
		raw, err = cl.Category(*id)
	} else {
		raw, err = cl.Categories()
	}
	if err != nil {
		return err
	}
	if cf.jsonOut || *id != "" {
		return emitRaw(raw)
	}
	// compact human view of the top-level tree
	var tree struct {
		Results []struct {
			ID         int    `json:"id"`
			Name       string `json:"name"`
			Categories []struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
			} `json:"categories"`
		} `json:"results"`
	}
	if err := json.Unmarshal(raw, &tree); err != nil {
		return emitRaw(raw)
	}
	for _, top := range tree.Results {
		fmt.Printf("%d  %s\n", top.ID, top.Name)
		for _, sub := range top.Categories {
			fmt.Printf("    %d  %s\n", sub.ID, sub.Name)
		}
	}
	return nil
}

func printHit(indent string, h client.Hit) {
	cat := h.Category()
	if cat != "" {
		cat = "(" + cat + ")"
	}
	fmt.Printf("%s[%s] %s — %s€ %s %s\n", indent, h.ID, h.DisplayName, h.Price.UnitPrice, refFormat(h.Price), cat)
}

func refFormat(p client.PriceInstructions) string {
	if p.ReferencePrice == "" || p.ReferenceFormat == "" {
		return ""
	}
	return fmt.Sprintf("(%s€/%s)", p.ReferencePrice, p.ReferenceFormat)
}

func collectTerms(file string, posArgs []string) ([]string, error) {
	if file == "" {
		return posArgs, nil
	}
	var r io.Reader
	if file == "-" {
		r = os.Stdin
	} else {
		f, err := os.Open(file)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		r = f
	}
	var terms []string
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		if t := strings.TrimSpace(sc.Text()); t != "" && !strings.HasPrefix(t, "#") {
			terms = append(terms, t)
		}
	}
	return terms, sc.Err()
}
