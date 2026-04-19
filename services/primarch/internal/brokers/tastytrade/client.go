// Package tastytrade is a read-only OAuth2 client for tastytrade's REST API,
// used by the wheel advisor. The client keeps a long-lived refresh token in
// Secret Manager and mints short-lived access tokens on demand.
//
// Execution endpoints (orders) are deliberately NOT in this file until the
// user opts in — the same client type will grow a PlaceOrder method later.
package tastytrade

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	defaultBaseURL  = "https://api.tastyworks.com"
	oauthTokenPath  = "/oauth/token"
)

// Client holds OAuth credentials + a cached access token.
type Client struct {
	baseURL      string
	clientID     string
	clientSecret string // optional — tastytrade public OAuth apps don't require it
	refreshToken string
	http         *http.Client
	logger       *slog.Logger

	mu          sync.Mutex
	accessToken string
	accessUntil time.Time
	// If tastytrade rotates the refresh token on exchange, we keep the new one
	// in memory so subsequent refreshes work. The caller is responsible for
	// persisting the rotated token back to Secret Manager (see OnTokenRotated).
	OnTokenRotated func(newRefresh string)
}

// NewFromEnv reads TASTYTRADE_CLIENT_ID + TASTYTRADE_REFRESH_TOKEN (required)
// and TASTYTRADE_CLIENT_SECRET (optional). Returns nil if client id or
// refresh token are missing — callers must tolerate a nil client.
func NewFromEnv(logger *slog.Logger) *Client {
	clientID := os.Getenv("TASTYTRADE_CLIENT_ID")
	refresh := os.Getenv("TASTYTRADE_REFRESH_TOKEN")
	if clientID == "" || refresh == "" || clientID == "unset" || refresh == "unset" {
		if logger != nil {
			logger.Warn("tastytrade not configured (TASTYTRADE_CLIENT_ID/REFRESH_TOKEN missing) — wheel advisor disabled")
		}
		return nil
	}
	base := os.Getenv("TASTYTRADE_BASE_URL")
	if base == "" {
		base = defaultBaseURL
	}
	return &Client{
		baseURL:      base,
		clientID:     clientID,
		clientSecret: os.Getenv("TASTYTRADE_CLIENT_SECRET"), // optional
		refreshToken: refresh,
		http:         &http.Client{Timeout: 30 * time.Second},
		logger:       logger,
	}
}

// Available reports whether credentials are in place.
func (c *Client) Available() bool {
	return c != nil && c.clientID != "" && c.refreshToken != ""
}

// Env returns a label for status endpoints.
func (c *Client) Env() string {
	if strings.Contains(c.baseURL, "tastyworks") {
		return "prod"
	}
	return "custom"
}

// tokenResp is tastytrade's OAuth token-endpoint response.
type tokenResp struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"` // may be rotated
	Scope        string `json:"scope"`
}

// refreshAccessToken exchanges the refresh token for a new access token.
// Must be called with c.mu held.
func (c *Client) refreshAccessToken(ctx context.Context) error {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", c.refreshToken)
	form.Set("client_id", c.clientID)
	if c.clientSecret != "" {
		form.Set("client_secret", c.clientSecret)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+oauthTokenPath, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("tastytrade oauth: %w", err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 400 {
		return fmt.Errorf("tastytrade oauth %d: %s", res.StatusCode, truncate(string(body), 300))
	}
	var tr tokenResp
	if err := json.Unmarshal(body, &tr); err != nil {
		return fmt.Errorf("decode token: %w", err)
	}
	if tr.AccessToken == "" {
		return fmt.Errorf("tastytrade oauth: empty access_token in response")
	}
	c.accessToken = tr.AccessToken
	expSec := tr.ExpiresIn
	if expSec <= 0 {
		expSec = 900 // default 15m if unspecified
	}
	c.accessUntil = time.Now().Add(time.Duration(expSec) * time.Second)
	// Handle refresh token rotation: if tastytrade issued a new one, update the
	// in-memory copy and notify the caller so they can re-stash it.
	if tr.RefreshToken != "" && tr.RefreshToken != c.refreshToken {
		c.refreshToken = tr.RefreshToken
		if c.OnTokenRotated != nil {
			go c.OnTokenRotated(tr.RefreshToken)
		}
	}
	return nil
}

// ensureAccess returns with a valid access token (refreshing if needed).
func (c *Client) ensureAccess(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.accessToken != "" && time.Now().Before(c.accessUntil.Add(-1*time.Minute)) {
		return nil
	}
	return c.refreshAccessToken(ctx)
}

// authedGet is a REST GET with Bearer token auth, auto-retrying once on 401.
func (c *Client) authedGet(ctx context.Context, path string, out interface{}) error {
	if err := c.ensureAccess(ctx); err != nil {
		return err
	}
	doReq := func(token string) (*http.Response, []byte, error) {
		req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+path, nil)
		if err != nil {
			return nil, nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/json")
		res, err := c.http.Do(req)
		if err != nil {
			return nil, nil, fmt.Errorf("GET %s: %w", path, err)
		}
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()
		return res, body, nil
	}
	res, body, err := doReq(c.accessToken)
	if err != nil {
		return err
	}
	if res.StatusCode == 401 {
		c.mu.Lock()
		c.accessToken = ""
		err := c.refreshAccessToken(ctx)
		tok := c.accessToken
		c.mu.Unlock()
		if err != nil {
			return err
		}
		res, body, err = doReq(tok)
		if err != nil {
			return err
		}
	}
	if res.StatusCode >= 400 {
		return fmt.Errorf("tastytrade GET %s %d: %s", path, res.StatusCode, truncate(string(body), 200))
	}
	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			return fmt.Errorf("decode %s: %w", path, err)
		}
	}
	return nil
}

