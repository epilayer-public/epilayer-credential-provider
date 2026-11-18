package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

const (
	apiVersion   = "credentialprovider.kubelet.k8s.io/v1"
	requestKind  = "CredentialProviderRequest"
	responseKind = "CredentialProviderResponse"

	cacheKeyTypeRegistry = "Registry"
)

// request is the subset of credentialprovider.kubelet.k8s.io/v1
// CredentialProviderRequest that this provider reads.
type request struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Image      string `json:"image"`
}

type authConfig struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// response is credentialprovider.kubelet.k8s.io/v1 CredentialProviderResponse.
type response struct {
	APIVersion    string                `json:"apiVersion"`
	Kind          string                `json:"kind"`
	CacheKeyType  string                `json:"cacheKeyType"`
	CacheDuration string                `json:"cacheDuration"`
	Auth          map[string]authConfig `json:"auth"`
}

type config struct {
	TokenFile     string
	Username      string
	CacheDuration time.Duration
}

func readRequest(r io.Reader) (request, error) {
	var req request
	dec := json.NewDecoder(r)
	if err := dec.Decode(&req); err != nil {
		return req, fmt.Errorf("decoding request: %w", err)
	}
	if req.APIVersion != apiVersion {
		return req, fmt.Errorf("unsupported apiVersion %q, want %q", req.APIVersion, apiVersion)
	}
	if req.Kind != requestKind {
		return req, fmt.Errorf("unsupported kind %q, want %q", req.Kind, requestKind)
	}
	if req.Image == "" {
		return req, errors.New("request has no image")
	}
	return req, nil
}

// registryHost returns the registry part of an image reference, including
// a port if there is one.
func registryHost(image string) (string, error) {
	host, _, found := strings.Cut(image, "/")
	if !found || host == "" {
		return "", fmt.Errorf("image %q has no registry host", image)
	}
	if !strings.ContainsAny(host, ".:") && host != "localhost" {
		return "", fmt.Errorf("image %q has no registry host", image)
	}
	return host, nil
}

func readToken(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading token: %w", err)
	}
	token := strings.TrimSpace(string(data))
	if token == "" {
		return "", fmt.Errorf("token file %s is empty", path)
	}
	return token, nil
}

func buildResponse(cfg config, req request) (response, error) {
	host, err := registryHost(req.Image)
	if err != nil {
		return response{}, err
	}
	token, err := readToken(cfg.TokenFile)
	if err != nil {
		return response{}, err
	}
	resp := response{
		APIVersion:    apiVersion,
		Kind:          responseKind,
		CacheKeyType:  cacheKeyTypeRegistry,
		CacheDuration: cfg.CacheDuration.String(),
		Auth: map[string]authConfig{
			host: {Username: cfg.Username, Password: token},
		},
	}
	return resp, nil
}

func run(cfg config, in io.Reader, out io.Writer) error {
	req, err := readRequest(in)
	if err != nil {
		return err
	}
	resp, err := buildResponse(cfg, req)
	if err != nil {
		return err
	}
	return json.NewEncoder(out).Encode(resp)
}
