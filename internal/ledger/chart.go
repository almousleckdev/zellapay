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

// MerchantAvailable holds funds a merchant may pay out.
func MerchantAvailable(merchantID string) string {
	return "merchant:" + merchantID + ":available"
}
