package db

// Store is the database.
type Store struct{}

// Namer names things.
type Namer interface{ Name() string }

// Describe names n.
func Describe(n Namer) string { return n.Name() + "application/json" }
