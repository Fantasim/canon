package build

// syncDir is nil: Windows cannot sync a directory, and makes a rename durable by itself.
func syncDir(string) error { return nil }
