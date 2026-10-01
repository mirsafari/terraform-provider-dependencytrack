# Signing keys are resolved from the issuer's OpenID Connect discovery document.
resource "dependencytrack_workload_identity_provider" "example" {
  name     = "github-actions"
  type     = "OIDC"
  issuer   = "https://token.actions.githubusercontent.com"
  audience = "https://dependency-track.example.com"
}

# Signing keys are provided inline, for issuers DependencyTrack cannot reach.
resource "dependencytrack_workload_identity_provider" "inline" {
  name                     = "kubernetes"
  type                     = "OIDC"
  issuer                   = "https://kubernetes.default.svc.cluster.local"
  audience                 = "https://dependency-track.example.com"
  session_lifetime_seconds = 1800
  jwks = jsonencode({
    keys = [{
      kty = "RSA"
      kid = "2011-04-29"
      alg = "RS256"
      use = "sig"
      e   = "AQAB"
      n   = "0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4QyQ5v-65YGjQR0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbOpbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-csFCur-kEgU8awapJzKnqDKgw"
    }]
  })
}

# Signing keys are fetched from a SPIFFE trust domain's bundle endpoint.
resource "dependencytrack_workload_identity_provider" "spiffe" {
  name     = "spire"
  type     = "SPIFFE"
  issuer   = "example.org"
  audience = "https://dependency-track.example.com"
  jwks_url = "https://spire.example.org/keys"
}
