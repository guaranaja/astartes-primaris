package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	_ "github.com/lib/pq"

	"github.com/guaranaja/astartes-primaris/services/primarch/internal/domain"
)

// PGStore implements DataStore using PostgreSQL.
type PGStore struct {
	db     *sql.DB
	logger *slog.Logger
}

// NewPGStore connects to PostgreSQL and returns a persistent store.
func NewPGStore(dsn string, logger *slog.Logger) (*PGStore, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}
	logger.Info("connected to PostgreSQL")
	s := &PGStore{db: db, logger: logger}
	s.ensureSchema()
	return s, nil
}

// ensureSchema creates tables that Primarch manages directly.
func (s *PGStore) ensureSchema() {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS commands (
			id            TEXT PRIMARY KEY,
			engine_id     TEXT NOT NULL,
			command       TEXT NOT NULL,
			scope         TEXT NOT NULL,
			params        JSONB,
			status        TEXT NOT NULL DEFAULT 'pending',
			error_message TEXT,
			created_at    TIMESTAMPTZ DEFAULT now(),
			updated_at    TIMESTAMPTZ DEFAULT now()
		);
		CREATE INDEX IF NOT EXISTS idx_commands_engine_status ON commands (engine_id, status);
	`)
	if err != nil {
		s.logger.Error("ensure schema", "error", err)
	}
}

// Close closes the database connection.
func (s *PGStore) Close() error {
	return s.db.Close()
}

// DB returns the underlying database connection for direct queries.
func (s *PGStore) DB() *sql.DB {
	return s.db
}

// ─── Fortress ───────────────────────────────────────────────

func (s *PGStore) ListFortresses() []domain.Fortress {
	rows, err := s.db.Query(`SELECT id, name, asset_class, metadata, created_at, updated_at FROM fortresses ORDER BY created_at`)
	if err != nil {
		s.logger.Error("list fortresses", "error", err)
		return nil
	}
	defer rows.Close()

	var out []domain.Fortress
	for rows.Next() {
		var f domain.Fortress
		var meta []byte
		if err := rows.Scan(&f.ID, &f.Name, &f.AssetClass, &meta, &f.CreatedAt, &f.UpdatedAt); err != nil {
			s.logger.Error("scan fortress", "error", err)
			continue
		}
		if len(meta) > 0 {
			json.Unmarshal(meta, &f.Metadata)
		}
		f.Companies = s.companiesForFortress(f.ID)
		out = append(out, f)
	}
	return out
}

func (s *PGStore) GetFortress(id string) (*domain.Fortress, error) {
	var f domain.Fortress
	var meta []byte
	err := s.db.QueryRow(`SELECT id, name, asset_class, metadata, created_at, updated_at FROM fortresses WHERE id = $1`, id).
		Scan(&f.ID, &f.Name, &f.AssetClass, &meta, &f.CreatedAt, &f.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("fortress %q not found", id)
	}
	if err != nil {
		return nil, err
	}
	if len(meta) > 0 {
		json.Unmarshal(meta, &f.Metadata)
	}
	f.Companies = s.companiesForFortress(f.ID)
	return &f, nil
}

func (s *PGStore) CreateFortress(f *domain.Fortress) error {
	meta, _ := json.Marshal(f.Metadata)
	now := time.Now()
	f.CreatedAt = now
	f.UpdatedAt = now
	_, err := s.db.Exec(`INSERT INTO fortresses (id, name, asset_class, metadata, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6)`,
		f.ID, f.Name, f.AssetClass, meta, f.CreatedAt, f.UpdatedAt)
	if err != nil {
		return fmt.Errorf("fortress %q: %w", f.ID, err)
	}
	return nil
}

func (s *PGStore) UpdateFortress(f *domain.Fortress) error {
	meta, _ := json.Marshal(f.Metadata)
	f.UpdatedAt = time.Now()
	res, err := s.db.Exec(`UPDATE fortresses SET name=$2, asset_class=$3, metadata=$4, updated_at=$5 WHERE id=$1`,
		f.ID, f.Name, f.AssetClass, meta, f.UpdatedAt)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("fortress %q not found", f.ID)
	}
	return nil
}

func (s *PGStore) DeleteFortress(id string) error {
	res, err := s.db.Exec(`DELETE FROM fortresses WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("fortress %q not found", id)
	}
	return nil
}

// ─── Company ────────────────────────────────────────────────

func (s *PGStore) companiesForFortress(fortressID string) []domain.Company {
	q := `SELECT id, fortress_id, name, type, risk_limits, created_at, updated_at FROM companies`
	var args []interface{}
	if fortressID != "" {
		q += ` WHERE fortress_id = $1`
		args = append(args, fortressID)
	}
	q += ` ORDER BY created_at`

	rows, err := s.db.Query(q, args...)
	if err != nil {
		s.logger.Error("list companies", "error", err)
		return nil
	}
	defer rows.Close()

	var out []domain.Company
	for rows.Next() {
		var c domain.Company
		var rl []byte
		if err := rows.Scan(&c.ID, &c.FortressID, &c.Name, &c.Type, &rl, &c.CreatedAt, &c.UpdatedAt); err != nil {
			s.logger.Error("scan company", "error", err)
			continue
		}
		if len(rl) > 0 {
			json.Unmarshal(rl, &c.RiskLimits)
		}
		c.Marines = s.marinesForCompany(c.ID)
		out = append(out, c)
	}
	return out
}

