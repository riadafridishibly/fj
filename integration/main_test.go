//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/testcontainers/testcontainers-go"
	tcexec "github.com/testcontainers/testcontainers-go/exec"
	"github.com/testcontainers/testcontainers-go/wait"
)

var (
	forgejoHost string // "localhost:PORT"
	forgejoURL  string // "http://localhost:PORT"
	fjBinary    string
	configDir   string
	testClient  *forgejo.Client

	adminUser  = "testadmin"
	adminPass  = "T3stP@ssw0rd!"
	adminEmail = "admin@test.local"
)

func projectRoot() string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("cannot determine project root")
	}
	return filepath.Dir(filepath.Dir(filename))
}

func TestMain(m *testing.M) {
	ctx := context.Background()

	tmpDir, err := os.MkdirTemp("", "fj-integration-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	// Build the fj binary
	fjBinary = filepath.Join(tmpDir, "fj")
	fmt.Fprintln(os.Stderr, "Building fj binary...")
	buildCmd := exec.Command("go", "build", "-o", fjBinary, "./cmd/fj")
	buildCmd.Dir = projectRoot()
	if out, err := buildCmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to build fj: %v\n%s\n", err, out)
		os.Exit(1)
	}

	// Check Docker is available
	if out, err := exec.Command("docker", "info").CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "docker not available, skipping integration tests: %v\n%s\n", err, out)
		os.Exit(0)
	}

	// Start Forgejo container
	fmt.Fprintln(os.Stderr, "Starting Forgejo container...")
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "codeberg.org/forgejo/forgejo:14",
			ExposedPorts: []string{"3000/tcp"},
			Env: map[string]string{
				"FORGEJO__security__INSTALL_LOCK":        "true",
				"FORGEJO__database__DB_TYPE":             "sqlite3",
				"FORGEJO__server__DOMAIN":                "localhost",
				"FORGEJO__server__HTTP_PORT":             "3000",
				"FORGEJO__service__DISABLE_REGISTRATION": "true",
				"FORGEJO__log__LEVEL":                    "Warn",
				"USER_UID":                               "1000",
				"USER_GID":                               "1000",
			},
			WaitingFor: wait.ForHTTP("/api/v1/version").
				WithPort("3000/tcp").
				WithStartupTimeout(120 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to start Forgejo container: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		_ = container.Terminate(ctx)
	}()

	// Get connection info
	port, err := container.MappedPort(ctx, "3000/tcp")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get port: %v\n", err)
		os.Exit(1)
	}
	host, err := container.Host(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get host: %v\n", err)
		os.Exit(1)
	}
	forgejoHost = fmt.Sprintf("%s:%s", host, port.Port())
	forgejoURL = fmt.Sprintf("http://%s", forgejoHost)
	fmt.Fprintf(os.Stderr, "Forgejo available at %s\n", forgejoURL)

	// Create admin user via exec
	fmt.Fprintln(os.Stderr, "Creating admin user...")
	exitCode, reader, err := container.Exec(ctx, []string{
		"forgejo", "admin", "user", "create",
		"--admin",
		"--username", adminUser,
		"--password", adminPass,
		"--email", adminEmail,
		"--must-change-password=false",
	}, tcexec.WithUser("git"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to exec admin create: %v\n", err)
		os.Exit(1)
	}
	if exitCode != 0 {
		out, _ := io.ReadAll(reader)
		fmt.Fprintf(os.Stderr, "admin create exited %d: %s\n", exitCode, out)
		os.Exit(1)
	}

	// Create API token via REST (avoids SDK method signature ambiguity)
	fmt.Fprintln(os.Stderr, "Creating API token...")
	token, err := createAPIToken(forgejoURL, adminUser, adminPass)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create API token: %v\n", err)
		os.Exit(1)
	}

	// Create authenticated SDK client
	testClient, err = forgejo.NewClient(forgejoURL, forgejo.SetToken(token))
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create SDK client: %v\n", err)
		os.Exit(1)
	}

	// Write fj config file
	configDir = filepath.Join(tmpDir, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create config dir: %v\n", err)
		os.Exit(1)
	}
	configYAML := fmt.Sprintf("hosts:\n  %s:\n    hostname: %s\n    token: %s\n    user: %s\n    git_protocol: https\n",
		forgejoHost, forgejoHost, token, adminUser)
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(configYAML), 0o600); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write config: %v\n", err)
		os.Exit(1)
	}

	// Populate test data
	fmt.Fprintln(os.Stderr, "Populating test data...")
	if err := populateTestData(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to populate test data: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintln(os.Stderr, "Running tests...")
	os.Exit(m.Run())
}

// createAPIToken uses the REST API with basic auth to create an access token.
func createAPIToken(baseURL, username, password string) (string, error) {
	body := `{"name":"integration-test","scopes":["all"]}`
	req, err := http.NewRequest("POST",
		baseURL+"/api/v1/users/"+username+"/tokens",
		strings.NewReader(body))
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(username, password)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("unexpected status %d: %s", resp.StatusCode, b)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	// Forgejo returns the token in "sha1" or "token" field depending on version
	for _, key := range []string{"sha1", "token"} {
		if v, ok := result[key]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s, nil
			}
		}
	}
	return "", fmt.Errorf("no token in response: %v", result)
}

