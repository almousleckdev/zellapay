package ledger

// AccountType classifies an account in the chart of accounts.
type AccountType string

// Account types of the chart of accounts.
const (
	Asset     AccountType = "asset"
	Liability AccountType = "liability"
	Revenue   AccountType = "revenue"
	Expense   AccountType = "expense"
)

// Valid reports whether t is a known account type.
func (t AccountType) Valid() bool {
	switch t {
	case Asset, Liability, Revenue, Expense:
		return true
	}
	return false
}

// DebitNormal reports whether the account's balance grows with debits.
func (t AccountType) DebitNormal() bool {
	return t == Asset || t == Expense
}

// Account is a named container of entries in a single currency.
type Account struct {
	ID             string
	Code           string
	Type           AccountType
	Currency       Currency
	AllowOverdraft bool
}

// NormalBalance converts debit and credit totals into the account's
// natural balance: debits minus credits for debit-normal accounts,
// credits minus debits otherwise.
func NormalBalance(t AccountType, debits, credits int64) int64 {
	if t.DebitNormal() {
		return debits - credits
	}
	return credits - debits
}
