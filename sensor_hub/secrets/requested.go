package secrets

// KeepPlaceholder is what a client may send back for a secret it was never
// shown. Like an omitted secret, it leaves the stored one alone.
const KeepPlaceholder = "****"

// Requested reads the secret a create or update asks for from its write-only
// field. change is false when the stored secret is to be left as it is: the
// field is omitted or holds KeepPlaceholder. An empty value asks for the
// secret to be cleared.
func Requested(field *string) (value string, change bool) {
	if field == nil || *field == KeepPlaceholder {
		return "", false
	}
	return *field, true
}
