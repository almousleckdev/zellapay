package ledger

// Naming of well-known accounts. Account IDs are stable strings so that
// every module refers to the same account without sharing state.

// Fees collects ZellaPay's fee revenue in one currency.
func Fees(currency string) string { return "zella:fees:" + currency }

// OperatorSettlement is what an operator owes ZellaPay.
func OperatorSettlement(operatorCode string) string {
	return "operator:" + operatorCode + ":settlement"
}

// MerchantPending holds collected funds not yet released for payout.
func MerchantPending(merchantID string) string {
	return "merchant:" + merchantID + ":pending"
}

// MerchantHold holds funds reserved for payouts in progress.
func MerchantHold(merchantID string) string {
	return "merchant:" + merchantID + ":hold"
}

// MerchantAvailable holds funds a merchant may pay out.
func MerchantAvailable(merchantID string) string {
	return "merchant:" + merchantID + ":available"
}

// Transfer returns the two entries that move amount from one account to
// another: a debit of from and a credit of to. Which accounts grow or shrink
// depends on their types; see NormalBalance.
func Transfer(from, to string, amount int64, currency Currency) []Entry {
	return []Entry{
		{Account: from, Side: Debit, Amount: amount, Currency: currency},
		{Account: to, Side: Credit, Amount: amount, Currency: currency},
	}
}
