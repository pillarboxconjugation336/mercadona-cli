package client

import (
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ResolveWarehouse maps a Spanish postal code to the Mercadona warehouse that
// serves it. It calls the same endpoint the web app hits when you set your
// delivery address (POST /api/postal-codes/actions/change-pc/); the warehouse
// code (e.g. "mad1") comes back in the x-customer-wh response header.
//
// This matters because product ids and prices are per-warehouse, and online
// checkout requires the cart's warehouse to match the delivery address — so
// resolving the right warehouse from the user's postal code keeps prices honest
// and avoids checkout mismatches. No authentication is required.
//
// A postal code Mercadona doesn't deliver to comes back with no warehouse
// header; that's reported as an error rather than an empty string.
func (c *Client) ResolveWarehouse(postalCode string) (warehouse string, err error) {
	req, err := c.newReq(http.MethodPost, c.BaseURL+"/api/postal-codes/actions/change-pc/",
		map[string]string{"new_postal_code": postalCode})
	if err != nil {
		return "", err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", &APIError{Status: resp.StatusCode, Body: string(body)}
	}
	wh := strings.TrimSpace(resp.Header.Get("x-customer-wh"))
	if wh == "" {
		return "", fmt.Errorf("Mercadona doesn't deliver to postal code %q (no warehouse returned)", postalCode)
	}
	return wh, nil
}
