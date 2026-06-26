package client

import (
	"encoding/json"
	"fmt"
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

// SubmitOrder places the order. This is IRREVERSIBLE and spends money — callers
// MUST gate it behind explicit user consent.
func (c *Client) SubmitOrder(checkoutID string) (json.RawMessage, error) {
	var raw json.RawMessage
	return raw, c.DoJSON("POST", c.custURL("checkouts/"+checkoutID+"/orders/"), nil, &raw)
}
