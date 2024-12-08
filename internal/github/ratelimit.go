package github

type RateLimit struct {
	Cost      int    `graphql:"cost" json:"cost"`
	Limit     int    `graphql:"limit" json:"limit"`
	Remaining int    `graphql:"remaining" json:"remaining"`
	ResetAt   string `graphql:"resetAt" json:"resetAt"`
}

type PageInfo struct {
	HasNextPage     bool   `graphql:"hasNextPage" json:"hasNextPage"`
	HasPreviousPage bool   `graphql:"hasPreviousPage" json:"hasPreviousPage"`
	StartCursor     string `graphql:"startCursor" json:"startCursor"`
	EndCursor       string `graphql:"endCursor" json:"endCursor"`
}
