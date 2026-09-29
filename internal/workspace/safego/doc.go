// Package safego starts goroutines whose panic is recovered and handed back as an error, the
// one helper the code doctrine allows a go statement in (tools/audit/rules.md bare-goroutine).
package safego