func (s *PGStore) ListCompanies(fortressID string) []domain.Company {
	return s.companiesForFortress(fortressID)
}

func (s *PGStore) GetCompany(id string) (*domain.Company, error) {
	var c domain.Company
	var rl []byte
	err := s.db.QueryRow(`SELECT id, fortress_id, name, type, risk_limits, created_at, updated_at FROM companies WHERE id=$1`, id).
		Scan(&c.ID, &c.FortressID, &c.Name, &c.Type, &rl, &c.CreatedAt, &c.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("company %q not found", id)
	}
	if err != nil {
		return nil, err
	}
	if len(rl) > 0 {
		json.Unmarshal(rl, &c.RiskLimits)
	}
	c.Marines = s.marinesForCompany(c.ID)
	return &c, nil
}

func (s *PGStore) CreateCompany(c *domain.Company) error {
	rl, _ := json.Marshal(c.RiskLimits)
	now := time.Now()
	c.CreatedAt = now
	c.UpdatedAt = now
	_, err := s.db.Exec(`INSERT INTO companies (id, fortress_id, name, type, risk_limits, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		c.ID, c.FortressID, c.Name, c.Type, rl, c.CreatedAt, c.UpdatedAt)
	return err
}

func (s *PGStore) UpdateCompany(c *domain.Company) error {
	rl, _ := json.Marshal(c.RiskLimits)
	c.UpdatedAt = time.Now()
	res, err := s.db.Exec(`UPDATE companies SET fortress_id=$2, name=$3, type=$4, risk_limits=$5, updated_at=$6 WHERE id=$1`,
		c.ID, c.FortressID, c.Name, c.Type, rl, c.UpdatedAt)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("company %q not found", c.ID)
	}
	return nil
}

func (s *PGStore) DeleteCompany(id string) error {
	res, err := s.db.Exec(`DELETE FROM companies WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("company %q not found", id)
	}
	return nil
}

// ─── Marine ─────────────────────────────────────────────────

func (s *PGStore) marinesForCompany(companyID string) []domain.Marine {
	q := `SELECT id, company_id, name, strategy_name, strategy_version, broker_account_id, status, schedule, parameters, resources, created_at, updated_at FROM marines`
	var args []interface{}
	if companyID != "" {
		q += ` WHERE company_id = $1`
		args = append(args, companyID)
	}
	q += ` ORDER BY created_at`

	rows, err := s.db.Query(q, args...)
	if err != nil {
		s.logger.Error("list marines", "error", err)
		return nil
	}
	defer rows.Close()
	return s.scanMarines(rows)
}

func (s *PGStore) scanMarines(rows *sql.Rows) []domain.Marine {
	var out []domain.Marine
	for rows.Next() {
		var m domain.Marine
		var sched, params, res []byte
		var broker sql.NullString
		if err := rows.Scan(&m.ID, &m.CompanyID, &m.Name, &m.StrategyName, &m.StrategyVersion,
			&broker, &m.Status, &sched, &params, &res, &m.CreatedAt, &m.UpdatedAt); err != nil {
			s.logger.Error("scan marine", "error", err)
			continue
		}
		m.BrokerAccountID = broker.String
		if len(sched) > 0 {
			json.Unmarshal(sched, &m.Schedule)
		}
		if len(params) > 0 {
			json.Unmarshal(params, &m.Parameters)
		}
		if len(res) > 0 {
			json.Unmarshal(res, &m.Resources)
		}
		out = append(out, m)
	}
	return out
}

func (s *PGStore) ListMarines(companyID string) []domain.Marine {
	return s.marinesForCompany(companyID)
}

func (s *PGStore) ListAllMarines() []domain.Marine {
	return s.marinesForCompany("")
}

func (s *PGStore) GetMarine(id string) (*domain.Marine, error) {
	var m domain.Marine
	var sched, params, res []byte
	var broker sql.NullString
	err := s.db.QueryRow(`SELECT id, company_id, name, strategy_name, strategy_version, broker_account_id, status, schedule, parameters, resources, created_at, updated_at FROM marines WHERE id=$1`, id).
		Scan(&m.ID, &m.CompanyID, &m.Name, &m.StrategyName, &m.StrategyVersion, &broker, &m.Status, &sched, &params, &res, &m.CreatedAt, &m.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("marine %q not found", id)
	}
	if err != nil {
		return nil, err
	}
	m.BrokerAccountID = broker.String
	if len(sched) > 0 {
		json.Unmarshal(sched, &m.Schedule)
	}
	if len(params) > 0 {
		json.Unmarshal(params, &m.Parameters)
	}
	if len(res) > 0 {
		json.Unmarshal(res, &m.Resources)
	}
	return &m, nil
}

