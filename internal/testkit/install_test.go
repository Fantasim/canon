package testkit

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const (
	installScript = "../../tools/install.sh"
	oldVersion    = "0.9.0"
	newVersion    = "0.9.1"
	otherFile     = "other"
	dirMode       = 0o700
	fileMode      = 0o600
	skipNotLinux  = "install.sh is Linux only"
)

// fakeBinary is a canon that prints its version.
func fakeBinary(version string) string { return "#!/bin/sh\necho \"canon " + version + " (fake)\"\n" }

// archContent makes each arch's binary differ, so a test can tell which one was installed.
func archContent(content, arch string) string { return content + "# " + arch + "\n" }

// hostBinary is the fake binary of version for this host's arch.
func hostBinary(version string) string { return archContent(fakeBinary(version), runtime.GOARCH) }

var fakeArches = [...]string{"amd64", "arm64"}

// release is one fake release: its version, the archive member's name and content, and whether
// its checksums.txt lists the archives.
type release struct {
	version, member, content string
	unlisted, latest         bool
}

// publish writes r under root/download/v<version>/, and also under root/latest/download/ when
// it is the latest, for both arches so the test does not depend on the host's.
func (r release) publish(t *testing.T, root string) {
	t.Helper()
	dirs := []string{filepath.Join(root, "download", "v"+r.version)}
	if r.latest {
		dirs = append(dirs, filepath.Join(root, "latest", "download"))
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, dirMode); err != nil {
			t.Fatal(err)
		}
		var sums strings.Builder
		for _, arch := range fakeArches {
			name := "canon_" + r.version + "_linux_" + arch + ".tar.gz"
			stage := t.TempDir()
			if err := os.WriteFile(filepath.Join(stage, r.member), []byte(archContent(r.content, arch)), dirMode); err != nil {
				t.Fatal(err)
			}
			run(t, d, "tar", "-czf", name, "-C", stage, r.member)
			if !r.unlisted {
				sums.WriteString(run(t, d, "sha256sum", name))
			}
		}
		if err := os.WriteFile(filepath.Join(d, "checksums.txt"), []byte(sums.String()), fileMode); err != nil {
			t.Fatal(err)
		}
	}
}

// run executes a helper command in dir and returns its output.
func run(t *testing.T, dir, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
	return string(out)
}

// install runs tools/install.sh with env on top of a clean one, returning its combined output.
func install(t *testing.T, env []string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("sh", append([]string{installScript}, args...)...)
	cmd.Env = append([]string{"PATH=" + os.Getenv("PATH"), "HOME=" + t.TempDir()}, env...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// installed reads what the script installed into dir.
func installed(dir string) string {
	got, err := os.ReadFile(filepath.Join(dir, "canon"))
	if err != nil {
		return ""
	}
	return string(got)
}

// TestInstallScript covers CLI.md §7 (decision 276); the wget path is untested, it cannot read file://.
func TestInstallScript(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip(skipNotLinux)
	}
	if runtime.GOARCH != fakeArches[0] && runtime.GOARCH != fakeArches[1] {
		t.Skip(skipNotLinux)
	}
	root := t.TempDir()
	release{version: oldVersion, member: "canon", content: fakeBinary(oldVersion)}.publish(t, root)
	release{version: newVersion, member: "canon", content: fakeBinary(newVersion), latest: true}.publish(t, root)
	dir := filepath.Join(t.TempDir(), "bin")
	env := []string{"CANON_RELEASE_BASE=file://" + root, "CANON_INSTALL_DIR=" + dir}

	cases := []struct {
		name    string
		env     []string
		args    []string
		wantErr bool
		want    string // in the output
		binary  string // what dir/canon holds afterwards, "" when not checked
	}{
		{"explicit version", env, []string{"v" + oldVersion}, false, "canon " + oldVersion, hostBinary(oldVersion)},
		{"latest updates the installed one", env, nil, false, "canon " + newVersion, hostBinary(newVersion)},
		{"version without v", env, []string{oldVersion}, false, "canon " + oldVersion, hostBinary(oldVersion)},
		{"version from the environment", append([]string{"CANON_VERSION=v" + newVersion}, env...), nil, false, "canon " + newVersion, hostBinary(newVersion)},
		{"off PATH warns", env, nil, false, "is not on your PATH", ""},
		{"unknown version", env, []string{"v8.8.8"}, true, "cannot download", ""},
		{"bad version", env, []string{"nightly"}, true, "bad version", ""},
		{"bad version, two lines", env, []string{"v1.0.0\njunk"}, true, "bad version", ""},
		{"bad version, trailing newline", env, []string{"v1.0.0\n"}, true, "bad version", ""},
		{"bad version, path-like", env, []string{"v1.0.0/../x"}, true, "bad version", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := install(t, tc.env, tc.args...)
			if (err != nil) != tc.wantErr || !strings.Contains(out, tc.want) {
				t.Fatalf("err = %v, output %q: want error %v containing %q", err, out, tc.wantErr, tc.want)
			}
			if tc.binary != "" && installed(dir) != tc.binary {
				t.Fatalf("installed canon = %q, want %q", installed(dir), tc.binary)
			}
		})
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Fatalf("the install dir holds %d entries, want canon alone (no staging file left)", len(entries))
	}
}

