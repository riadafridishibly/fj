package comment

import (
	"fmt"
	"net/http"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"

	"github.com/riadafridishibly/fj/internal/debug"
)

const reviewPageSize = 50

// listAllReviews pages through every review on a PR.
func listAllReviews(client *forgejo.Client, owner, repo string, index int64) ([]*forgejo.PullReview, error) {
	var all []*forgejo.PullReview
	for page := 1; ; page++ {
		debug.Logf(3, "ListPullReviews %s/%s pr=%d page=%d", owner, repo, index, page)
		done := debug.Track(2, fmt.Sprintf("ListPullReviews pr=%d page=%d", index, page))
		reviews, resp, err := client.ListPullReviews(owner, repo, index, forgejo.ListPullReviewsOptions{
			ListOptions: forgejo.ListOptions{Page: page, PageSize: reviewPageSize},
		})
		done()
		if err != nil {
			debug.Logf(3, "ListPullReviews error status=%s: %v", respStatus(resp), err)
			if respCode(resp) == http.StatusNotFound {
				return nil, fmt.Errorf("pr #%d not found in %s/%s", index, owner, repo)
			}
			return nil, fmt.Errorf("listing reviews on pr #%d: %w", index, err)
		}
		debug.Logf(3, "ListPullReviews pr=%d page=%d returned=%d", index, page, len(reviews))
		if len(reviews) == 0 {
			break
		}
		all = append(all, reviews...)
		if len(reviews) < reviewPageSize {
			break
		}
	}
	return all, nil
}

// listReviewComments fetches inline comments for a single review.
func listReviewComments(client *forgejo.Client, owner, repo string, index, reviewID int64) ([]*forgejo.PullReviewComment, error) {
	debug.Logf(3, "ListPullReviewComments %s/%s pr=%d review=%d", owner, repo, index, reviewID)
	done := debug.Track(2, fmt.Sprintf("ListPullReviewComments pr=%d review=%d", index, reviewID))
	comments, resp, err := client.ListPullReviewComments(owner, repo, index, reviewID)
	done()
	if err != nil {
		debug.Logf(3, "ListPullReviewComments error review=%d status=%s: %v", reviewID, respStatus(resp), err)
		if respCode(resp) == http.StatusNotFound {
			return nil, fmt.Errorf("review #%d not found on pr #%d (run `fj pr review list %d` for valid review ids)", reviewID, index, index)
		}
		return nil, fmt.Errorf("listing comments for review #%d: %w", reviewID, err)
	}
	debug.Logf(3, "ListPullReviewComments review=%d returned=%d", reviewID, len(comments))
	return comments, nil
}

func respCode(r *forgejo.Response) int {
	if r == nil || r.Response == nil {
		return 0
	}
	return r.StatusCode
}

func respStatus(r *forgejo.Response) string {
	if r == nil || r.Response == nil {
		return "none"
	}
	return r.Status
}
