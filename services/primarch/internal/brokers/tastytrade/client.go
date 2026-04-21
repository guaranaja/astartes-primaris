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
	"strconv"
	"strings"
	"sync"
	"time"
)

// flexFloat / flexInt tolerate tastytrade's inconsistent numeric encoding —
// the same field can arrive quoted on one endpoint and bare on another.
type flexFloat float64

func (f *flexFloat) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	if len(data) >= 2 && data[0] == '"' && data[len(data)-1] == '"' {
		data = data[1 : len(data)-1]
	}
	if len(data) == 0 {
		return nil
	}
	v, err := strconv.ParseFloat(string(data), 64)
	if err != nil {
		return err
	}
	*f = flexFloat(v)
	return nil
}

type flexInt int64

func (i *flexInt) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	if len(data) >= 2 && data[0] == '"' && data[len(data)-1] == '"' {
		data = data[1 : len(data)-1]
	}
	if len(data) == 0 {
		return nil
	}
	v, err := strconv.ParseFloat(string(data), 64)
	if err != nil {
		return err
	}
	*i = flexInt(v)
	return nil
}

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
	c := &Client{
		baseURL:      base,
		clientID:     clientID,
		clientSecret: os.Getenv("TASTYTRADE_CLIENT_SECRET"), // optional
		refreshToken: refresh,
		http:         &http.Client{Timeout: 30 * time.Second},
		logger:       logger,
	}
	// When running on Cloud Run, persist rotated refresh tokens back to Secret
	// Manager so cold starts don't read a stale value. Env points to the
	// short secret name (e.g. "tastytrade-refresh-token").
	if cb := newGCPSecretPersister(os.Getenv("TASTYTRADE_REFRESH_TOKEN_SECRET"), logger); cb != nil {
		c.OnTokenRotated = cb
	}
	return c
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
	Symbol                     string
	ImpliedVolatilityIndexRank float64
	HistoricalVolatility30d    float64
	BetaMarket                 float64
	MarketCap                  float64
}

// UnmarshalJSON handles tastytrade's inconsistent encoding (bare vs. quoted
// numerics) without forcing callers to touch named-type arithmetic.
func (m *MarketMetric) UnmarshalJSON(data []byte) error {
	var wire struct {
		Symbol                     string    `json:"symbol"`
		ImpliedVolatilityIndexRank flexFloat `json:"implied-volatility-index-rank"`
		HistoricalVolatility30d    flexFloat `json:"historical-volatility-30-day"`
		BetaMarket                 flexFloat `json:"beta"`
		MarketCap                  flexFloat `json:"market-cap"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	m.Symbol = wire.Symbol
	m.ImpliedVolatilityIndexRank = float64(wire.ImpliedVolatilityIndexRank)
	m.HistoricalVolatility30d = float64(wire.HistoricalVolatility30d)
	m.BetaMarket = float64(wire.BetaMarket)
	m.MarketCap = float64(wire.MarketCap)
	return nil
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

// ─── Market data (quotes) ───────────────────────────────────

// Quote is a point-in-time market data snapshot used for reliability checks
// in the wheel advisor — bid/ask, open interest (options only), volume.
type Quote struct {
	Symbol         string  `json:"symbol"`
	InstrumentType string  // filled in by GetMarketData based on which bucket
	Bid            float64 `json:"bid,string"`
	Ask            float64 `json:"ask,string"`
	BidSize        int     `json:"bid-size"`
	AskSize        int     `json:"ask-size"`
	Last           float64 `json:"last,string"`
	Mark           float64 `json:"mark,string"`
	Volume         int     `json:"volume"`
	OpenInterest   int     `json:"open-interest"`
	UpdatedAt      string  `json:"updated-at"`
}

// marketDataResp wraps tastytrade's /market-data/by-type response. The
// "items" array mixes quote records across all instrument types requested.
type marketDataResp struct {
	Data struct {
		Items []struct {
			Symbol         string  `json:"symbol"`
			InstrumentType string  `json:"instrument-type"`
			Bid            string  `json:"bid"`
			Ask            string  `json:"ask"`
			BidSize        flexInt `json:"bid-size"`
			AskSize        flexInt `json:"ask-size"`
			Last           string  `json:"last"`
			Mark           string  `json:"mark"`
			Volume         flexInt `json:"volume"`
			OpenInterest   flexInt `json:"open-interest"`
			UpdatedAt      string  `json:"updated-at"`
		} `json:"items"`
	} `json:"data"`
}

// GetMarketData fetches snapshot quotes for a mixed batch of equities +
// equity options. Options should be provided in tastytrade's formatted
// symbol shape (from ChainStrike.Call / .Put fields).
//
// Up to ~100 items per call is safe; larger batches are chunked.
func (c *Client) GetMarketData(ctx context.Context, equities []string, options []string) ([]Quote, error) {
	if len(equities) == 0 && len(options) == 0 {
		return nil, nil
	}
	params := url.Values{}
	if len(equities) > 0 {
		params.Set("equity", strings.Join(equities, ","))
	}
	if len(options) > 0 {
		params.Set("equity-option", strings.Join(options, ","))
	}
	var resp marketDataResp
	if err := c.authedGet(ctx, "/market-data/by-type?"+params.Encode(), &resp); err != nil {
		return nil, err
	}
	out := make([]Quote, 0, len(resp.Data.Items))
	for _, it := range resp.Data.Items {
		q := Quote{
			Symbol:         it.Symbol,
			InstrumentType: it.InstrumentType,
			BidSize:        int(it.BidSize),
			AskSize:        int(it.AskSize),
			Volume:         int(it.Volume),
			OpenInterest:   int(it.OpenInterest),
			UpdatedAt:      it.UpdatedAt,
		}
		// Strings come through when values are present; may be empty strings
		// outside market hours. Use parseFloat with 0 fallback.
		q.Bid = parseFloat(it.Bid)
		q.Ask = parseFloat(it.Ask)
		q.Last = parseFloat(it.Last)
		q.Mark = parseFloat(it.Mark)
		out = append(out, q)
	}
	return out, nil
}

func parseFloat(s string) float64 {
	if s == "" {
		return 0
	}
	var v float64
	fmt.Sscanf(s, "%f", &v)
	return v
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
