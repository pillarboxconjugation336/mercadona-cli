package client

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

func (c *Client) custURL(suffix string) string {
	return fmt.Sprintf("%s/api/customers/%s/%s", c.BaseURL, c.CustomerID, suffix)
}

// Addresses lists the customer's saved delivery addresses.
func (c *Client) Addresses() (json.RawMessage, error) {
	var raw json.RawMessage
	return raw, c.DoJSON("GET", c.custURL("addresses/"), nil, &raw)
}

// Slots lists delivery slots for an address. Slots live under the ADDRESS, not
// the checkout (GET /api/customers/<cid>/addresses/<addrId>/slots/), and the
// response is {next_page, results:[{id,start,end,price,available,open,...}]}.
func (c *Client) Slots(addressID int) (json.RawMessage, error) {
	var raw json.RawMessage
	return raw, c.DoJSON("GET", c.custURL(fmt.Sprintf("addresses/%d/slots/", addressID)), nil, &raw)
}

// CreateCheckout opens a checkout from the current cart. The response carries
// the checkout id plus the default address (raw JSON); delivery slots are
// fetched separately via Slots, since they hang off the address.
func (c *Client) CreateCheckout(cart *Cart) (json.RawMessage, error) {
	body := map[string]any{"cart": map[string]any{
		"id": cart.ID, "version": cart.Version, "lines": cart.Lines,
	}}
	var raw json.RawMessage
	return raw, c.DoJSON("POST", c.custURL("checkouts/"), body, &raw)
}

// SetDelivery attaches a delivery address + slot to an open checkout.
func (c *Client) SetDelivery(checkoutID string, addressID int, slotID string) (json.RawMessage, error) {
	body := map[string]any{
		"address": map[string]any{"id": addressID},
		"slot":    map[string]any{"id": slotID},
	}
	var raw json.RawMessage
	return raw, c.DoJSON("PUT", c.custURL("checkouts/"+checkoutID+"/delivery-info/"), body, &raw)
}

// GetCheckout reads an open checkout — used to read its authoritative total (incl.
// delivery) right before the irreversible submit, for the budget guard.
func (c *Client) GetCheckout(checkoutID string) (json.RawMessage, error) {
	var raw json.RawMessage
	return raw, c.DoJSON("GET", c.custURL("checkouts/"+checkoutID+"/"), nil, &raw)
}

// SubmitOrder places the order. This is IRREVERSIBLE and spends money — callers
// MUST gate it behind explicit user consent.
func (c *Client) SubmitOrder(checkoutID string) (json.RawMessage, error) {
	var raw json.RawMessage
	return raw, c.DoJSON("POST", c.custURL("checkouts/"+checkoutID+"/orders/"), nil, &raw)
}

// ExtractTotal pulls the order/cart total (in €) out of a cart or checkout JSON
// response. It reads field-by-field and leniently: a live checkout carries the
// payable total at summary.total ("76.84" = products 68.64 + slot 8.20) but also a
// bare-string "price" subtotal ("68.64") — decoding the whole thing into one struct
// errors on that string and would discard summary.total, so each candidate is
// parsed on its own. Returns false when no positive total is found. (Field shape
// verified against a live checkout, 2026-06-26.)
func ExtractTotal(raw json.RawMessage) (float64, bool) {
	var top map[string]json.RawMessage
	if json.Unmarshal(raw, &top) != nil {
		return 0, false
	}
	if t, ok := totalField(top["summary"]); ok { // authoritative for carts + checkouts
		return t, true
	}
	if t, ok := parseMoney(top["total"]); ok { // some shapes carry a top-level total
		return t, true
	}
	if t, ok := totalField(top["price"]); ok { // fallback only if price is an object
		return t, true
	}
	return 0, false
}

// totalField parses the "total" key of a JSON object value; false if the value
// isn't an object (e.g. a bare string) or has no positive total.
func totalField(obj json.RawMessage) (float64, bool) {
	if len(obj) == 0 {
		return 0, false
	}
	var m map[string]json.RawMessage
	if json.Unmarshal(obj, &m) != nil {
		return 0, false
	}
	return parseMoney(m["total"])
}

// parseMoney reads a euro amount the API sends as a JSON string ("76.84") or a
// number; false for empty/null/non-numeric/non-positive.
func parseMoney(b json.RawMessage) (float64, bool) {
	if len(b) == 0 {
		return 0, false
	}
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		return 0, false
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || f <= 0 {
		return 0, false
	}
	return f, true
}
