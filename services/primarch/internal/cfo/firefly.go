// Package cfo provides clients for the CFO Engine (Firefly III) and Monarch Money APIs.
// Council uses these to build a unified view of personal + family finances alongside trading data.
package cfo

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// FireflyClient talks to the Firefly III REST API.
type FireflyClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// NewFireflyClient creates a client for the CFO Engine (Firefly III).
func NewFireflyClient(baseURL, personalAccessToken string) *FireflyClient {
	return &FireflyClient{
		baseURL: baseURL,
		token:   personalAccessToken,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *FireflyClient) do(method, path string, body io.Reader) (*http.Response, error) {
	u := c.baseURL + path
	req, err := http.NewRequest(method, u, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.httpClient.Do(req)
}

func (c *FireflyClient) getJSON(path string, out interface{}) error {
	resp, err := c.do("GET", path, nil)
	if err != nil {
		return fmt.Errorf("firefly request %s: %w", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("firefly %s returned %d: %s", path, resp.StatusCode, string(b))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// ─── Firefly III Data Types ────────────────────────────────

// FFAccount represents a Firefly III account.
type FFAccount struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	Type           string  `json:"type"`    // asset, expense, revenue, liability
	Balance        float64 `json:"current_balance,string"`
	Currency       string  `json:"currency_code"`
	AccountRole    string  `json:"account_role"` // defaultAsset, savingAsset, ccAsset
	IBAN           string  `json:"iban"`
	Active         bool    `json:"active"`
	IncludeNetWorth bool   `json:"include_net_worth"`
}

type ffAccountWrapper struct {
	ID         string    `json:"id"`
	Attributes FFAccount `json:"attributes"`
}

type ffAccountsResponse struct {
	Data []ffAccountWrapper `json:"data"`
}

// FFTransaction represents a Firefly III transaction.
type FFTransaction struct {
	ID          string  `json:"transaction_journal_id"`
	Description string  `json:"description"`
	Date        string  `json:"date"`
	Amount      float64 `json:"amount,string"`
	Type        string  `json:"type"` // withdrawal, deposit, transfer
	Category    string  `json:"category_name"`
	Source      string  `json:"source_name"`
	Destination string  `json:"destination_name"`
	Currency    string  `json:"currency_code"`
	Tags        []string `json:"tags"`
}

type ffTransactionGroup struct {
	ID         string `json:"id"`
	Attributes struct {
		Transactions []FFTransaction `json:"transactions"`
	} `json:"attributes"`
}

type ffTransactionsResponse struct {
	Data []ffTransactionGroup `json:"data"`
}

// FFBudget represents a Firefly III budget.
type FFBudget struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Limit  float64 `json:"-"` // populated from budget limits
	Spent  float64 `json:"-"`
	Active bool    `json:"active"`
}

type ffBudgetWrapper struct {
	ID         string   `json:"id"`
	Attributes FFBudget `json:"attributes"`
}

type ffBudgetsResponse struct {
	Data []ffBudgetWrapper `json:"data"`
}

// FFBudgetLimit represents spending limit for a budget in a period.
type FFBudgetLimit struct {
	Amount float64 `json:"amount,string"`
	Spent  float64 `json:"spent,string"`
	Period string  `json:"period"`
	Start  string  `json:"start"`
	End    string  `json:"end"`
}

type ffBudgetLimitWrapper struct {
	Attributes FFBudgetLimit `json:"attributes"`
}

type ffBudgetLimitsResponse struct {
	Data []ffBudgetLimitWrapper `json:"data"`
}

// FFBill represents a recurring bill in Firefly III.
type FFBill struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	AmountMin float64 `json:"amount_min,string"`
	AmountMax float64 `json:"amount_max,string"`
	Date      string  `json:"date"` // next expected date
	Repeat    string  `json:"repeat_freq"`
	Active    bool    `json:"active"`
	Currency  string  `json:"currency_code"`
}

type ffBillWrapper struct {
	ID         string `json:"id"`
	Attributes FFBill `json:"attributes"`
}

type ffBillsResponse struct {
	Data []ffBillWrapper `json:"data"`
}

// FFSummary is a key-value map of summary data from Firefly.
type FFSummary map[string]struct {
	Value    float64 `json:"value"`
	Currency string  `json:"currency_code"`
}

// ─── API Methods ───────────────────────────────────────────

// ListAccounts returns all accounts.
func (c *FireflyClient) ListAccounts() ([]FFAccount, error) {
	var resp ffAccountsResponse
	if err := c.getJSON("/api/v1/accounts?type=all&limit=100", &resp); err != nil {
		return nil, err
	}
	out := make([]FFAccount, len(resp.Data))
	for i, d := range resp.Data {
		out[i] = d.Attributes
		out[i].ID = d.ID
	}
	return out, nil
}

// ListAssetAccounts returns only asset accounts (bank, savings, checking).
func (c *FireflyClient) ListAssetAccounts() ([]FFAccount, error) {
	var resp ffAccountsResponse
	if err := c.getJSON("/api/v1/accounts?type=asset&limit=100", &resp); err != nil {
		return nil, err
	}
	out := make([]FFAccount, len(resp.Data))
	for i, d := range resp.Data {
		out[i] = d.Attributes
		out[i].ID = d.ID
	}
	return out, nil
}

// ListTransactions returns transactions in a date range.
func (c *FireflyClient) ListTransactions(start, end time.Time) ([]FFTransaction, error) {
	path := fmt.Sprintf("/api/v1/transactions?start=%s&end=%s&limit=100",
		url.QueryEscape(start.Format("2006-01-02")),
		url.QueryEscape(end.Format("2006-01-02")))
	var resp ffTransactionsResponse
	if err := c.getJSON(path, &resp); err != nil {
		return nil, err
	}
	var out []FFTransaction
	for _, g := range resp.Data {
		out = append(out, g.Attributes.Transactions...)
	}
	return out, nil
}

// ListBudgets returns all budgets.
func (c *FireflyClient) ListBudgets() ([]FFBudget, error) {
	var resp ffBudgetsResponse
	if err := c.getJSON("/api/v1/budgets?limit=100", &resp); err != nil {
		return nil, err
	}
	out := make([]FFBudget, len(resp.Data))
	for i, d := range resp.Data {
		out[i] = d.Attributes
		out[i].ID = d.ID
	}
	return out, nil
}

// ListBills returns all active bills.
func (c *FireflyClient) ListBills() ([]FFBill, error) {
	var resp ffBillsResponse
	if err := c.getJSON("/api/v1/bills?limit=100", &resp); err != nil {
		return nil, err
	}
	var out []FFBill
	for _, d := range resp.Data {
		if d.Attributes.Active {
			b := d.Attributes
			b.ID = d.ID
			out = append(out, b)
		}
	}
	return out, nil
}

// GetSummary returns the basic summary for a date range (net worth, balance, income, expenses).
func (c *FireflyClient) GetSummary(start, end time.Time) (FFSummary, error) {
	path := fmt.Sprintf("/api/v1/summary/basic?start=%s&end=%s",
		url.QueryEscape(start.Format("2006-01-02")),
		url.QueryEscape(end.Format("2006-01-02")))
	var summary FFSummary
	if err := c.getJSON(path, &summary); err != nil {
		return nil, err
	}
	return summary, nil
}

// Ping tests connectivity to the Firefly III instance.
func (c *FireflyClient) Ping() error {
	_, err := c.do("GET", "/api/v1/about", nil)
	return err
}
