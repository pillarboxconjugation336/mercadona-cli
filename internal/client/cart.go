package client

import (
	"encoding/json"
	"fmt"
)

// Cart mirrors GET /api/customers/<id>/cart/. Note the read and write shapes
// differ: GET nests the product object under each line; PUT wants a flat
// product_id (see CartLine's custom (Un)MarshalJSON).
type Cart struct {
	ID            string     `json:"id"`
	Version       int        `json:"version"`
	ProductsCount int        `json:"products_count"`
	Lines         []CartLine `json:"lines"`
	Summary       struct {
		Total string `json:"total"`
	} `json:"summary"`
}

// CartLine is one product row, normalized to a flat product id in both
// directions. Quantity is a float: the API returns it as 1.0, and weight/bulk
// products can be fractional.
type CartLine struct {
	ProductID string
	Quantity  float64
	Sources   []any
}

// UnmarshalJSON reads the GET shape, where the product id lives at product.id
// (and tolerates a flat product_id just in case).
func (l *CartLine) UnmarshalJSON(b []byte) error {
	var raw struct {
		Quantity  float64 `json:"quantity"`
		ProductID string  `json:"product_id"`
		Sources   []any   `json:"sources"`
		Product   struct {
			ID flexStr `json:"id"`
		} `json:"product"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	l.Quantity = raw.Quantity
	l.Sources = raw.Sources
	l.ProductID = raw.ProductID
	if l.ProductID == "" {
		l.ProductID = raw.Product.ID.String()
	}
	return nil
}

// MarshalJSON writes the PUT shape: a flat product_id with sources.
func (l CartLine) MarshalJSON() ([]byte, error) {
	sources := l.Sources
	if sources == nil {
		sources = []any{}
	}
	return json.Marshal(map[string]any{
		"quantity":   l.Quantity,
		"product_id": l.ProductID,
		"sources":    sources,
	})
}

func (c *Client) cartURL() string {
	return fmt.Sprintf("%s/api/customers/%s/cart/?lang=%s&wh=%s", c.BaseURL, c.CustomerID, c.Lang, c.Warehouse)
}

// GetCart returns the projected cart plus the full raw JSON.
func (c *Client) GetCart() (*Cart, json.RawMessage, error) {
	if err := c.EnsureCustomer(); err != nil {
		return nil, nil, err
	}
	var raw json.RawMessage
	if err := c.DoJSON("GET", c.cartURL(), nil, &raw); err != nil {
		return nil, nil, err
	}
	var cart Cart
	_ = json.Unmarshal(raw, &cart) // best-effort projection; raw is the source of truth
	return &cart, raw, nil
}

// PutCart writes the desired line set. The observed web request sends {id, lines}
// (no version), so we match that shape.
func (c *Client) PutCart(cart *Cart) (json.RawMessage, error) {
	body := map[string]any{"id": cart.ID, "lines": cart.Lines}
	var raw json.RawMessage
	return raw, c.DoJSON("PUT", c.cartURL(), body, &raw)
}

// AddLine adds qty to a product's existing quantity (creating the line if new).
func (c *Client) AddLine(productID string, qty float64) (json.RawMessage, error) {
	return c.mutateLine(productID, qty, true)
}

// SetLine sets a product's absolute quantity (0 removes it).
func (c *Client) SetLine(productID string, qty float64) (json.RawMessage, error) {
	return c.mutateLine(productID, qty, false)
}

func (c *Client) mutateLine(productID string, qty float64, add bool) (json.RawMessage, error) {
	cart, _, err := c.GetCart()
	if err != nil {
		return nil, err
	}
	cart.Lines = upsertLine(cart.Lines, productID, qty, add)
	return c.PutCart(cart)
}

func upsertLine(lines []CartLine, productID string, qty float64, add bool) []CartLine {
	out := make([]CartLine, 0, len(lines)+1)
	found := false
	for _, l := range lines {
		if l.ProductID == productID {
			found = true
			if add {
				l.Quantity += qty
			} else {
				l.Quantity = qty
			}
			if l.Quantity > 0 {
				out = append(out, l)
			}
			continue // drop the line if quantity fell to <= 0
		}
		out = append(out, l)
	}
	if !found && qty > 0 {
		out = append(out, CartLine{Quantity: qty, ProductID: productID, Sources: []any{}})
	}
	return out
}
