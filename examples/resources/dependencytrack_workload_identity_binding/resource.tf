resource "dependencytrack_service_account" "example" {
  name = "ci-pipeline"
}

resource "dependencytrack_workload_identity_provider" "example" {
  name     = "github-actions"
  type     = "OIDC"
  issuer   = "https://token.actions.githubusercontent.com"
  audience = "https://dependency-track.example.com"
}

# Allow any workflow in the acme-inc/app repository, when running on the main branch.
resource "dependencytrack_workload_identity_binding" "example" {
  service_account = dependencytrack_service_account.example.name
  provider_name   = dependencytrack_workload_identity_provider.example.name
  subject         = "repo:acme-inc/app:*"
  condition       = "claims.ref == \"refs/heads/main\""
}
