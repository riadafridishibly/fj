package repo

import (
	"cmp"
	"fmt"
	"maps"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"

	"github.com/riadafridishibly/fj/internal/cmdutil"
)

// repoFields are gh's repository JSON fields that Forgejo can fill. The rest,
// such as licenseInfo and pushedAt, have no source in Forgejo's API.
var repoFields = []string{
	"archivedAt", "assignableUsers", "createdAt", "defaultBranchRef", "deleteBranchOnMerge",
	"description", "diskUsage", "forkCount", "hasIssuesEnabled", "hasProjectsEnabled",
	"hasWikiEnabled", "homepageUrl", "id", "isArchived", "isEmpty", "isFork", "isMirror",
	"isPrivate", "isTemplate", "issues", "labels", "languages", "latestRelease",
	"mergeCommitAllowed", "milestones", "mirrorUrl", "name", "nameWithOwner", "owner", "parent",
	"primaryLanguage", "pullRequests", "rebaseMergeAllowed", "repositoryTopics", "sshUrl",
	"squashMergeAllowed", "stargazerCount", "updatedAt", "url", "viewerCanAdminister",
	"viewerDefaultMergeMethod", "viewerPermission", "visibility", "watchers",
}

// apiRepository is forgejo.Repository plus the fields the SDK does not
// decode.
type apiRepository struct {
	forgejo.Repository
	ArchivedAt                    time.Time `json:"archived_at"`
	Topics                        []string  `json:"topics"`
	Language                      string    `json:"language"`
	DefaultDeleteBranchAfterMerge bool      `json:"default_delete_branch_after_merge"`
}

// getRepo fetches a repository with the fields the SDK drops.
func getRepo(f *cmdutil.Factory, repo cmdutil.Repo) (*apiRepository, error) {
	client, err := f.APIClient(repo)
	if err != nil {
		return nil, err
	}
	r := new(apiRepository)
	if err := client.GetJSON(repo.APIPath(""), r); err != nil {
		return nil, fmt.Errorf("getting repository: %w", err)
	}
	return r, nil
}

