package api

const (
	contentJSON = "application/json; charset=utf-8"
	kindUser    = "user"
	statusLive  = "active"
	MaxThing    = 3
)

const (
	StatusPending = "pending"
	StatusRunning = "running"
)

const aliasJSON = contentJSON

const orderBy = " ORDER BY "

func limit() int { return 99 }

var _, _ = limit, orderBy

var routes = map[string]string{"alpha": "alpha", "beta": "beta"}

// TOTPState mirrors the library's.
type TOTPState string

const TOTPEnabled TOTPState = "enabled"