// ─── Option chain ───────────────────────────────────────────

type ChainStrike struct {
	Strike         float64 `json:"strike-price,string"`
	Call           string  `json:"call"`
	Put            string  `json:"put"`
	CallStreamerID string  `json:"call-streamer-symbol"`
	PutStreamerID  string  `json:"put-streamer-symbol"`
}

type ChainExpiration struct {
	ExpirationType   string        `json:"expiration-type"`
	ExpirationDate   string        `json:"expiration-date"`
	DaysToExpiration int           `json:"days-to-expiration"`
	SettlementType   string        `json:"settlement-type"`
	Strikes          []ChainStrike `json:"strikes"`
}

type OptionChain struct {
	Symbol      string            `json:"underlying-symbol"`
	Expirations []ChainExpiration `json:"expirations"`
}

type nestedChainResp struct {
	Data struct {
		Items []OptionChain `json:"items"`
	} `json:"data"`
}

// GetOptionChainNested fetches the nested option chain for a symbol.
func (c *Client) GetOptionChainNested(ctx context.Context, symbol string) (*OptionChain, error) {
	var resp nestedChainResp
	if err := c.authedGet(ctx, "/option-chains/"+symbol+"/nested", &resp); err != nil {
		return nil, err
	}
	if len(resp.Data.Items) == 0 {
		return nil, fmt.Errorf("no chain for %s", symbol)
	}
	return &resp.Data.Items[0], nil
}

// ─── Market metrics ─────────────────────────────────────────

type MarketMetric struct {
	Symbol                      string  `json:"symbol"`
	ImpliedVolatilityIndexRank  float64 `json:"implied-volatility-index-rank,string"`
	HistoricalVolatility30d     float64 `json:"historical-volatility-30-day,string"`
	BetaMarket                  float64 `json:"beta,string"`
	MarketCap                   float64 `json:"market-cap,string"`
}

type marketMetricsResp struct {
	Data struct {
		Items []MarketMetric `json:"items"`
	} `json:"data"`
}

// GetMarketMetrics batches metrics for multiple symbols.
func (c *Client) GetMarketMetrics(ctx context.Context, symbols []string) ([]MarketMetric, error) {
	if len(symbols) == 0 {
		return nil, nil
	}
	q := strings.Join(symbols, ",")
	var resp marketMetricsResp
	if err := c.authedGet(ctx, "/market-metrics?symbols="+q, &resp); err != nil {
		return nil, err
	}
	return resp.Data.Items, nil
}

// ─── Accounts + positions ───────────────────────────────────

type Position struct {
	Symbol            string  `json:"symbol"`
	InstrumentType    string  `json:"instrument-type"`
	UnderlyingSymbol  string  `json:"underlying-symbol"`
	Quantity          float64 `json:"quantity,string"`
	QuantityDirection string  `json:"quantity-direction"`
	AverageOpenPrice  float64 `json:"average-open-price,string"`
	MarkPrice         float64 `json:"mark-price,string"`
	CostEffect        string  `json:"cost-effect"`
	ExpiresAt         string  `json:"expires-at"`
	IsFrozen          bool    `json:"is-frozen"`
}

type positionsResp struct {
	Data struct {
		Items []Position `json:"items"`
	} `json:"data"`
}

func (c *Client) ListPositions(ctx context.Context, accountNumber string) ([]Position, error) {
	var resp positionsResp
	if err := c.authedGet(ctx, "/accounts/"+accountNumber+"/positions", &resp); err != nil {
		return nil, err
	}
	return resp.Data.Items, nil
}

type AccountBalance struct {
	NetLiquidatingValue         float64 `json:"net-liquidating-value,string"`
	CashBalance                 float64 `json:"cash-balance,string"`
	LongEquityValue             float64 `json:"long-equity-value,string"`
	ShortEquityValue            float64 `json:"short-equity-value,string"`
	LongDerivativeValue         float64 `json:"long-derivative-value,string"`
	ShortDerivativeValue        float64 `json:"short-derivative-value,string"`
	BuyingPower                 float64 `json:"derivative-buying-power,string"`
}

type balanceResp struct {
	Data AccountBalance `json:"data"`
}

// GetAccountBalance returns the current cash + equity breakdown for an account.
func (c *Client) GetAccountBalance(ctx context.Context, accountNumber string) (*AccountBalance, error) {
	var resp balanceResp
	if err := c.authedGet(ctx, "/accounts/"+accountNumber+"/balances", &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

type accountsResp struct {
	Data struct {
		Items []struct {
			Account struct {
				AccountNumber string `json:"account-number"`
				Nickname      string `json:"nickname"`
			} `json:"account"`
		} `json:"items"`
	} `json:"data"`
}

func (c *Client) ListAccountNumbers(ctx context.Context) ([]string, error) {
	var resp accountsResp
	if err := c.authedGet(ctx, "/customers/me/accounts", &resp); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(resp.Data.Items))
	for _, a := range resp.Data.Items {
		out = append(out, a.Account.AccountNumber)
	}
	return out, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
