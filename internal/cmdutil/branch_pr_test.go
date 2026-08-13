package cmdutil

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// fakeForge serves the endpoints branchPR touches. Its paging clamps the
// requested page size the way Forgejo does, which is what the termination
// condition in findBranchPR has to survive.
type fakeForge struct {
	pulls   map[string][]*forgejo.PullRequest // full name -> open pull requests
	parents map[string]string                 // full name -> parent full name

	pageSizeCap int  // server-side page size limit; 0 honours the request
	omitTotal   bool // drop the X-Total-Count header
	repoFails   bool // GetRepo answers 500

	pullPages int // /pulls responses served
}

func (s *fakeForge) client(t *testing.T) *forgejo.Client {
	t.Helper()
	server := httptest.NewServer(s)
	t.Cleanup(server.Close)
	client, err := forgejo.NewClient(server.URL, forgejo.SetToken("token"))
	if err != nil {
		t.Fatalf("forgejo client: %v", err)
	}
	return client
}

func (s *fakeForge) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/")
	switch {
	case path == "version":
		writeJSON(w, map[string]string{"version": "1.22.0"})
	case strings.HasPrefix(path, "repos/") && strings.HasSuffix(path, "/pulls"):
		s.servePulls(w, r, strings.TrimSuffix(strings.TrimPrefix(path, "repos/"), "/pulls"))
	case strings.HasPrefix(path, "repos/"):
		s.serveRepo(w, strings.TrimPrefix(path, "repos/"))
	default:
		http.Error(w, "unexpected request "+r.URL.Path, http.StatusNotFound)
	}
}

func (s *fakeForge) servePulls(w http.ResponseWriter, r *http.Request, fullName string) {
	s.pullPages++
	all := s.pulls[fullName]

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if s.pageSizeCap > 0 && limit > s.pageSizeCap {
		limit = s.pageSizeCap
	}

	if !s.omitTotal {
		w.Header().Set("X-Total-Count", strconv.Itoa(len(all)))
	}
	start := min((page-1)*limit, len(all))
	writeJSON(w, all[start:min(start+limit, len(all))])
}

func (s *fakeForge) serveRepo(w http.ResponseWriter, fullName string) {
	if s.repoFails {
		http.Error(w, `{"message":"internal error"}`, http.StatusInternalServerError)
		return
	}
	repo := repoPayload(fullName)
	if parent, ok := s.parents[fullName]; ok {
		repo.Parent = repoPayload(parent)
	}
	writeJSON(w, repo)
}

func repoPayload(fullName string) *forgejo.Repository {
	owner, name, _ := strings.Cut(fullName, "/")
	return &forgejo.Repository{
		Name:     name,
		FullName: fullName,
		Owner:    &forgejo.User{UserName: owner},
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		panic(err)
	}
}

// openPRs builds n pull requests on branch "other", with "feature" at the
// one-based position match so a test can place it on a chosen page.
func openPRs(n, match int, headRepo string) []*forgejo.PullRequest {
	prs := make([]*forgejo.PullRequest, n)
	for i := range prs {
		ref := "other"
		if i+1 == match {
			ref = "feature"
		}
		prs[i] = &forgejo.PullRequest{
			Index: int64(i + 1),
			Head:  &forgejo.PRBranchInfo{Ref: ref, Repository: repoPayload(headRepo)},
		}
	}
	return prs
}

func TestFindBranchPRPagesPastAClampedPageSize(t *testing.T) {
	base := Repo{Host: "forgejo.example.com", Owner: "example-org", Name: "example-repo"}
	forge := &fakeForge{
		pulls:       map[string][]*forgejo.PullRequest{base.FullName(): openPRs(25, 25, base.FullName())},
		pageSizeCap: 10,
	}

	index, found, err := findBranchPR(forge.client(t), base, base, "feature")
	if err != nil {
		t.Fatal(err)
	}
	if !found || index != 25 {
		t.Errorf("findBranchPR = (%d, %v), want (25, true)", index, found)
	}
	if forge.pullPages != 3 {
		t.Errorf("served %d pages, want 3", forge.pullPages)
	}
}

func TestFindBranchPRStopsOnAnEmptyPage(t *testing.T) {
	base := Repo{Host: "forgejo.example.com", Owner: "example-org", Name: "example-repo"}
	forge := &fakeForge{
		pulls:     map[string][]*forgejo.PullRequest{base.FullName(): openPRs(5, 0, base.FullName())},
		omitTotal: true,
	}

	_, found, err := findBranchPR(forge.client(t), base, base, "feature")
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Error("findBranchPR found a match, want none")
	}
	if forge.pullPages != 2 {
		t.Errorf("served %d pages, want 2 (one full, one empty)", forge.pullPages)
	}
}