// TestInstallScriptRefusals: each bad release is refused and nothing is installed (decision 276).
func TestInstallScriptRefusals(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip(skipNotLinux)
	}
	cases := []struct {
		name   string
		r      release
		tamper bool
		latest bool
		want   string
	}{
		{"no release yet", release{}, false, true, "is there a release yet"},
		{"checksum mismatch", release{member: "canon", content: fakeBinary(oldVersion)}, true, false, "checksum mismatch"},
		{"not listed in checksums.txt", release{member: "canon", content: fakeBinary(oldVersion), unlisted: true}, false, false, "is not listed"},
		{"archive without canon", release{member: otherFile, content: "x"}, false, false, "holds no canon binary"},
		{"binary that does not run", release{member: "canon", content: "#!/nonexistent\n"}, false, false, "does not run"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			args := []string{"v" + oldVersion}
			if tc.latest {
				args = nil
			} else {
				tc.r.version = oldVersion
				tc.r.publish(t, root)
			}
			if tc.tamper {
				for _, a := range fakeArches {
					name := filepath.Join(root, "download", "v"+oldVersion, "canon_"+oldVersion+"_linux_"+a+".tar.gz")
					if err := os.WriteFile(name, []byte("tampered"), fileMode); err != nil {
						t.Fatal(err)
					}
				}
			}
			dir := filepath.Join(t.TempDir(), "bin")
			out, err := install(t, []string{"CANON_RELEASE_BASE=file://" + root, "CANON_INSTALL_DIR=" + dir}, args...)
			if err == nil || !strings.Contains(out, tc.want) {
				t.Fatalf("err = %v, output %q: want a failure containing %q", err, out, tc.want)
			}
			if installed(dir) != "" {
				t.Fatal("canon was installed despite the refusal")
			}
			if entries, _ := os.ReadDir(dir); len(entries) != 0 {
				t.Fatalf("the install dir holds %d entries, want none", len(entries))
			}
		})
	}
}

// TestMakeRefusesInjectedVersion: a backtick or $(shell) VERSION is refused unexpanded (decision 276).
func TestMakeRefusesInjectedVersion(t *testing.T) {
	if _, err := exec.LookPath("make"); err != nil {
		t.Skip("make is not installed")
	}
	for _, target := range []string{"dist", "tag"} {
		for _, payload := range []string{"v1`touch %s`", "v1.0.0$(shell touch %s)", "v1.0.0;touch %s", "v1.0.0$$(touch %s)", "v1.0.0\njunk %s"} {
			t.Run(target+" "+payload, func(t *testing.T) {
				// t.TempDir's name holds the subtest's name, `$(` included: use a plain one.
				dir, err := os.MkdirTemp("", "marker")
				if err != nil {
					t.Fatal(err)
				}
				defer os.RemoveAll(dir)
				marker := filepath.Join(dir, "PWNED")
				cmd := exec.Command("make", target, "VERSION="+strings.ReplaceAll(payload, "%s", marker))
				cmd.Dir = "../.."
				out, err := cmd.CombinedOutput()
				if err == nil {
					t.Fatalf("make %s accepted %q: %s", target, payload, out)
				}
				if _, statErr := os.Stat(marker); statErr == nil {
					t.Fatalf("make %s ran the payload %q", target, payload)
				}
			})
		}
	}
}
