package comment

import (
	"fmt"
	"net/http"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

const reviewPageSize = 50

// listAllReviews pages through every review on a PR.
func listAllReviews(client *forgejo.Client, owner, repo string, index int64) ([]*forgejo.PullReview, error) {
	var all []*forgejo.PullReview
	for page := 1; ; page++ {
		reviews, resp, err := client.ListPullReviews(owner, repo, index, forgejo.ListPullReviewsOptions{
			ListOptions: forgejo.ListOptions{Page: page, PageSize: reviewPageSize},
		})
		if err != nil {
			if respCode(resp) == http.StatusNotFound {
				return nil, fmt.Errorf("pr #%d not found in %s/%s", index, owner, repo)
			}
			return nil, fmt.Errorf("listing reviews on pr #%d: %w", index, err)
		}
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
	comments, resp, err := client.ListPullReviewComments(owner, repo, index, reviewID)
	if err != nil {
		if respCode(resp) == http.StatusNotFound {
			return nil, fmt.Errorf("review #%d not found on pr #%d (run `fj pr review list %d` for valid review ids)", reviewID, index, index)
		}
		return nil, fmt.Errorf("listing comments for review #%d: %w", reviewID, err)
	}
	return comments, nil
}

func respCode(r *forgejo.Response) int {
	if r == nil || r.Response == nil {
		return 0
	}
	return r.StatusCode
}