func TestFindBranchPRStopsAtTheMaxPageBound(t *testing.T) {
	base := Repo{Host: "forgejo.example.com", Owner: "example-org", Name: "example-repo"}
	// One pull request per page and no total, so nothing but the bound can
	// end the search; the match sits past it.
	forge := &fakeForge{
		pulls:       map[string][]*forgejo.PullRequest{base.FullName(): openPRs(branchPRMaxPages+5, branchPRMaxPages+5, base.FullName())},
		pageSizeCap: 1,
		omitTotal:   true,
	}

	_, found, err := findBranchPR(forge.client(t), base, base, "feature")
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Error("findBranchPR searched past branchPRMaxPages")
	}
	if forge.pullPages != branchPRMaxPages {
		t.Errorf("served %d pages, want %d", forge.pullPages, branchPRMaxPages)
	}
}

func TestBranchPRFallsBackToTheParent(t *testing.T) {
	fork := Repo{Host: "forgejo.example.com", Owner: "contributor", Name: "example-repo"}
	parent := Repo{Host: "forgejo.example.com", Owner: "example-org", Name: "example-repo"}
	forge := &fakeForge{
		pulls: map[string][]*forgejo.PullRequest{
			fork.FullName():   {},
			parent.FullName(): openPRs(3, 2, fork.FullName()),
		},
		parents: map[string]string{fork.FullName(): parent.FullName()},
	}

	index, repo, err := branchPR(forge.client(t), fork, "feature")
	if err != nil {
		t.Fatal(err)
	}
	if index != 2 {
		t.Errorf("index = %d, want 2", index)
	}
	if repo != parent {
		t.Errorf("repo = %v, want the parent %v", repo, parent)
	}
}

func TestBranchPRPrefersTheForkHeadOnTheParent(t *testing.T) {
	fork := Repo{Host: "forgejo.example.com", Owner: "contributor", Name: "example-repo"}
	parent := Repo{Host: "forgejo.example.com", Owner: "example-org", Name: "example-repo"}
	forge := &fakeForge{
		pulls: map[string][]*forgejo.PullRequest{
			fork.FullName(): {},
			parent.FullName(): {
				{Index: 4, Head: &forgejo.PRBranchInfo{Ref: "feature", Repository: repoPayload("someone/example-repo")}},
				{Index: 9, Head: &forgejo.PRBranchInfo{Ref: "feature", Repository: repoPayload(fork.FullName())}},
			},
		},
		parents: map[string]string{fork.FullName(): parent.FullName()},
	}

	index, _, err := branchPR(forge.client(t), fork, "feature")
	if err != nil {
		t.Fatal(err)
	}
	if index != 9 {
		t.Errorf("index = %d, want 9 (the pull request from our fork)", index)
	}
}

func TestBranchPRKeepsTheNoPullRequestMessageWhenTheParentLookupFails(t *testing.T) {
	repo := Repo{Host: "forgejo.example.com", Owner: "example-org", Name: "example-repo"}
	forge := &fakeForge{
		pulls:     map[string][]*forgejo.PullRequest{repo.FullName(): {}},
		repoFails: true,
	}

	_, _, err := branchPR(forge.client(t), repo, "feature")
	want := noBranchPRError("feature", repo.FullName()).Error()
	if err == nil || err.Error() != want {
		t.Errorf("branchPR error = %v, want %q", err, want)
	}
}

func TestBranchPRReportsBothRepositoriesWhenTheParentHasNoMatch(t *testing.T) {
	fork := Repo{Host: "forgejo.example.com", Owner: "contributor", Name: "example-repo"}
	parent := Repo{Host: "forgejo.example.com", Owner: "example-org", Name: "example-repo"}
	forge := &fakeForge{
		pulls: map[string][]*forgejo.PullRequest{
			fork.FullName():   {},
			parent.FullName(): openPRs(2, 0, parent.FullName()),
		},
		parents: map[string]string{fork.FullName(): parent.FullName()},
	}

	_, _, err := branchPR(forge.client(t), fork, "feature")
	want := noBranchPRError("feature", fork.FullName()+" or "+parent.FullName()).Error()
	if err == nil || err.Error() != want {
		t.Errorf("branchPR error = %v, want %q", err, want)
	}
}
