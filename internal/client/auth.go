package client

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ivorjpc/mercadona/internal/config"
)

const tokenCacheFile = "token.json"

// Token is the cached session: the bearer + refresh token from
// POST /api/auth/tokens/ plus an optional raw Cookie header (Akamai clearance)
// imported from a browser session.
type Token struct {
	AccessToken  string  `json:"access_token"`
	RefreshToken string  `json:"refresh_token,omitempty"`
	CustomerID   flexStr `json:"customer_id"`
	Cookie       string  `json:"cookie,omitempty"`
}

// flexStr decodes a JSON value that may be a string or a number into a string,
// since the API's customer_id type is not guaranteed across responses.
type flexStr string

func (f *flexStr) UnmarshalJSON(b []byte) error {
	*f = flexStr(bytes.Trim(b, `"`))
	return nil
}

func (f flexStr) String() string { return string(f) }

// Login exchanges username/password for access+refresh tokens and caches them.
func (c *Client) Login(username, password string) (*Token, error) {
	var tok Token
	body := map[string]string{"username": username, "password": password}
	if err := c.doOnce("POST", c.BaseURL+"/api/auth/tokens/", body, &tok); err != nil {
		return nil, err
	}
	c.Username, c.Password = username, password
	c.applyToken(tok)
	c.saveSession()
	return &tok, nil
}

// Refresh swaps the refresh token for a fresh access token (no password). The
// API reuses the same endpoint with a {refresh_token} body.
func (c *Client) Refresh() (*Token, error) {
	if c.RefreshToken == "" {
		return nil, fmt.Errorf("no refresh token")
	}
	var tok Token
	body := map[string]string{"refresh_token": c.RefreshToken}
	if err := c.doOnce("POST", c.BaseURL+"/api/auth/tokens/", body, &tok); err != nil {
		return nil, err
	}
	if tok.RefreshToken == "" {
		tok.RefreshToken = c.RefreshToken // some servers don't rotate the refresh token
	}
	c.applyToken(tok)
	c.saveSession()
	return &tok, nil
}

// EnsureToken obtains an access token if we don't have one yet, using a cached
// refresh token or stored credentials.
func (c *Client) EnsureToken() error {
	if c.Token != "" {
		return nil
	}
	if c.CanReauth() {
		return c.reauth()
	}
	return fmt.Errorf("not authenticated and no credentials to log in")
}

// reauth tries the cheapest path first (refresh), then a full re-login.
func (c *Client) reauth() error {
	if c.RefreshToken != "" {
		if _, err := c.Refresh(); err == nil {
			return nil
		}
	}
	if c.Username != "" && c.Password != "" {
		if _, err := c.Login(c.Username, c.Password); err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("reauth failed: no valid refresh token or credentials")
}

func (c *Client) CanReauth() bool {
	return c.RefreshToken != "" || (c.Username != "" && c.Password != "")
}

func (c *Client) saveSession() {
	_ = config.Save(tokenCacheFile, Token{
		AccessToken:  c.Token,
		RefreshToken: c.RefreshToken,
		CustomerID:   flexStr(c.CustomerID),
		Cookie:       c.Cookie,
	})
}

// LoadToken hydrates the client from a previously cached token. Returns false if
// none is present.
func (c *Client) LoadToken() bool {
	var tok Token
	if err := config.Load(tokenCacheFile, &tok); err == nil && (tok.AccessToken != "" || tok.Cookie != "") {
		c.applyToken(tok)
		return true
	}
	return false
}

func (c *Client) applyToken(t Token) {
	c.Token = t.AccessToken
	if t.RefreshToken != "" {
		c.RefreshToken = t.RefreshToken
	}
	if t.Cookie != "" {
		c.Cookie = t.Cookie
	}
	if id := t.CustomerID.String(); id != "" {
		c.CustomerID = id
	}
}

// SaveSession caches a session assembled from outside the login flow (e.g. a
// browser "Copy as cURL"): a bearer token and/or a raw cookie header.
func SaveSession(token, cookie, customer string) error {
	return config.Save(tokenCacheFile, Token{
		AccessToken: token,
		Cookie:      cookie,
		CustomerID:  flexStr(customer),
	})
}

// Customer is the projected /api/customers/me/ response (its id may be reported
// under "id" or "customer_id" depending on the response).
type Customer struct {
	ID         flexStr `json:"id"`
	CustomerID flexStr `json:"customer_id"`
}

// Resolve returns whichever id field the response carried.
func (cu Customer) Resolve() string {
	if cu.ID != "" {
		return cu.ID.String()
	}
	return cu.CustomerID.String()
}

// EnsureCustomer fills CustomerID from the JWT's customer_uuid claim when it
// isn't already known. The literal "me" alias is NOT accepted by the API
// (returns 403), so every authed call needs the real id.
func (c *Client) EnsureCustomer() error {
	if c.CustomerID != "" {
		return nil
	}
	if c.Token != "" {
		if id := customerFromJWT(c.Token); id != "" {
			c.CustomerID = id
			return nil
		}
	}
	return fmt.Errorf("unknown customer id: token has no customer_uuid; set MERCADONA_CUSTOMER")
}

// customerFromJWT pulls the customer_uuid claim out of a SimpleJWT access token
// (no signature verification — we only need the id the server already trusts).
func customerFromJWT(token string) string {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var claims struct {
		CustomerUUID string `json:"customer_uuid"`
	}
	if json.Unmarshal(payload, &claims) != nil {
		return ""
	}
	return claims.CustomerUUID
}

// Me fetches the current customer (GET /api/customers/<id>/) — also the cheapest
// way to verify that the supplied token actually authenticates.
func (c *Client) Me() (*Customer, json.RawMessage, error) {
	if err := c.EnsureCustomer(); err != nil {
		return nil, nil, err
	}
	url := fmt.Sprintf("%s/api/customers/%s/?lang=%s&wh=%s", c.BaseURL, c.CustomerID, c.Lang, c.Warehouse)
	var raw json.RawMessage
	if err := c.DoJSON("GET", url, nil, &raw); err != nil {
		return nil, nil, err
	}
	var cu Customer
	_ = json.Unmarshal(raw, &cu)
	return &cu, raw, nil
}