func (s *PGStore) CreateMarine(m *domain.Marine) error {
	sched, _ := json.Marshal(m.Schedule)
	params, _ := json.Marshal(m.Parameters)
	res, _ := json.Marshal(m.Resources)
	now := time.Now()
	m.CreatedAt = now
	m.UpdatedAt = now
	if m.Status == "" {
		m.Status = domain.StatusDormant
	}
	_, err := s.db.Exec(`INSERT INTO marines (id, company_id, name, strategy_name, strategy_version, broker_account_id, status, schedule, parameters, resources, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		m.ID, m.CompanyID, m.Name, m.StrategyName, m.StrategyVersion, m.BrokerAccountID, m.Status, sched, params, res, m.CreatedAt, m.UpdatedAt)
	return err
}

func (s *PGStore) UpdateMarine(m *domain.Marine) error {
	sched, _ := json.Marshal(m.Schedule)
	params, _ := json.Marshal(m.Parameters)
	res, _ := json.Marshal(m.Resources)
	m.UpdatedAt = time.Now()
	_, err := s.db.Exec(`UPDATE marines SET company_id=$2, name=$3, strategy_name=$4, strategy_version=$5, broker_account_id=$6, status=$7, schedule=$8, parameters=$9, resources=$10, updated_at=$11 WHERE id=$1`,
		m.ID, m.CompanyID, m.Name, m.StrategyName, m.StrategyVersion, m.BrokerAccountID, m.Status, sched, params, res, m.UpdatedAt)
	return err
}

func (s *PGStore) UpdateMarineStatus(id string, status domain.MarineStatus) error {
	_, err := s.db.Exec(`UPDATE marines SET status=$2, updated_at=$3 WHERE id=$1`, id, status, time.Now())
	return err
}

func (s *PGStore) DeleteMarine(id string) error {
	res, err := s.db.Exec(`DELETE FROM marines WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("marine %q not found", id)
	}
	return nil
}

// ─── Cycles ─────────────────────────────────────────────────

