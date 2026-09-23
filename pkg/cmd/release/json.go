package release

import (
	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"

	"github.com/riadafridishibly/fj/internal/cmdutil"
)

// releaseFields are gh's release view fields that Forgejo can fill. Left
// out: isImmutable and uploadUrl.
var releaseFields = []string{
	"apiUrl", "assets", "author", "body", "createdAt", "databaseId", "id", "isDraft",
	"isPrerelease", "name", "publishedAt", "tagName", "tarballUrl", "targetCommitish",
	"url", "zipballUrl",
}

// listFields are gh's release list fields, less isImmutable. listFJFields
// are the rest of releaseFields, which the list response carries too.
var (
	listFields   = []string{"createdAt", "isDraft", "isLatest", "isPrerelease", "name", "publishedAt", "tagName"}
	listFJFields = []string{"apiUrl", "assets", "author", "body", "databaseId", "id", "tarballUrl", "targetCommitish", "url", "zipballUrl"}
)

// releaseJSON returns r keyed by gh field name.
func releaseJSON(r *forgejo.Release) map[string]any {
	assets := make([]map[string]any, len(r.Attachments))
	for i, a := range r.Attachments {
		assets[i] = map[string]any{
			"id": a.ID, "name": a.Name, "size": a.Size, "downloadCount": a.DownloadCount,
			"createdAt": cmdutil.JSONTime(&a.Created), "url": a.DownloadURL,
		}
	}
	return map[string]any{
		"apiUrl":          r.URL,
		"assets":          assets,
		"author":          cmdutil.JSONUser(r.Publisher),
		"body":            r.Note,
		"createdAt":       cmdutil.JSONTime(&r.CreatedAt),
		"databaseId":      r.ID,
		"id":              r.ID,
		"isDraft":         r.IsDraft,
		"isPrerelease":    r.IsPrerelease,
		"name":            r.Title,
		"publishedAt":     cmdutil.JSONTime(&r.PublishedAt),
		"tagName":         r.TagName,
		"tarballUrl":      r.TarURL,
		"targetCommitish": r.Target,
		"url":             r.HTMLURL,
		"zipballUrl":      r.ZipURL,
	}
}
