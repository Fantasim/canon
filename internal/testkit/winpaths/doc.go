// Package winpaths models how Go's filepath reads a Windows name, so a test on any system runs the
// volume rules of Windows: VolumeName is filepath.VolumeName as Windows returns it, its
// backslashes included, for names written with either separator. It is a port of Go 1.25's
// volumeNameLen (internal/filepathlite, path_windows.go): drive letters, UNC shares, and the
// device paths `\\.\`, `\\?\` and `\??\`. Feed it to project.NewPaths to get the FS names the
// compiler compares.
package winpaths