func (s *PGStore) RecordCycle(c domain.MarineCycle) {
	_, err := s.db.Exec(`INSERT INTO marine_cycles (id, marine_id, wake_at, sleep_at, status, signals_generated, orders_submitted, duration_ms, error_message) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		c.ID, c.MarineID, c.WakeAt, c.SleepAt, c.Status, c.SignalsGenerated, c.OrdersSubmitted, c.DurationMs, c.Error)
	if err != nil {
		s.logger.Error("record cycle", "error", err)
	}
}

func (s *PGStore) GetCycles(marineID string, limit int) []domain.MarineCycle {
	rows, err := s.db.Query(`SELECT id, marine_id, wake_at, sleep_at, status, signals_generated, orders_submitted, duration_ms, error_message FROM marine_cycles WHERE marine_id=$1 ORDER BY wake_at DESC LIMIT $2`, marineID, limit)
	if err != nil {
		s.logger.Error("get cycles", "error", err)
		return nil
	}
	defer rows.Close()

	var out []domain.MarineCycle
	for rows.Next() {
		var c domain.MarineCycle
		var errMsg sql.NullString
		if err := rows.Scan(&c.ID, &c.MarineID, &c.WakeAt, &c.SleepAt, &c.Status, &c.SignalsGenerated, &c.OrdersSubmitted, &c.DurationMs, &errMsg); err != nil {
			continue
		}
		c.Error = errMsg.String
		out = append(out, c)
	}
	return out
}

// ─── Kill Switch ────────────────────────────────────────────

func (s *PGStore) ActivateKillSwitch(scope string) {
	if scope == "imperium" {
		s.db.Exec(`UPDATE marines SET status='disabled', schedule = jsonb_set(schedule, '{enabled}', 'false')`)
		return
	}
	// Try fortress scope
	res, _ := s.db.Exec(`UPDATE marines SET status='disabled', schedule = jsonb_set(schedule, '{enabled}', 'false') WHERE company_id IN (SELECT id FROM companies WHERE fortress_id=$1)`, scope)
	if n, _ := res.RowsAffected(); n > 0 {
		return
	}
	// Try company scope
	res, _ = s.db.Exec(`UPDATE marines SET status='disabled', schedule = jsonb_set(schedule, '{enabled}', 'false') WHERE company_id=$1`, scope)
	if n, _ := res.RowsAffected(); n > 0 {
		return
	}
	// Try marine scope
	s.db.Exec(`UPDATE marines SET status='disabled', schedule = jsonb_set(schedule, '{enabled}', 'false') WHERE id=$1`, scope)
}

// ─── Trading Accounts ───────────────────────────────────────

func (s *PGStore) ListAccounts() []domain.TradingAccount {
	rows, err := s.db.Query(`SELECT id, name, broker, type, account_number, initial_balance, current_balance, total_pnl, total_payouts, payout_count, profit_split, status, instruments, created_at, updated_at FROM trading_accounts ORDER BY created_at`)
	if err != nil {
		s.logger.Error("list accounts", "error", err)
		return nil
	}
	defer rows.Close()
	return s.scanAccounts(rows)
}

func (s *PGStore) scanAccounts(rows *sql.Rows) []domain.TradingAccount {
	var out []domain.TradingAccount
	for rows.Next() {
		var a domain.TradingAccount
		var acctNum sql.NullString
		var instruments []byte
		if err := rows.Scan(&a.ID, &a.Name, &a.Broker, &a.Type, &acctNum, &a.InitialBalance, &a.CurrentBalance, &a.TotalPnL, &a.TotalPayouts, &a.PayoutCount, &a.ProfitSplit, &a.Status, &instruments, &a.CreatedAt, &a.UpdatedAt); err != nil {
			s.logger.Error("scan account", "error", err)
			continue
		}
		a.AccountNumber = acctNum.String
		out = append(out, a)
	}
	return out
}

func (s *PGStore) GetAccount(id string) (*domain.TradingAccount, error) {
	var a domain.TradingAccount
	var acctNum sql.NullString
	var instruments []byte
	err := s.db.QueryRow(`SELECT id, name, broker, type, account_number, initial_balance, current_balance, total_pnl, total_payouts, payout_count, profit_split, status, instruments, created_at, updated_at FROM trading_accounts WHERE id=$1`, id).
		Scan(&a.ID, &a.Name, &a.Broker, &a.Type, &acctNum, &a.InitialBalance, &a.CurrentBalance, &a.TotalPnL, &a.TotalPayouts, &a.PayoutCount, &a.ProfitSplit, &a.Status, &instruments, &a.CreatedAt, &a.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("account %q not found", id)
	}
	if err != nil {
		return nil, err
	}
	a.AccountNumber = acctNum.String
	return &a, nil
}

func (s *PGStore) CreateAccount(a *domain.TradingAccount) error {
	now := time.Now()
	a.CreatedAt = now
	a.UpdatedAt = now
	if a.Status == "" {
		a.Status = "active"
	}
	if a.ProfitSplit == 0 && a.Type == domain.AccountProp {
		a.ProfitSplit = 0.90
	}
	if a.ProfitSplit == 0 && a.Type == domain.AccountPersonal {
		a.ProfitSplit = 1.0
	}
	_, err := s.db.Exec(`INSERT INTO trading_accounts (id, name, broker, type, account_number, initial_balance, current_balance, total_pnl, total_payouts, payout_count, profit_split, status, instruments, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
		a.ID, a.Name, a.Broker, a.Type, a.AccountNumber, a.InitialBalance, a.CurrentBalance, a.TotalPnL, a.TotalPayouts, a.PayoutCount, a.ProfitSplit, a.Status, "{}", a.CreatedAt, a.UpdatedAt)
	return err
}

func (s *PGStore) UpdateAccount(a *domain.TradingAccount) error {
	a.UpdatedAt = time.Now()
	_, err := s.db.Exec(`UPDATE trading_accounts SET name=$2, broker=$3, type=$4, current_balance=$5, total_pnl=$6, total_payouts=$7, payout_count=$8, profit_split=$9, status=$10, updated_at=$11 WHERE id=$1`,
		a.ID, a.Name, a.Broker, a.Type, a.CurrentBalance, a.TotalPnL, a.TotalPayouts, a.PayoutCount, a.ProfitSplit, a.Status, a.UpdatedAt)
	return err
}

// ─── Payouts ────────────────────────────────────────────────

func (s *PGStore) ListPayouts(accountID string) []domain.Payout {
	q := `SELECT id, account_id, gross_amount, net_amount, destination, status, requested_at, completed_at, note FROM payouts`
	var args []interface{}
	if accountID != "" {
		q += ` WHERE account_id = $1`
		args = append(args, accountID)
	}
	q += ` ORDER BY requested_at DESC`

	rows, err := s.db.Query(q, args...)
	if err != nil {
		s.logger.Error("list payouts", "error", err)
		return nil
	}
	defer rows.Close()

	var out []domain.Payout
	for rows.Next() {
		var p domain.Payout
		var note sql.NullString
		if err := rows.Scan(&p.ID, &p.AccountID, &p.GrossAmount, &p.NetAmount, &p.Destination, &p.Status, &p.RequestedAt, &p.CompletedAt, &note); err != nil {
			continue
		}
		p.Note = note.String
		out = append(out, p)
	}
	return out
}

func (s *PGStore) RecordPayout(p domain.Payout) error {
	// Get account for split calculation
	a, err := s.GetAccount(p.AccountID)
	if err != nil {
		return err
	}
	if p.NetAmount == 0 {
		p.NetAmount = p.GrossAmount * a.ProfitSplit
	}
	if p.Status == "" {
		p.Status = "completed"
	}
	p.RequestedAt = time.Now()

	_, err = s.db.Exec(`INSERT INTO payouts (id, account_id, gross_amount, net_amount, destination, status, requested_at, note) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		p.ID, p.AccountID, p.GrossAmount, p.NetAmount, p.Destination, p.Status, p.RequestedAt, p.Note)
	if err != nil {
		return err
	}

	// Update account totals
	_, err = s.db.Exec(`UPDATE trading_accounts SET total_payouts = total_payouts + $2, payout_count = payout_count + 1, updated_at = $3 WHERE id = $1`,
		p.AccountID, p.NetAmount, time.Now())
	return err
}

// ─── Budget & Allocations ───────────────────────────────────

func (s *PGStore) GetBudget() *domain.BudgetSummary {
	var data []byte
	err := s.db.QueryRow(`SELECT data FROM budget WHERE id='current'`).Scan(&data)
	if err != nil {
		b := DefaultBudget()
		return b
	}
	var b domain.BudgetSummary
	json.Unmarshal(data, &b)
	return &b
}

func (s *PGStore) UpdateBudget(b *domain.BudgetSummary) {
	data, _ := json.Marshal(b)
	s.db.Exec(`INSERT INTO budget (id, data, updated_at) VALUES ('current', $1, $2) ON CONFLICT (id) DO UPDATE SET data=$1, updated_at=$2`, data, time.Now())
}

func (s *PGStore) GetAllocations() []domain.Allocation {
	rows, err := s.db.Query(`SELECT category, percentage, amount FROM allocations ORDER BY percentage DESC`)
	if err != nil {
		return DefaultAllocations()
	}
	defer rows.Close()

	var out []domain.Allocation
	for rows.Next() {
		var a domain.Allocation
		rows.Scan(&a.Category, &a.Percentage, &a.Amount)
		out = append(out, a)
	}
	if len(out) == 0 {
		return DefaultAllocations()
	}
	return out
}

func (s *PGStore) SetAllocations(allocs []domain.Allocation) {
	tx, err := s.db.Begin()
	if err != nil {
		return
	}
	tx.Exec(`DELETE FROM allocations`)
	for _, a := range allocs {
		tx.Exec(`INSERT INTO allocations (category, percentage, amount) VALUES ($1, $2, $3)`, a.Category, a.Percentage, a.Amount)
	}
	tx.Commit()
}

// ─── Roadmap ────────────────────────────────────────────────

func (s *PGStore) GetRoadmap() *domain.Roadmap {
	var data []byte
	err := s.db.QueryRow(`SELECT data FROM roadmap WHERE id='default'`).Scan(&data)
	if err != nil {
		return DefaultRoadmap()
	}
	var r domain.Roadmap
	json.Unmarshal(data, &r)
	return &r
}

func (s *PGStore) UpdateRoadmap(r *domain.Roadmap) {
	r.UpdatedAt = time.Now()
	data, _ := json.Marshal(r)
	s.db.Exec(`INSERT INTO roadmap (id, current_phase, data, updated_at) VALUES ('default', $1, $2, $3) ON CONFLICT (id) DO UPDATE SET current_phase=$1, data=$2, updated_at=$3`,
		r.CurrentPhase, data, r.UpdatedAt)
}

// ─── Withdrawal Advice ──────────────────────────────────────

func (s *PGStore) GetWithdrawalAdvice() []domain.WithdrawalAdvice {
	accounts := s.ListAccounts()
	allPayouts := s.ListPayouts("")
	allocs := s.GetAllocations()

	var advice []domain.WithdrawalAdvice
	for _, a := range accounts {
		if a.Status != "active" || a.Type != domain.AccountProp {
			continue
		}
		profit := a.CurrentBalance - a.InitialBalance
		if profit <= 0 {
			continue
		}
		wa := domain.WithdrawalAdvice{
			AccountID:       a.ID,
			AccountName:     a.Name,
			CurrentBalance:  a.CurrentBalance,
			AvailableProfit: profit,
			NextReviewAt:    time.Now().AddDate(0, 0, 7),
		}
		net := profit * a.ProfitSplit
		switch {
		case net >= 2000:
			wa.Urgency = "now"
			wa.RecommendedAmt = net
			wa.Reason = fmt.Sprintf("$%.0f available — withdraw to protect gains and fund goals", net)
		case net >= 1000:
			wa.Urgency = "soon"
			wa.RecommendedAmt = net
			wa.Reason = fmt.Sprintf("$%.0f available — good time to take profits", net)
		case net >= 500:
			wa.Urgency = "hold"
			wa.Reason = "Building cushion — let it grow unless you need cash flow"
		default:
			wa.Urgency = "wait"
			wa.Reason = "Keep trading — not enough profit to justify withdrawal fees"
		}
		if wa.RecommendedAmt > 0 {
			wa.Allocations = distributeWithdrawal(wa.RecommendedAmt, allocs)
		}
		// Goal context
		goals := s.ListGoals()
		if topGoal := topActiveGoalFromList(goals); topGoal != nil {
			remaining := topGoal.TargetAmount - topGoal.CurrentAmount
			if remaining > 0 {
				needed := calcPayoutsNeededFromList(remaining, allPayouts, allocs)
				if needed > 0 {
					wa.Reason += fmt.Sprintf(" (%d payouts to %s)", needed, topGoal.Name)
				}
			}
		}
		advice = append(advice, wa)
	}
	return advice
}

// ─── Business Metrics ───────────────────────────────────────

func (s *PGStore) GetBusinessMetrics() domain.BusinessMetrics {
	accounts := s.ListAccounts()
	budget := s.GetBudget()
	rm := s.GetRoadmap()

	m := domain.BusinessMetrics{CurrentPhase: rm.CurrentPhase}
	for _, a := range accounts {
		m.LifetimePnL += a.TotalPnL
		m.LifetimePayouts += a.TotalPayouts
		if a.Status == "blown" {
			m.AccountsBlown++
		}
		if a.Status == "graduated" {
			m.AccountsGraduated++
		}
		if a.Type == domain.AccountPersonal {
			m.PersonalAccountValue += a.CurrentBalance
		}
	}
	m.MonthlyPnL = budget.TradingIncome
	m.MonthlyPayouts = budget.PropPayouts
	m.MonthlyExpenses = budget.TotalExpenses
	m.MonthlyNetIncome = budget.NetCashFlow

	switch rm.CurrentPhase {
	case domain.PhaseInitiate:
		m.PersonalAccountGoal = 5000
	case domain.PhaseNeophyte:
		m.PersonalAccountGoal = 25000
	case domain.PhaseBattleBrother:
		m.PersonalAccountGoal = 50000
	case domain.PhaseVeteran:
		m.PersonalAccountGoal = 100000
	default:
		m.PersonalAccountGoal = 250000
	}
	if m.PersonalAccountGoal > 0 {
		m.GoalProgress = m.PersonalAccountValue / m.PersonalAccountGoal
		if m.GoalProgress > 1 {
			m.GoalProgress = 1
		}
	}
	for _, p := range rm.Phases {
		if p.Phase == rm.CurrentPhase && len(p.UnlockWhen) > 0 {
			met := 0
			for _, c := range p.UnlockWhen {
				if c.Met {
					met++
				}
			}
			m.PhaseProgress = float64(met) / float64(len(p.UnlockWhen))
			break
		}
	}
	if rm.StartedAt.Year() > 2000 {
		m.DaysInPhase = int(time.Since(rm.StartedAt).Hours() / 24)
	}
	return m
}

// ─── Goals ──────────────────────────────────────────────────

func (s *PGStore) ListGoals() []domain.Goal {
	rows, err := s.db.Query(`SELECT id, name, description, category, target_amount, current_amount, priority, target_date, status, icon, created_at, updated_at, completed_at FROM goals ORDER BY priority, created_at`)
	if err != nil {
		s.logger.Error("list goals", "error", err)
		return nil
	}
	defer rows.Close()

	allPayouts := s.ListPayouts("")
	allocs := s.GetAllocations()

	var out []domain.Goal
	for rows.Next() {
		var g domain.Goal
		var desc, icon sql.NullString
		if err := rows.Scan(&g.ID, &g.Name, &desc, &g.Category, &g.TargetAmount, &g.CurrentAmount, &g.Priority, &g.TargetDate, &g.Status, &icon, &g.CreatedAt, &g.UpdatedAt, &g.CompletedAt); err != nil {
			continue
		}
		g.Description = desc.String
		g.Icon = icon.String
		g.PayoutsNeeded = calcPayoutsNeededFromList(g.TargetAmount-g.CurrentAmount, allPayouts, allocs)
		out = append(out, g)
	}
	return out
}

func (s *PGStore) GetGoal(id string) (*domain.Goal, error) {
	var g domain.Goal
	var desc, icon sql.NullString
	err := s.db.QueryRow(`SELECT id, name, description, category, target_amount, current_amount, priority, target_date, status, icon, created_at, updated_at, completed_at FROM goals WHERE id=$1`, id).
		Scan(&g.ID, &g.Name, &desc, &g.Category, &g.TargetAmount, &g.CurrentAmount, &g.Priority, &g.TargetDate, &g.Status, &icon, &g.CreatedAt, &g.UpdatedAt, &g.CompletedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("goal %q not found", id)
	}
	if err != nil {
		return nil, err
	}
	g.Description = desc.String
	g.Icon = icon.String
	return &g, nil
}

func (s *PGStore) CreateGoal(g *domain.Goal) error {
	now := time.Now()
	g.CreatedAt = now
	g.UpdatedAt = now
	if g.Status == "" {
		g.Status = domain.GoalActive
	}
	if g.Priority == 0 {
		g.Priority = 3
	}
	_, err := s.db.Exec(`INSERT INTO goals (id, name, description, category, target_amount, current_amount, priority, target_date, status, icon, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		g.ID, g.Name, g.Description, g.Category, g.TargetAmount, g.CurrentAmount, g.Priority, g.TargetDate, g.Status, g.Icon, g.CreatedAt, g.UpdatedAt)
	return err
}

func (s *PGStore) UpdateGoal(g *domain.Goal) error {
	g.UpdatedAt = time.Now()
	if g.CurrentAmount >= g.TargetAmount && g.Status == domain.GoalActive {
		g.Status = domain.GoalCompleted
		now := time.Now()
		g.CompletedAt = &now
	}
	_, err := s.db.Exec(`UPDATE goals SET name=$2, description=$3, category=$4, target_amount=$5, current_amount=$6, priority=$7, target_date=$8, status=$9, icon=$10, updated_at=$11, completed_at=$12 WHERE id=$1`,
		g.ID, g.Name, g.Description, g.Category, g.TargetAmount, g.CurrentAmount, g.Priority, g.TargetDate, g.Status, g.Icon, g.UpdatedAt, g.CompletedAt)
	return err
}

func (s *PGStore) DeleteGoal(id string) error {
	res, err := s.db.Exec(`DELETE FROM goals WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("goal %q not found", id)
	}
	return nil
}

func (s *PGStore) ContributeToGoal(c domain.GoalContribution) error {
	c.CreatedAt = time.Now()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`INSERT INTO goal_contributions (id, goal_id, amount, source, payout_id, note, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		c.ID, c.GoalID, c.Amount, c.Source, c.PayoutID, c.Note, c.CreatedAt)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`UPDATE goals SET current_amount = current_amount + $2, updated_at = $3 WHERE id = $1`, c.GoalID, c.Amount, time.Now())
	if err != nil {
		return err
	}
	// Auto-complete if target reached
	tx.Exec(`UPDATE goals SET status='completed', completed_at=$2 WHERE id=$1 AND current_amount >= target_amount AND status='active'`, c.GoalID, time.Now())
	return tx.Commit()
}

func (s *PGStore) ListContributions(goalID string) []domain.GoalContribution {
	q := `SELECT id, goal_id, amount, source, payout_id, note, created_at FROM goal_contributions`
	var args []interface{}
	if goalID != "" {
		q += ` WHERE goal_id = $1`
		args = append(args, goalID)
	}
	q += ` ORDER BY created_at DESC`

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var out []domain.GoalContribution
	for rows.Next() {
		var c domain.GoalContribution
		var payoutID, note sql.NullString
		rows.Scan(&c.ID, &c.GoalID, &c.Amount, &c.Source, &payoutID, &note, &c.CreatedAt)
		c.PayoutID = payoutID.String
		c.Note = note.String
		out = append(out, c)
	}
	return out
}

// ─── Expenses & Billing ─────────────────────────────────────

func (s *PGStore) ListExpenses() []domain.Expense {
	rows, err := s.db.Query(`SELECT id, name, category, amount, frequency, due_day, auto_pay, status, next_due, created_at, updated_at FROM expenses ORDER BY created_at`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var out []domain.Expense
	for rows.Next() {
		var e domain.Expense
		rows.Scan(&e.ID, &e.Name, &e.Category, &e.Amount, &e.Frequency, &e.DueDay, &e.AutoPay, &e.Status, &e.NextDue, &e.CreatedAt, &e.UpdatedAt)
		out = append(out, e)
	}
	return out
}

func (s *PGStore) CreateExpense(e *domain.Expense) error {
	now := time.Now()
	e.CreatedAt = now
	e.UpdatedAt = now
	if e.Status == "" {
		e.Status = "active"
	}
	_, err := s.db.Exec(`INSERT INTO expenses (id, name, category, amount, frequency, due_day, auto_pay, status, next_due, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		e.ID, e.Name, e.Category, e.Amount, e.Frequency, e.DueDay, e.AutoPay, e.Status, e.NextDue, e.CreatedAt, e.UpdatedAt)
	return err
}

func (s *PGStore) UpdateExpense(e *domain.Expense) error {
	e.UpdatedAt = time.Now()
	_, err := s.db.Exec(`UPDATE expenses SET name=$2, category=$3, amount=$4, frequency=$5, due_day=$6, auto_pay=$7, status=$8, next_due=$9, updated_at=$10 WHERE id=$1`,
		e.ID, e.Name, e.Category, e.Amount, e.Frequency, e.DueDay, e.AutoPay, e.Status, e.NextDue, e.UpdatedAt)
	return err
}

func (s *PGStore) DeleteExpense(id string) error {
	res, err := s.db.Exec(`DELETE FROM expenses WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("expense %q not found", id)
	}
	return nil
}

func (s *PGStore) RecordPayment(p domain.Payment) error {
	p.PaidAt = time.Now()
	_, err := s.db.Exec(`INSERT INTO payments (id, expense_id, amount, paid_at, method, note) VALUES ($1,$2,$3,$4,$5,$6)`,
		p.ID, p.ExpenseID, p.Amount, p.PaidAt, p.Method, p.Note)
	return err
}

func (s *PGStore) ListPayments(expenseID string) []domain.Payment {
	q := `SELECT id, expense_id, amount, paid_at, method, note FROM payments`
	var args []interface{}
	if expenseID != "" {
		q += ` WHERE expense_id = $1`
		args = append(args, expenseID)
	}
	q += ` ORDER BY paid_at DESC`

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var out []domain.Payment
	for rows.Next() {
		var p domain.Payment
		var note sql.NullString
		rows.Scan(&p.ID, &p.ExpenseID, &p.Amount, &p.PaidAt, &p.Method, &note)
		p.Note = note.String
		out = append(out, p)
	}
	return out
}

func (s *PGStore) GetBillingSummary() domain.BillingSummary {
	expenses := s.ListExpenses()
	payments := s.ListPayments("")
	budget := s.GetBudget()

	now := time.Now()
	month := now.Format("2006-01")
	summary := domain.BillingSummary{Month: month}

	for _, e := range expenses {
		if e.Status != "active" {
			continue
		}
		monthly := e.Amount
		switch e.Frequency {
		case domain.FreqWeekly:
			monthly = e.Amount * 4.33
		case domain.FreqBiweekly:
			monthly = e.Amount * 2.17
		case domain.FreqAnnual:
			monthly = e.Amount / 12
		case domain.FreqOneTime:
			if e.NextDue != nil && e.NextDue.Format("2006-01") != month {
				continue
			}
		}
		summary.TotalExpenses += monthly
		summary.Expenses = append(summary.Expenses, e)
	}

	for _, p := range payments {
		if p.PaidAt.Format("2006-01") == month {
			summary.TotalPaid += p.Amount
			summary.Payments = append(summary.Payments, p)
		}
	}

	summary.TotalPending = summary.TotalExpenses - summary.TotalPaid
	if summary.TotalPending < 0 {
		summary.TotalPending = 0
	}
	if summary.TotalExpenses > 0 && budget != nil {
		summary.TradingCoverage = budget.TradingIncome / summary.TotalExpenses
		if summary.TradingCoverage > 1 {
			summary.TradingCoverage = 1
		}
	}
	return summary
}

// ─── Helpers ────────────────────────────────────────────────

func topActiveGoalFromList(goals []domain.Goal) *domain.Goal {
	var top *domain.Goal
	for i := range goals {
		if goals[i].Status != domain.GoalActive {
			continue
		}
		if top == nil || goals[i].Priority < top.Priority {
			top = &goals[i]
		}
	}
	return top
}

func calcPayoutsNeededFromList(remaining float64, payouts []domain.Payout, allocs []domain.Allocation) int {
	if remaining <= 0 {
		return 0
	}
	if len(payouts) == 0 {
		return -1
	}
	total := 0.0
	for _, p := range payouts {
		total += p.NetAmount
	}
	avg := total / float64(len(payouts))
	if avg <= 0 {
		return -1
	}
	personalPct := 0.10
	for _, a := range allocs {
		if a.Category == "personal" || a.Category == "savings" {
			personalPct += a.Percentage / 100
		}
	}
	if personalPct > 0 {
		perPayout := avg * personalPct
		if perPayout > 0 {
			return int(remaining/perPayout) + 1
		}
	}
	return -1
}

// ─── Holdings ────────────────────────────────────────────

func (s *PGStore) ListHoldings() []domain.Holding {
	rows, err := s.db.Query(`SELECT id, symbol, quantity, avg_cost, acquired_at, notes, created_at, updated_at FROM holdings ORDER BY symbol`)
	if err != nil {
		s.logger.Error("list holdings", "error", err)
		return nil
	}
	defer rows.Close()

	var out []domain.Holding
	for rows.Next() {
		var h domain.Holding
		var acq, notes sql.NullString
		if err := rows.Scan(&h.ID, &h.Symbol, &h.Quantity, &h.AvgCost, &acq, &notes, &h.CreatedAt, &h.UpdatedAt); err != nil {
			s.logger.Error("scan holding", "error", err)
			continue
		}
		h.AcquiredAt = acq.String
		h.Notes = notes.String
		out = append(out, h)
	}
	return out
}

func (s *PGStore) CreateHolding(h *domain.Holding) error {
	now := time.Now()
	h.CreatedAt = now
	h.UpdatedAt = now
	_, err := s.db.Exec(`INSERT INTO holdings (id, symbol, quantity, avg_cost, acquired_at, notes, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		h.ID, h.Symbol, h.Quantity, h.AvgCost, nullStr(h.AcquiredAt), nullStr(h.Notes), h.CreatedAt, h.UpdatedAt)
	return err
}

func (s *PGStore) UpdateHolding(h *domain.Holding) error {
	h.UpdatedAt = time.Now()
	res, err := s.db.Exec(`UPDATE holdings SET symbol=$2, quantity=$3, avg_cost=$4, acquired_at=$5, notes=$6, updated_at=$7 WHERE id=$1`,
		h.ID, h.Symbol, h.Quantity, h.AvgCost, nullStr(h.AcquiredAt), nullStr(h.Notes), h.UpdatedAt)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("holding %q not found", h.ID)
	}
	return nil
}

func (s *PGStore) DeleteHolding(id string) error {
	res, err := s.db.Exec(`DELETE FROM holdings WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("holding %q not found", id)
	}
	return nil
}

// ─── Commands (Engine Protocol) ────────────────────────────

func (s *PGStore) CreateCommand(c *domain.Command) error {
	now := time.Now()
	c.CreatedAt = now
	c.UpdatedAt = now
	params, _ := json.Marshal(c.Params)
	_, err := s.db.Exec(`INSERT INTO commands (id, engine_id, command, scope, params, status, error_message, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		c.ID, c.EngineID, c.Command, c.Scope, params, c.Status, nullStr(c.Error), c.CreatedAt, c.UpdatedAt)
	return err
}

func (s *PGStore) GetCommand(id string) (*domain.Command, error) {
	var c domain.Command
	var params []byte
	var errMsg sql.NullString
	err := s.db.QueryRow(`SELECT id, engine_id, command, scope, params, status, error_message, created_at, updated_at FROM commands WHERE id=$1`, id).
		Scan(&c.ID, &c.EngineID, &c.Command, &c.Scope, &params, &c.Status, &errMsg, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("command %q not found", id)
	}
	json.Unmarshal(params, &c.Params)
	c.Error = errMsg.String
	return &c, nil
}

func (s *PGStore) ListPendingCommands(engineID string) []domain.Command {
	rows, err := s.db.Query(`SELECT id, engine_id, command, scope, params, status, error_message, created_at, updated_at
		FROM commands WHERE engine_id=$1 AND status IN ('pending','acked') ORDER BY created_at`, engineID)
	if err != nil {
		s.logger.Error("list pending commands", "error", err)
		return nil
	}
	defer rows.Close()

	var out []domain.Command
	for rows.Next() {
		var c domain.Command
		var params []byte
		var errMsg sql.NullString
		if err := rows.Scan(&c.ID, &c.EngineID, &c.Command, &c.Scope, &params, &c.Status, &errMsg, &c.CreatedAt, &c.UpdatedAt); err != nil {
			s.logger.Error("scan command", "error", err)
			continue
		}
		json.Unmarshal(params, &c.Params)
		c.Error = errMsg.String
		out = append(out, c)
	}
	return out
}

func (s *PGStore) UpdateCommand(c *domain.Command) error {
	c.UpdatedAt = time.Now()
	res, err := s.db.Exec(`UPDATE commands SET status=$2, error_message=$3, updated_at=$4 WHERE id=$1`,
		c.ID, c.Status, nullStr(c.Error), c.UpdatedAt)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("command %q not found", c.ID)
	}
	return nil
}

func nullStr(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// ─── Helpers ────────────────────────────────────────────

func distributeWithdrawal(amount float64, allocs []domain.Allocation) []domain.Allocation {
	if len(allocs) == 0 {
		allocs = DefaultAllocations()
	}
	result := make([]domain.Allocation, len(allocs))
	for i, a := range allocs {
		result[i] = domain.Allocation{
			Category:   a.Category,
			Percentage: a.Percentage,
			Amount:     amount * (a.Percentage / 100),
		}
	}
	return result
}
