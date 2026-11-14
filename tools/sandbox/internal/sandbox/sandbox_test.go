package sandbox

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRemoteCommand(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"one argument is a shell line", []string{"echo $HOME && ls"}, "echo $HOME && ls"},
		{"plain words stay bare", []string{"sw_vers", "-productVersion"}, "sw_vers -productVersion"},
		{"spaces and shell characters are quoted", []string{"printf", "%s|", "a b", "$HOME", "*"}, `printf '%s|' 'a b' '$HOME' '*'`},
		{"single quotes survive", []string{"echo", "it's"}, `echo 'it'\''s'`},
		{"empty arguments survive", []string{"echo", ""}, "echo ''"},
	}
	for _, c := range cases {
		if got := RemoteCommand(c.args); got != c.want {
			t.Errorf("%s: got %s, want %s", c.name, got, c.want)
		}
	}
}

func TestSealKeyOpensWhatItsPublicKeySealed(t *testing.T) {
	key, err := newSealKey()
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := key.Public()
	if err != nil {
		t.Fatal(err)
	}

	pemBytes, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode(pemBytes)
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, parsed.(*rsa.PublicKey), []byte("four-words-here.trycloudflare.com"), nil)
	if err != nil {
		t.Fatal(err)
	}

	opened, err := key.Open(base64.StdEncoding.EncodeToString(sealed))
	if err != nil {
		t.Fatal(err)
	}
	if opened != "four-words-here.trycloudflare.com" {
		t.Errorf("opened %q", opened)
	}

	other, err := newSealKey()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := other.Open(base64.StdEncoding.EncodeToString(sealed)); err == nil {
		t.Error("a different key opened the sealed address")
	}
}

func TestAnnouncementValidate(t *testing.T) {
	valid := Announcement{
		ID:        "sbx_abcd2345",
		Transport: "cloudflare-quick",
		User:      "runner",
		HostKey:   "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIF+02Lxuodp429ovGwiWNYhiADLRfo2bQY4nlYwpUKqR",
	}
	if err := valid.Validate("sbx_abcd2345", "cloudflare-quick"); err != nil {
		t.Fatalf("valid announcement rejected: %s", err)
	}

	cases := map[string]func(a *Announcement){
		"another sandbox's announcement": func(a *Announcement) { a.ID = "sbx_zzzz2345" },
		"another transport":              func(a *Announcement) { a.Transport = "tailscale" },
		"a user that is an ssh option":   func(a *Announcement) { a.User = "-oProxyCommand=x" },
		"a host key with a second entry": func(a *Announcement) { a.HostKey += "\n* ssh-ed25519 AAAA" },
		"a host key of another type":     func(a *Announcement) { a.HostKey = "ssh-rsa AAAA" },
	}
	for name, corrupt := range cases {
		announcement := valid
		corrupt(&announcement)
		if announcement.Validate("sbx_abcd2345", "cloudflare-quick") == nil {
			t.Errorf("%s was accepted", name)
		}
	}
}

func TestDescribe(t *testing.T) {
	stopAt := time.Unix(1_800_000_000, 0)
	record := Record{ID: "sbx_abcd2345", Transport: "cloudflare-quick", Ready: true, AutoStopAt: stopAt, HardStopAt: stopAt}

	ready := describe(Machine{ID: "sbx_abcd2345", State: StateReady}, "github-actions", record, true)
	if ready.AutoStopAt == nil || !ready.AutoStopAt.Equal(stopAt) || !ready.HasKey {
		t.Errorf("a ready sandbox with a local key should show its auto-stop time, got %+v", ready)
	}

	elsewhere := describe(Machine{ID: "sbx_abcd2345", State: StateReady}, "github-actions", Record{}, false)
	if elsewhere.AutoStopAt != nil || elsewhere.HasKey {
		t.Errorf("a sandbox created elsewhere has no local details, got %+v", elsewhere)
	}

	stopped := describe(Machine{ID: "sbx_abcd2345", State: StateStopped}, "github-actions", record, true)
	if stopped.AutoStopAt != nil {
		t.Errorf("a stopped sandbox has no auto-stop time, got %+v", stopped)
	}
}

func TestNewIDIsAnID(t *testing.T) {
	for range 200 {
		if id := newID(); !IsID(id) {
			t.Fatalf("%q is not a sandbox ID", id)
		}
	}
}

func TestReadSSHLog(t *testing.T) {
	cases := []struct {
		name        string
		log         string
		hasLoggedIn bool
		complaints  string
	}{
		{"a clean run", "Authenticated to host (via proxy) using \"publickey\".\nTransferred: sent 1, received 2 bytes, in 0.1 seconds\nBytes per second: sent 1, received 2\n", true, ""},
		{"a stall before login", "Connection to UNKNOWN port 65535 timed out\n", false, "Connection to UNKNOWN port 65535 timed out\n"},
		{"a drop after login", "Authenticated to host (via proxy) using \"publickey\".\nTimeout, server host not responding.\n", true, "Timeout, server host not responding.\n"},
		{"a remote line that only looks like a login", "scp: Authenticated to nothing\n", false, "scp: Authenticated to nothing\n"},
	}
	for _, c := range cases {
		path := filepath.Join(t.TempDir(), "ssh.log")
		if err := os.WriteFile(path, []byte(c.log), 0o600); err != nil {
			t.Fatal(err)
		}
		result, err := readSSHLog(path)
		if err != nil {
			t.Fatal(err)
		}
		if result.HasLoggedIn != c.hasLoggedIn || result.Complaints != c.complaints {
			t.Errorf("%s: got %+v", c.name, result)
		}
	}
}
