// Command epilayer-credential-provider is a kubelet image credential provider
// that returns a token read from a file on the node as registry credentials.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"time"
)

var version = "dev"

func parseFlags(args []string, stderr io.Writer) (config, bool, error) {
	fs := flag.NewFlagSet("epilayer-credential-provider", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var cfg config
	fs.StringVar(&cfg.TokenFile, "token-file", "", "path to the file holding the registry token (required)")
	fs.StringVar(&cfg.Username, "username", "", "username to return with the token (required)")
	fs.DurationVar(&cfg.CacheDuration, "cache-duration", time.Minute, "how long kubelet may cache the credentials; keep it short so rotated tokens are picked up")
	showVersion := fs.Bool("version", false, "print the version and exit")
	if err := fs.Parse(args); err != nil {
		return cfg, false, err
	}
	if *showVersion {
		return cfg, true, nil
	}
	if fs.NArg() > 0 {
		return cfg, false, fmt.Errorf("unexpected arguments: %v", fs.Args())
	}
	if cfg.TokenFile == "" {
		return cfg, false, fmt.Errorf("--token-file is required")
	}
	if cfg.Username == "" {
		return cfg, false, fmt.Errorf("--username is required")
	}
	if cfg.CacheDuration < 0 {
		return cfg, false, fmt.Errorf("--cache-duration must not be negative")
	}
	return cfg, false, nil
}

func main() {
	cfg, showVersion, err := parseFlags(os.Args[1:], os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if showVersion {
		fmt.Println(version)
		return
	}
	if err := run(cfg, os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
