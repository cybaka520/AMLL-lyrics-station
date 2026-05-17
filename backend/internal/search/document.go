package search

type LyricDocument struct {
	ID               string   `json:"id"`
	RawLyricFile     string   `json:"raw_lyric_file"`
	MusicNames       []string `json:"music_names"`
	Artists          []string `json:"artists"`
	Albums           []string `json:"albums"`
	NcmMusicIds      []string `json:"ncm_music_ids"`
	QqMusicIds       []string `json:"qq_music_ids"`
	SpotifyIds       []string `json:"spotify_ids"`
	AppleMusicIds    []string `json:"apple_music_ids"`
	Isrcs            []string `json:"isrcs"`
	TtmlAuthorGithub string   `json:"ttml_author_github"`
	TtmlAuthorLogin  string   `json:"ttml_author_login"`
	LyricContent     string   `json:"lyric_content"`
	TranslatedLyric  string   `json:"translated_lyric,omitempty"`
	RomanLyric       string   `json:"roman_lyric,omitempty"`
	UpdatedAt        int64    `json:"updated_at"`
}

type SearchRequest struct {
	Query      string   `form:"q" binding:"required"`
	Fields     []string `form:"fields"`
	Filters    string   `form:"filters"`
	Sort       []string `form:"sort"`
	Page       int      `form:"page,default=1"`
	Limit      int      `form:"limit,default=20"`
	ExactMatch bool     `form:"exact_match"`
}

type SearchResponse struct {
	Hits       []LyricDocument `json:"hits"`
	Total      int64           `json:"total"`
	Page       int             `json:"page"`
	Limit      int             `json:"limit"`
	TotalPages int             `json:"total_pages"`
	Query      string          `json:"query"`
	DurationMS float64         `json:"duration_ms"`
}
