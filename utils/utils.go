package utils

// TokenURIFromAccount forms a Google Cloud Storage URI for a YouTube token
// based on the provided account. If the account is empty, it's assumed that
// the legacy token is being used. Otherwise, the account is used to form the
// URI. This means we can have tokens stored for different YouTube accounts.
// The URI is of the form: gs://ausocean/<account>.youtube.token.json
// e.g. gs://ausocean/social@ausocean.org.youtube.token.json
func TokenURIFromAccount(account string) string {
	const (
		bucket          = "gs://ausocean/"
		legacyTokenName = "youtube-api-credentials.json"
		defaultTokenURI = bucket + legacyTokenName
	)

	if account == "" {
		return defaultTokenURI
	}

	const tokenPostfix = ".youtube.token.json"

	return bucket + account + tokenPostfix
}
