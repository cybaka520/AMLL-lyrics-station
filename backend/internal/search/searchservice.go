package search

import "context"

type SearchService interface {
	Search(ctx context.Context, req SearchRequest) (SearchResponse, error)
}
