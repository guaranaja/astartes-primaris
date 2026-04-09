package cfo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	monarchGraphQLURL = "https://api.monarch.com/graphql"
	monarchLoginURL   = "https://api.monarch.com/auth/login/"
)

// MonarchClient talks to the Monarch Money GraphQL API.
type MonarchClient struct {
	token      string
	httpClient *http.Client
}

// NewMonarchClient creates a client with an existing session token.
func NewMonarchClient(sessionToken string) *MonarchClient {
	return &MonarchClient{
		token: sessionToken,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Login authenticates with email/password via Monarch's REST login endpoint.
func Login(email, password string) (*MonarchClient, error) {
	payload := map[string]interface{}{
		"username":       email,
		"password":       password,
		"supports_mfa":   true,
		"trusted_device": false,
	}
	b, _ := json.Marshal(payload)
	resp, err := http.Post(monarchLoginURL, "application/json", bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("monarch login: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 403 {
		return nil, fmt.Errorf("monarch login: MFA required or blocked (403)")
	}
	if resp.StatusCode == 429 {
		return nil, fmt.Errorf("monarch login: rate limited (429)")
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("monarch login: HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("monarch login decode: %w", err)
	}
	if result.Token == "" {
		return nil, fmt.Errorf("monarch login: no token returned")
	}
	return NewMonarchClient(result.Token), nil
}

// Token returns the session token for persistence.
func (c *MonarchClient) Token() string {
	return c.token
}

func (c *MonarchClient) query(gql string, variables map[string]interface{}, out interface{}) error {
	payload := map[string]interface{}{
		"query":     gql,
		"variables": variables,
	}
	b, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", monarchGraphQLURL, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Token "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("monarch request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("monarch returned %d: %s", resp.StatusCode, string(body))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// ─── Monarch Data Types ────────────────────────────────────

// MAccount is a Monarch Money account.
type MAccount struct {
	ID              string  `json:"id"`
	DisplayName     string  `json:"displayName"`
	Type            string  `json:"type"` // depository, credit, investment, loan, etc.
	Subtype         string  `json:"subtype"`
	Balance         float64 `json:"currentBalance"`
	AvailableBalance float64 `json:"availableBalance"`
	Institution     string  `json:"institution_name"`
	IsHidden        bool    `json:"isHidden"`
	UpdatedAt       string  `json:"updatedAt"`
}

// MTransaction is a Monarch Money transaction.
type MTransaction struct {
	ID          string  `json:"id"`
	Date        string  `json:"date"`
	Amount      float64 `json:"amount"`
	Merchant    string  `json:"merchant_name"`
	Category    string  `json:"category_name"`
	Account     string  `json:"account_name"`
	Notes       string  `json:"notes"`
	IsRecurring bool    `json:"isRecurring"`
	IsPending   bool    `json:"pending"`
}

// MBudget is a Monarch budget category.
type MBudget struct {
	Category    string  `json:"category"`
	Budgeted    float64 `json:"budgeted"`
	Spent       float64 `json:"spent"`
	Available   float64 `json:"available"`
}

// MCashFlow is a Monarch cash flow summary.
type MCashFlow struct {
	Income   float64 `json:"income"`
	Expenses float64 `json:"expenses"`
	Savings  float64 `json:"savings"`
}

// ─── API Methods ───────────────────────────────────────────

// ListAccounts returns all Monarch accounts.
func (c *MonarchClient) ListAccounts() ([]MAccount, error) {
	gql := `query {
		accounts {
			id
			displayName
			type { name }
			subtype { name }
			currentBalance
			availableBalance
			institution { name }
			isHidden
			updatedAt
		}
	}`
	var resp struct {
		Data struct {
			Accounts []struct {
				ID               string  `json:"id"`
				DisplayName      string  `json:"displayName"`
				Type             struct{ Name string } `json:"type"`
				Subtype          struct{ Name string } `json:"subtype"`
				CurrentBalance   float64 `json:"currentBalance"`
				AvailableBalance float64 `json:"availableBalance"`
				Institution      struct{ Name string } `json:"institution"`
				IsHidden         bool    `json:"isHidden"`
				UpdatedAt        string  `json:"updatedAt"`
			} `json:"accounts"`
		} `json:"data"`
	}
	if err := c.query(gql, nil, &resp); err != nil {
		return nil, err
	}
	out := make([]MAccount, len(resp.Data.Accounts))
	for i, a := range resp.Data.Accounts {
		out[i] = MAccount{
			ID:               a.ID,
			DisplayName:      a.DisplayName,
			Type:             a.Type.Name,
			Subtype:          a.Subtype.Name,
			Balance:          a.CurrentBalance,
			AvailableBalance: a.AvailableBalance,
			Institution:      a.Institution.Name,
			IsHidden:         a.IsHidden,
			UpdatedAt:        a.UpdatedAt,
		}
	}
	return out, nil
}

// ListTransactions returns recent transactions.
func (c *MonarchClient) ListTransactions(start, end time.Time) ([]MTransaction, error) {
	gql := `query GetTransactions($startDate: Date!, $endDate: Date!) {
		allTransactions(filters: { startDate: $startDate, endDate: $endDate }) {
			results {
				id
				date
				amount
				merchant { name }
				category { name }
				account { displayName }
				notes
				isRecurring
				pending
			}
		}
	}`
	vars := map[string]interface{}{
		"startDate": start.Format("2006-01-02"),
		"endDate":   end.Format("2006-01-02"),
	}
	var resp struct {
		Data struct {
			AllTransactions struct {
				Results []struct {
					ID          string  `json:"id"`
					Date        string  `json:"date"`
					Amount      float64 `json:"amount"`
					Merchant    struct{ Name string } `json:"merchant"`
					Category    struct{ Name string } `json:"category"`
					Account     struct{ DisplayName string } `json:"account"`
					Notes       string  `json:"notes"`
					IsRecurring bool    `json:"isRecurring"`
					Pending     bool    `json:"pending"`
				} `json:"results"`
			} `json:"allTransactions"`
		} `json:"data"`
	}
	if err := c.query(gql, vars, &resp); err != nil {
		return nil, err
	}
	out := make([]MTransaction, len(resp.Data.AllTransactions.Results))
	for i, t := range resp.Data.AllTransactions.Results {
		out[i] = MTransaction{
			ID:          t.ID,
			Date:        t.Date,
			Amount:      t.Amount,
			Merchant:    t.Merchant.Name,
			Category:    t.Category.Name,
			Account:     t.Account.DisplayName,
			Notes:       t.Notes,
			IsRecurring: t.IsRecurring,
			IsPending:   t.Pending,
		}
	}
	return out, nil
}

// GetCashFlow returns income/expenses/savings for a date range.
func (c *MonarchClient) GetCashFlow(start, end time.Time) (*MCashFlow, error) {
	gql := `query GetCashFlow($startDate: Date!, $endDate: Date!) {
		cashFlow(startDate: $startDate, endDate: $endDate) {
			totalIncome
			totalExpenses
			savings
		}
	}`
	vars := map[string]interface{}{
		"startDate": start.Format("2006-01-02"),
		"endDate":   end.Format("2006-01-02"),
	}
	var resp struct {
		Data struct {
			CashFlow struct {
				TotalIncome   float64 `json:"totalIncome"`
				TotalExpenses float64 `json:"totalExpenses"`
				Savings       float64 `json:"savings"`
			} `json:"cashFlow"`
		} `json:"data"`
	}
	if err := c.query(gql, vars, &resp); err != nil {
		return nil, err
	}
	return &MCashFlow{
		Income:   resp.Data.CashFlow.TotalIncome,
		Expenses: resp.Data.CashFlow.TotalExpenses,
		Savings:  resp.Data.CashFlow.Savings,
	}, nil
}

// GetRecurringTransactions returns upcoming recurring bills/income.
func (c *MonarchClient) GetRecurringTransactions() ([]MTransaction, error) {
	gql := `query {
		recurringTransactions {
			id
			title
			amount
			merchant { name }
			category { name }
			account { displayName }
			isRecurring
		}
	}`
	var resp struct {
		Data struct {
			RecurringTransactions []struct {
				ID       string  `json:"id"`
				Title    string  `json:"title"`
				Amount   float64 `json:"amount"`
				Merchant struct{ Name string } `json:"merchant"`
				Category struct{ Name string } `json:"category"`
				Account  struct{ DisplayName string } `json:"account"`
			} `json:"recurringTransactions"`
		} `json:"data"`
	}
	if err := c.query(gql, nil, &resp); err != nil {
		return nil, err
	}
	var out []MTransaction
	for _, t := range resp.Data.RecurringTransactions {
		out = append(out, MTransaction{
			ID:          t.ID,
			Merchant:    t.Merchant.Name,
			Amount:      t.Amount,
			Category:    t.Category.Name,
			Account:     t.Account.DisplayName,
			IsRecurring: true,
		})
	}
	return out, nil
}

// Ping tests connectivity to Monarch.
func (c *MonarchClient) Ping() error {
	gql := `query { subscription { id } }`
	var resp struct {
		Data struct {
			Subscription struct {
				ID string `json:"id"`
			} `json:"subscription"`
		} `json:"data"`
	}
	return c.query(gql, nil, &resp)
}
