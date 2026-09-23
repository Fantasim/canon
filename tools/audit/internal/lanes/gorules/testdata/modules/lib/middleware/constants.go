package middleware

const HeaderAuthorization = "Authorization"

// TOTPState is a second-factor state.
type TOTPState string

const (
	TOTPEnabled  TOTPState = "enabled"
	TOTPDisabled TOTPState = "disabled"
)
