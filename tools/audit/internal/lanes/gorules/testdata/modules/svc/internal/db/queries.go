package db

import "time"

// listUsersQuery is a whole statement: it stays beside the function that runs it.
const listUsersQuery = `
	-- the active users, newest first
	select id, name FROM users WHERE state = $1`

// insertUserQuery is exempt (another package's constant folds as an operand), usersOrder is a
// fragment: the block counts one.
const (
	insertUserQuery = "INSERT INTO users (name, at) " + "VALUES ($1, " + time.RFC3339 + ")"
	usersOrder      = " ORDER BY name"
)

// pickServer is prose, not a statement.
const pickServer = "Select a server"

// lockUsers is a package-level literal var holding a statement: exempt.
var lockUsers = "LOCK TABLE users IN SHARE MODE"

func listUsers() []string {
	return []string{listUsersQuery, insertUserQuery, usersOrder, pickServer, lockUsers}
}