// repoJSON returns r keyed by gh field name. assignableUsers, labels,
// languages, latestRelease and milestones cost a request each, so they are
// fetched only when asked for.
func repoJSON(f *cmdutil.Factory, host string, r *apiRepository, j *cmdutil.JSONFlags) (map[string]any, error) {
	var archivedAt any
	if r.Archived {
		archivedAt = cmdutil.JSONTime(&r.ArchivedAt)
	}
	var mirrorURL string
	if r.Mirror {
		mirrorURL = r.OriginalURL
	}
	var parent, language map[string]any
	if p := r.Parent; p != nil {
		parent = map[string]any{"id": p.ID, "name": p.Name, "owner": ownerJSON(p.Owner)}
	}
	if r.Language != "" {
		language = map[string]any{"name": r.Language}
	}
	topics := make([]map[string]any, len(r.Topics))
	for i, t := range r.Topics {
		topics[i] = map[string]any{"name": t}
	}
	perm := r.Permissions
	if perm == nil {
		perm = &forgejo.Permission{}
	}

	m := map[string]any{
		"archivedAt":               archivedAt,
		"createdAt":                cmdutil.JSONTime(&r.Created),
		"defaultBranchRef":         map[string]any{"name": r.DefaultBranch},
		"deleteBranchOnMerge":      r.DefaultDeleteBranchAfterMerge,
		"description":              r.Description,
		"diskUsage":                r.Size,
		"forkCount":                r.Forks,
		"hasIssuesEnabled":         r.HasIssues,
		"hasProjectsEnabled":       r.HasProjects,
		"hasWikiEnabled":           r.HasWiki,
		"homepageUrl":              r.Website,
		"id":                       r.ID,
		"isArchived":               r.Archived,
		"isEmpty":                  r.Empty,
		"isFork":                   r.Fork,
		"isMirror":                 r.Mirror,
		"isPrivate":                r.Private,
		"isTemplate":               r.Template,
		"issues":                   map[string]any{"totalCount": r.OpenIssues},
		"mergeCommitAllowed":       r.AllowMerge,
		"mirrorUrl":                mirrorURL,
		"name":                     r.Name,
		"nameWithOwner":            r.FullName,
		"owner":                    ownerJSON(r.Owner),
		"parent":                   parent,
		"primaryLanguage":          language,
		"pullRequests":             map[string]any{"totalCount": r.OpenPulls},
		"rebaseMergeAllowed":       r.AllowRebase,
		"repositoryTopics":         topics,
		"sshUrl":                   r.SSHURL,
		"squashMergeAllowed":       r.AllowSquash,
		"stargazerCount":           r.Stars,
		"updatedAt":                cmdutil.JSONTime(&r.Updated),
		"url":                      r.HTMLURL,
		"viewerCanAdminister":      perm.Admin,
		"viewerDefaultMergeMethod": strings.ToUpper(strings.ReplaceAll(string(r.DefaultMergeStyle), "-", "_")),
		"viewerPermission":         viewerPermission(perm),
		"visibility":               visibility(r),
		"watchers":                 map[string]any{"totalCount": r.Watchers},
	}

	if !j.Has("assignableUsers") && !j.Has("labels") && !j.Has("languages") &&
		!j.Has("latestRelease") && !j.Has("milestones") {
		return m, nil
	}
	owner, name, _ := strings.Cut(r.FullName, "/")
	repo := cmdutil.Repo{Host: host, Owner: owner, Name: name}
	client, err := f.ClientForRepo(repo)
	if err != nil {
		return nil, err
	}
	apiClient, err := f.APIClient(repo)
	if err != nil {
		return nil, err
	}
	if j.Has("assignableUsers") {
		users, _, err := client.GetAssignees(owner, name)
		if err != nil {
			return nil, fmt.Errorf("listing assignees: %w", err)
		}
		m["assignableUsers"] = cmdutil.JSONUsers(users)
	}
	if j.Has("labels") {
		labels, err := cmdutil.ListAll[*forgejo.Label](apiClient, repo.APIPath("/labels"))
		if err != nil {
			return nil, fmt.Errorf("listing labels: %w", err)
		}
		m["labels"] = cmdutil.JSONLabels(labels)
	}
	if j.Has("languages") {
		langs, _, err := client.GetRepoLanguages(owner, name)
		if err != nil {
			return nil, fmt.Errorf("listing languages: %w", err)
		}
		m["languages"] = languagesJSON(langs)
	}
	if j.Has("latestRelease") {
		rel, resp, err := client.GetLatestRelease(owner, name)
		switch {
		case err == nil:
			m["latestRelease"] = map[string]any{
				"name": rel.Title, "tagName": rel.TagName, "url": rel.HTMLURL,
				"publishedAt": cmdutil.JSONTime(&rel.PublishedAt),
			}
		case resp != nil && resp.StatusCode == http.StatusNotFound:
			m["latestRelease"] = nil
		default:
			return nil, fmt.Errorf("getting latest release: %w", err)
		}
	}
	if j.Has("milestones") {
		milestones, err := cmdutil.ListAll[*forgejo.Milestone](apiClient, repo.APIPath("/milestones?state=open"))
		if err != nil {
			return nil, fmt.Errorf("listing milestones: %w", err)
		}
		out := make([]map[string]any, len(milestones))
		for i, ms := range milestones {
			out[i] = cmdutil.JSONMilestone(ms)
		}
		m["milestones"] = out
	}
	return m, nil
}

// ownerJSON is gh's repository owner shape, which has no name.
func ownerJSON(u *forgejo.User) map[string]any {
	if u == nil {
		return nil
	}
	return map[string]any{"id": u.ID, "login": u.UserName}
}

func visibility(r *apiRepository) string {
	switch {
	case r.Private:
		return "PRIVATE"
	case r.Internal:
		return "INTERNAL"
	}
	return "PUBLIC"
}

func viewerPermission(p *forgejo.Permission) string {
	switch {
	case p.Admin:
		return "ADMIN"
	case p.Push:
		return "WRITE"
	case p.Pull:
		return "READ"
	}
	return ""
}

// languagesJSON is gh's languages shape, largest first, then by name.
func languagesJSON(langs map[string]int64) []map[string]any {
	names := slices.Sorted(maps.Keys(langs))
	slices.SortStableFunc(names, func(a, b string) int { return cmp.Compare(langs[b], langs[a]) })
	out := make([]map[string]any, len(names))
	for i, n := range names {
		out[i] = map[string]any{"node": map[string]any{"name": n}, "size": langs[n]}
	}
	return out
}

// writeRepo prints the repository after a write, fetched again since the
// SDK's response lacks the fields apiRepository adds.
func writeRepo(f *cmdutil.Factory, host, fullName string, j *cmdutil.JSONFlags) error {
	repo, err := cmdutil.RepoFromFullName(fullName)
	if err != nil {
		return err
	}
	repo.Host = host
	r, err := getRepo(f, repo)
	if err != nil {
		return err
	}
	data, err := repoJSON(f, repo.Host, r, j)
	if err != nil {
		return err
	}
	return j.Write(os.Stdout, data)
}
