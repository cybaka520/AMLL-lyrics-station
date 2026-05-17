package search

import "github.com/meilisearch/meilisearch-go"

func applyIndexSettings(idx meilisearch.IndexManager) (*meilisearch.TaskInfo, error) {
	settings := &meilisearch.Settings{}
	searchable := []string{
		"music_names",
		"artists",
		"albums",
		"ncm_music_ids",
		"qq_music_ids",
		"spotify_ids",
		"apple_music_ids",
		"isrcs",
		"lyric_content",
		"translated_lyric",
		"roman_lyric",
	}
	filterable := []string{"artists", "albums", "ttml_author_login"}
	sortable := []string{"updated_at"}
	settings.SearchableAttributes = searchable
	settings.FilterableAttributes = filterable
	settings.SortableAttributes = sortable
	settings.TypoTolerance = &meilisearch.TypoTolerance{Enabled: true}
	return idx.UpdateSettings(settings)
}
