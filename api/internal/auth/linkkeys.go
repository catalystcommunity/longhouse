package auth

// This package will handle linkkeys IDP integration:
// - Verifying signed identity assertions (Ed25519)
// - Fetching domain public keys via DNS/HTTP
// - Creating/updating local member records from assertions
// - Checking trusted domain membership
// - Validating share access for external users
//
// Implementation will follow after the CSIL protocol and
// linkkeys client library integration are in place.
