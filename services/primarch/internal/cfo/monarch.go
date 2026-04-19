package cfo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const monarchGraphQLURL = "https://api.monarch.com/graphql"

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

// Login authenticates with email/password and stores the session token.
func Login(email, password string) (*MonarchClient, error) {
	payload := map[string]interface{}{
		"query": `mutation Login($email: String!, $password: String!) {
			loginUser(input: {email: $email, password: $password}) {
				token
				errors { message }
			}
		}`,
		"variables": map[string]string{
			"email":    email,
			"password": password,
		},
	}
	b, _ := json.Marshal(payload)
	resp, err := http.Post(monarchGraphQLURL, "application/json", bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("monarch login: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Data struct {
			LoginUser struct {
				Token  string `json:"token"`
				Errors []struct {
					Message string `json:"message"`
				} `json:"errors"`
			} `json:"loginUser"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("monarch login decode: %w", err)
	}
	if len(result.Data.LoginUser.Errors) > 0 {
		return nil, fmt.Errorf("monarch login: %s", result.Data.LoginUser.Errors[0].Message)
	}
	if result.Data.LoginUser.Token == "" {
		return nil, fmt.Errorf("monarch login: no token returned")
	}
	return NewMonarchClient(result.Data.LoginUser.Token), nil
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
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("MONARCH TOKEN EXPIRED OR INVALID (HTTP %d) — "+
			"Log in to app.monarch.com, open DevTools → Network tab → click a graphql request → copy Authorization header token, "+
			"copy the gist.web.userToken value, then run: "+
			"gcloud secrets versions add monarch-token --data-file=- <<< '<new-token>' "+
			"and redeploy Primarch", resp.StatusCode)
	}
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
			currentBalance
			institution { name }
			isHidden
			updatedAt
		}
	}`
	var resp struct {
		Data struct {
			Accounts []struct {
				ID             string  `json:"id"`
				DisplayName    string  `json:"displayName"`
				Type           struct{ Name string } `json:"type"`
				CurrentBalance float64 `json:"currentBalance"`
				Institution    struct{ Name string } `json:"institution"`
				IsHidden       bool    `json:"isHidden"`
				UpdatedAt      string  `json:"updatedAt"`
			} `json:"accounts"`
		} `json:"data"`
	}
	if err := c.query(gql, nil, &resp); err != nil {
		return nil, err
	}
	out := make([]MAccount, len(resp.Data.Accounts))
	for i, a := range resp.Data.Accounts {
		out[i] = MAccount{
			ID:          a.ID,
			DisplayName: a.DisplayName,
			Type:        a.Type.Name,
			Balance:     a.CurrentBalance,
			Institution: a.Institution.Name,
			IsHidden:    a.IsHidden,
			UpdatedAt:   a.UpdatedAt,
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
// Uses the aggregates query (Monarch v2 API — cashFlow query was removed).
func (c *MonarchClient) GetCashFlow(start, end time.Time) (*MCashFlow, error) {
	gql := `query GetCashFlow($startDate: Date!, $endDate: Date!) {
		aggregates(filters: { startDate: $startDate, endDate: $endDate }) {
			summary {
				sumIncome
				sumExpense
				savings
			}
		}
	}`
	vars := map[string]interface{}{
		"startDate": start.Format("2006-01-02"),
		"endDate":   end.Format("2006-01-02"),
	}
	var resp struct {
		Data struct {
			Aggregates []struct {
				Summary struct {
					SumIncome  float64 `json:"sumIncome"`
					SumExpense float64 `json:"sumExpense"`
					Savings    float64 `json:"savings"`
				} `json:"summary"`
			} `json:"aggregates"`
		} `json:"data"`
	}
	if err := c.query(gql, vars, &resp); err != nil {
		return nil, err
	}
	cf := &MCashFlow{}
	if len(resp.Data.Aggregates) > 0 {
		s := resp.Data.Aggregates[0].Summary
		cf.Income = s.SumIncome
		cf.Expenses = -s.SumExpense // sumExpense is negative, normalize to positive
		cf.Savings = s.Savings
	}
	return cf, nil
}

// MRecurringStream is a Monarch "recurring transaction stream" — the
// authoritative bill/income definition with expected amount and next forecast.
// This is what Monarch's Recurring/Bills UI displays.
type MRecurringStream struct {
	ID            string  `json:"id"`
	Merchant      string  `json:"merchant"`
	Category      string  `json:"category"`
	Frequency     string  `json:"frequency"` // MONTHLY, YEARLY, WEEKLY, BIWEEKLY, etc.
	Amount        float64 `json:"amount"`    // signed — negative = expense
	NextDate      string  `json:"next_date,omitempty"`
	NextAmount    float64 `json:"next_amount,omitempty"`
	IsActive      bool    `json:"is_active"`
	Account       string  `json:"account,omitempty"`
	LogoURL       string  `json:"logo_url,omitempty"`
}

// GetRecurringStreams returns the authoritative recurring streams Monarch
// displays on its Bills/Recurring page. Preferred over GetRecurringTransactions
// because it surfaces stream *definitions* (including ones that haven't charged
// recently) with their expected amount and next forecasted date.
func (c *MonarchClient) GetRecurringStreams() ([]MRecurringStream, error) {
	gql := `query Web_GetRecurringTransactionsList {
		recurringTransactionStreams {
			id
			frequency
			amount
			isActive
			baseDate
			nextForecastedTransaction {
				date
				amount
			}
			merchant { name logoUrl }
			category { name }
			account { displayName }
		}
	}`
	var resp struct {
		Data struct {
			RecurringTransactionStreams []struct {
				ID        string  `json:"id"`
				Frequency string  `json:"frequency"`
				Amount    float64 `json:"amount"`
				IsActive  bool    `json:"isActive"`
				BaseDate  string  `json:"baseDate"`
				NextForecastedTransaction *struct {
					Date   string  `json:"date"`
					Amount float64 `json:"amount"`
				} `json:"nextForecastedTransaction"`
				Merchant *struct {
					Name    string `json:"name"`
					LogoUrl string `json:"logoUrl"`
				} `json:"merchant"`
				Category *struct {
					Name string `json:"name"`
				} `json:"category"`
				Account *struct {
					DisplayName string `json:"displayName"`
				} `json:"account"`
			} `json:"recurringTransactionStreams"`
		} `json:"data"`
	}
	if err := c.query(gql, nil, &resp); err != nil {
		return nil, err
	}
	out := make([]MRecurringStream, 0, len(resp.Data.RecurringTransactionStreams))
	for _, s := range resp.Data.RecurringTransactionStreams {
		stream := MRecurringStream{
			ID:        s.ID,
			Frequency: s.Frequency,
			Amount:    s.Amount,
			IsActive:  s.IsActive,
		}
		if s.Merchant != nil {
			stream.Merchant = s.Merchant.Name
			stream.LogoURL = s.Merchant.LogoUrl
		}
		if s.Category != nil {
			stream.Category = s.Category.Name
		}
		if s.Account != nil {
			stream.Account = s.Account.DisplayName
		}
		if s.NextForecastedTransaction != nil {
			stream.NextDate = s.NextForecastedTransaction.Date
			stream.NextAmount = s.NextForecastedTransaction.Amount
		}
		out = append(out, stream)
	}
	return out, nil
}

// GetRecurringTransactions is the legacy fallback — queries the 60-day window of
// transaction instances. Kept for resilience if the streams endpoint changes
// shape; prefer GetRecurringStreams.
func (c *MonarchClient) GetRecurringTransactions() ([]MTransaction, error) {
	// Pull last 60 days of recurring to capture monthly bills
	end := time.Now()
	start := end.AddDate(0, -2, 0)
	gql := `query GetRecurring($startDate: Date!, $endDate: Date!) {
		allTransactions(filters: { startDate: $startDate, endDate: $endDate, isRecurring: true }) {
			results {
				id
				date
				amount
				merchant { name }
				category { name }
				account { displayName }
				isRecurring
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
					ID       string  `json:"id"`
					Date     string  `json:"date"`
					Amount   float64 `json:"amount"`
					Merchant struct{ Name string } `json:"merchant"`
					Category struct{ Name string } `json:"category"`
					Account  struct{ DisplayName string } `json:"account"`
				} `json:"results"`
			} `json:"allTransactions"`
		} `json:"data"`
	}
	if err := c.query(gql, vars, &resp); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []MTransaction
	for _, t := range resp.Data.AllTransactions.Results {
		key := t.Merchant.Name + "|" + t.Category.Name
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, MTransaction{
			ID:          t.ID,
			Date:        t.Date,
			Merchant:    t.Merchant.Name,
			Amount:      t.Amount,
			Category:    t.Category.Name,
			Account:     t.Account.DisplayName,
			IsRecurring: true,
		})
	}
	return out, nil
}

// GetBudgets returns spending by category for a date range.
// Monarch v2 removed the budgetData query, so we derive from transaction aggregates.
func (c *MonarchClient) GetBudgets(start, end time.Time) ([]MBudget, error) {
	txns, err := c.ListTransactions(start, end)
	if err != nil {
		return nil, fmt.Errorf("monarch budgets (via transactions): %w", err)
	}
	// Group spending by category
	cats := map[string]float64{}
	for _, t := range txns {
		if t.Amount >= 0 {
			continue // skip income
		}
		cats[t.Category] += -t.Amount // normalize to positive
	}
	var out []MBudget
	for cat, spent := range cats {
		if cat == "" || cat == "Transfer" {
			continue
		}
		out = append(out, MBudget{
			Category: cat,
			Spent:    spent,
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
