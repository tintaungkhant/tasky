package cmd

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tintaungkhant/tasky/internal/config"
	"github.com/tintaungkhant/tasky/internal/registry"
	"github.com/tintaungkhant/tasky/internal/web"
)

func TestServeCommandIsAvailable(t *testing.T) {
	command, _, err := rootCmd.Find([]string{"serve"})
	if err != nil || command == rootCmd || command.Name() != "serve" {
		t.Fatalf("serve command is missing: %v", err)
	}
	if flag := command.Flags().Lookup("port"); flag == nil || flag.DefValue != "9000" {
		t.Fatal("serve must provide a port selector with default 9000")
	}
	if command.Flags().Lookup("address") != nil || command.Flags().Lookup("host") != nil {
		t.Fatal("serve must not expose non-loopback binding")
	}
}

type serveOutput chan string

func (out serveOutput) Write(p []byte) (int, error) {
	out <- string(p)
	return len(p), nil
}

func TestServeCommandPrintsAccessLinkAndStops(t *testing.T) {
	t.Setenv("TASKY_CONFIG", filepath.Join(t.TempDir(), "config.yaml"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	command := newServeCommand()
	command.SetContext(ctx)
	command.SetArgs([]string{"--port", "0"})
	out := make(serveOutput, 1)
	command.SetOut(out)
	command.SetErr(io.Discard)
	done := make(chan error, 1)
	go func() { done <- command.Execute(); close(done) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Error("serve did not stop")
		}
	})
	select {
	case output := <-out:
		line, _, _ := strings.Cut(output, "\n")
		link, err := url.Parse(strings.TrimPrefix(line, "Dashboard: "))
		if err != nil || link.Hostname() != "127.0.0.1" || link.Port() == "0" || !strings.HasPrefix(link.Fragment, "token=") {
			t.Fatal("serve did not print a valid loopback access link")
		}
	case err := <-done:
		t.Fatalf("serve exited before startup: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("serve did not announce startup")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("shutdown: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("serve did not stop after cancellation")
	}
}

func TestServeCommandRejectsInvalidFlags(t *testing.T) {
	for _, args := range [][]string{{"--port", "-1"}, {"--port", "65536"}, {"--host", "0.0.0.0"}, {"unexpected"}} {
		command := newServeCommand()
		command.SetOut(io.Discard)
		command.SetErr(io.Discard)
		command.SetArgs(args)
		if err := command.Execute(); err == nil {
			t.Fatalf("accepted invalid flags: %v", args)
		}
	}
}

func TestServeHTTPResolvesSharedCLIContext(t *testing.T) {
	root, repo, _ := storedSetup(t)
	configPath, err := config.Path()
	if err != nil {
		t.Fatal(err)
	}
	chdir(t, t.TempDir())
	service, err := web.New(configPath, repo, 0)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- service.Serve(ctx) }()
	t.Cleanup(func() {
		cancel()
		service.Close()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("HTTP shutdown: %v", err)
			}
		case <-time.After(10 * time.Second):
			t.Error("HTTP service did not stop")
		}
	})
	link, err := url.Parse(service.URL())
	if err != nil {
		t.Fatal("invalid access link")
	}
	fragment, err := url.ParseQuery(link.Fragment)
	if err != nil {
		t.Fatal("invalid access fragment")
	}
	base := link.Scheme + "://" + link.Host
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Timeout: 5 * time.Second, Jar: jar}
	defer client.CloseIdleConnections()
	payload, _ := json.Marshal(map[string]string{"token": fragment.Get("token")})
	request, err := http.NewRequest(http.MethodPost, base+"/api/session", strings.NewReader(string(payload)))
	if err != nil {
		t.Fatal("invalid session request")
	}
	request.Header.Set("Origin", base)
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		t.Fatal("session request failed")
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("session status: %d", response.StatusCode)
	}
	var first registry.Context
	for _, query := range []string{"?root=company&repo=project-a", ""} {
		response, err := client.Get(base + "/api/context" + query)
		if err != nil {
			t.Fatal("context request failed")
		}
		var selected registry.Context
		err = json.NewDecoder(response.Body).Decode(&selected)
		response.Body.Close()
		if err != nil || response.StatusCode != http.StatusOK {
			t.Fatalf("context response: %d %v", response.StatusCode, err)
		}
		if selected.Root.Path != root || selected.Repo.Path != repo || selected.Root.ID == "" || selected.Repo.ID == "" {
			t.Fatalf("wrong shared context: %+v", selected)
		}
		if query != "" {
			first = selected
		} else if selected != first {
			t.Fatal("captured-directory context differs from explicit selection")
		}
	}
}