// populateTestData creates repos, labels, milestones, issues, branches, and PRs.
func populateTestData() error {
	owner := adminUser

	// --- Repositories ---
	_, _, err := testClient.CreateRepo(forgejo.CreateRepoOption{
		Name:        "test-repo",
		Description: "Main test repository",
		AutoInit:    true,
	})
	if err != nil {
		return fmt.Errorf("creating test-repo: %w", err)
	}

	_, _, err = testClient.CreateRepo(forgejo.CreateRepoOption{
		Name:        "another-repo",
		Description: "Secondary test repository",
		AutoInit:    true,
	})
	if err != nil {
		return fmt.Errorf("creating another-repo: %w", err)
	}

	_, _, err = testClient.CreateRepo(forgejo.CreateRepoOption{
		Name:        "private-repo",
		Description: "Private test repository",
		AutoInit:    true,
		Private:     true,
	})
	if err != nil {
		return fmt.Errorf("creating private-repo: %w", err)
	}

	// --- Labels (in test-repo) ---
	for _, l := range []struct{ name, color string }{
		{"bug", "#ee0701"},
		{"enhancement", "#a2eeef"},
		{"documentation", "#0075ca"},
	} {
		_, _, err := testClient.CreateLabel(owner, "test-repo", forgejo.CreateLabelOption{
			Name:  l.name,
			Color: l.color,
		})
		if err != nil {
			return fmt.Errorf("creating label %s: %w", l.name, err)
		}
	}

	// --- Milestone ---
	_, _, err = testClient.CreateMilestone(owner, "test-repo", forgejo.CreateMilestoneOption{
		Title: "v1.0",
	})
	if err != nil {
		return fmt.Errorf("creating milestone: %w", err)
	}

	// Resolve label IDs
	labels, _, err := testClient.ListRepoLabels(owner, "test-repo", forgejo.ListLabelsOptions{})
	if err != nil {
		return fmt.Errorf("listing labels: %w", err)
	}
	labelMap := make(map[string]int64)
	for _, l := range labels {
		labelMap[l.Name] = l.ID
	}

	// --- Issues (in test-repo) ---
	// #1 open, bug
	_, _, err = testClient.CreateIssue(owner, "test-repo", forgejo.CreateIssueOption{
		Title:  "First issue",
		Body:   "This is the first test issue",
		Labels: []int64{labelMap["bug"]},
	})
	if err != nil {
		return fmt.Errorf("creating issue #1: %w", err)
	}

	// #2 open, enhancement
	_, _, err = testClient.CreateIssue(owner, "test-repo", forgejo.CreateIssueOption{
		Title:  "Second issue",
		Body:   "Enhancement request for testing",
		Labels: []int64{labelMap["enhancement"]},
	})
	if err != nil {
		return fmt.Errorf("creating issue #2: %w", err)
	}

	// #3 closed
	_, _, err = testClient.CreateIssue(owner, "test-repo", forgejo.CreateIssueOption{
		Title: "Closed issue",
		Body:  "This issue will be closed",
	})
	if err != nil {
		return fmt.Errorf("creating issue #3: %w", err)
	}
	closed := forgejo.StateClosed
	_, _, err = testClient.EditIssue(owner, "test-repo", 3, forgejo.EditIssueOption{
		State: &closed,
	})
	if err != nil {
		return fmt.Errorf("closing issue #3: %w", err)
	}

	// --- Branches & PRs ---
	for i, branch := range []string{"feature-1", "feature-2"} {
		_, _, err := testClient.CreateBranch(owner, "test-repo", forgejo.CreateBranchOption{
			BranchName:    branch,
			OldBranchName: "main",
		})
		if err != nil {
			return fmt.Errorf("creating branch %s: %w", branch, err)
		}

		content := fmt.Sprintf("Content for %s\n", branch)
		_, _, err = testClient.CreateFile(owner, "test-repo", branch+".txt", forgejo.CreateFileOptions{
			FileOptions: forgejo.FileOptions{
				Message:    fmt.Sprintf("Add %s file", branch),
				BranchName: branch,
			},
			Content: base64.StdEncoding.EncodeToString([]byte(content)),
		})
		if err != nil {
			return fmt.Errorf("creating file on %s: %w", branch, err)
		}

		// PR indices will be #4 and #5 (after 3 issues)
		_, _, err = testClient.CreatePullRequest(owner, "test-repo", forgejo.CreatePullRequestOption{
			Head:  branch,
			Base:  "main",
			Title: fmt.Sprintf("Add %s", branch),
			Body:  fmt.Sprintf("This PR adds %s", branch),
		})
		if err != nil {
			return fmt.Errorf("creating PR #%d: %w", i+4, err)
		}
	}

	return nil
}

// runFJ executes the fj binary with the given args and returns stdout, stderr, and error.
func runFJ(args ...string) (string, string, error) {
	return runFJIn(os.TempDir(), args...) // temp dir avoids git remote detection
}

// runFJIn is runFJ with an explicit working directory, for commands whose
// behaviour depends on the surrounding git repository.
func runFJIn(dir string, args ...string) (string, string, error) {
	cmd := exec.Command(fjBinary, args...)
	cmd.Env = append(
		os.Environ(),
		"FJ_CONFIG_DIR="+configDir,
		"FJ_INSECURE=1",
		"NO_COLOR=1",
	)
	cmd.Dir = dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// mustRunFJ runs fj and fails the test if it errors.
func mustRunFJ(t *testing.T, args ...string) string {
	t.Helper()
	stdout, stderr, err := runFJ(args...)
	if err != nil {
		t.Fatalf("fj %v failed: %v\nstdout: %s\nstderr: %s", args, err, stdout, stderr)
	}
	return stdout
}
