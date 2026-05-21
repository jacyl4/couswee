package accounts

// AccountStore is the persistence boundary used by Service. SQLiteStore is the production implementation.
type AccountStore interface {
	Accounts() []Account
	Replace([]Account) error
	Mutate(func([]Account) ([]Account, error)) error
}
