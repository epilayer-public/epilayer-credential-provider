package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeToken(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func requestJSON(image string) string {
	return `{"apiVersion":"credentialprovider.kubelet.k8s.io/v1","kind":"CredentialProviderRequest","image":"` + image + `"}`
}

func TestRun(t *testing.T) {
	cfg := config{
		TokenFile:     writeToken(t, "s3cr3t\n"),
		Username:      "k8s",
		CacheDuration: time.Minute,
	}
	var out bytes.Buffer
	err := run(cfg, strings.NewReader(requestJSON("registry.example.com/project-a/app:1.0")), &out)
	if err != nil {
		t.Fatal(err)
	}

	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"apiVersion":    "credentialprovider.kubelet.k8s.io/v1",
		"kind":          "CredentialProviderResponse",
		"cacheKeyType":  "Registry",
		"cacheDuration": "1m0s",
		"auth": map[string]any{
			"registry.example.com": map[string]any{
				"username": "k8s",
				"password": "s3cr3t",
			},
		},
	}
	gotJSON, _ := json.Marshal(got)
	wantJSON, _ := json.Marshal(want)
	if !bytes.Equal(gotJSON, wantJSON) {
		t.Errorf("response\n got: %s\nwant: %s", gotJSON, wantJSON)
	}
}

func TestRunReadsTokenOnEveryCall(t *testing.T) {
	path := writeToken(t, "old")
	cfg := config{TokenFile: path, Username: "u"}
	req := requestJSON("registry.example.com/p/app")

	if err := run(cfg, strings.NewReader(req), io.Discard); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := run(cfg, strings.NewReader(req), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"password":"new"`) {
		t.Errorf("rotated token not returned: %s", out.String())
	}
}

func TestRunZeroCacheDuration(t *testing.T) {
	cfg := config{TokenFile: writeToken(t, "t"), Username: "u"}
	var out bytes.Buffer
	if err := run(cfg, strings.NewReader(requestJSON("registry.example.com/p/app")), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"cacheDuration":"0s"`) {
		t.Errorf("want cacheDuration 0s: %s", out.String())
	}
}

func TestRunErrors(t *testing.T) {
	validToken := writeToken(t, "t")
	tests := []struct {
		name      string
		tokenFile string
		input     string
		wantErr   string
	}{
		{"invalid json", validToken, "{", "decoding request"},
		{"wrong apiVersion", validToken, `{"apiVersion":"credentialprovider.kubelet.k8s.io/v1beta1","kind":"CredentialProviderRequest","image":"r.example.com/a"}`, "unsupported apiVersion"},
		{"wrong kind", validToken, `{"apiVersion":"credentialprovider.kubelet.k8s.io/v1","kind":"Other","image":"r.example.com/a"}`, "unsupported kind"},
		{"no image", validToken, `{"apiVersion":"credentialprovider.kubelet.k8s.io/v1","kind":"CredentialProviderRequest"}`, "no image"},
		{"no registry host", validToken, requestJSON("library/nginx"), "no registry host"},
		{"missing token file", filepath.Join(t.TempDir(), "absent"), requestJSON("r.example.com/a"), "reading token"},
		{"empty token file", writeToken(t, " \n"), requestJSON("r.example.com/a"), "is empty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			err := run(config{TokenFile: tt.tokenFile, Username: "u"}, strings.NewReader(tt.input), &out)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
			}
			if out.Len() != 0 {
				t.Errorf("wrote output on error: %s", out.String())
			}
		})
	}
}

func TestRegistryHost(t *testing.T) {
	tests := map[string]string{
		"registry.example.com/p/app:1":         "registry.example.com",
		"registry.example.com:5000/p/app":      "registry.example.com:5000",
		"localhost/p/app":                      "localhost",
		"[::1]:5000/p/app@sha256:0123456789ab": "[::1]:5000",
	}
	for image, want := range tests {
		got, err := registryHost(image)
		if err != nil {
			t.Errorf("registryHost(%q): %v", image, err)
			continue
		}
		if got != want {
			t.Errorf("registryHost(%q) = %q, want %q", image, got, want)
		}
	}
	for _, image := range []string{"nginx", "library/nginx", "/p/app"} {
		if _, err := registryHost(image); err == nil {
			t.Errorf("registryHost(%q): want error", image)
		}
	}
}

func TestParseFlags(t *testing.T) {
	cfg, showVersion, err := parseFlags([]string{"--token-file", "/t", "--username", "u", "--cache-duration", "30s"}, io.Discard)
	if err != nil || showVersion {
		t.Fatalf("err = %v, showVersion = %v", err, showVersion)
	}
	if cfg != (config{TokenFile: "/t", Username: "u", CacheDuration: 30 * time.Second}) {
		t.Errorf("cfg = %+v", cfg)
	}

	cfg, _, err = parseFlags([]string{"--token-file", "/t", "--username", "u"}, io.Discard)
	if err != nil || cfg.CacheDuration != time.Minute {
		t.Errorf("default cache duration: cfg = %+v, err = %v", cfg, err)
	}

	if _, showVersion, err := parseFlags([]string{"--version"}, io.Discard); err != nil || !showVersion {
		t.Errorf("--version: showVersion = %v, err = %v", showVersion, err)
	}

	for _, args := range [][]string{
		{"--username", "u"},
		{"--token-file", "/t"},
		{"--token-file", "/t", "--username", "u", "extra"},
		{"--token-file", "/t", "--username", "u", "--cache-duration", "-1s"},
		{"--unknown"},
	} {
		if _, _, err := parseFlags(args, io.Discard); err == nil {
			t.Errorf("parseFlags(%v): want error", args)
		}
	}
}
