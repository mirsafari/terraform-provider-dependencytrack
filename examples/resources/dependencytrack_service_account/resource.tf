resource "dependencytrack_service_account" "example" {
  name  = "ci-pipeline"
  email = "ci-pipeline@example.com"
}

# Permissions and team memberships are granted with the Service Account username.
resource "dependencytrack_user_permission" "example" {
  username   = dependencytrack_service_account.example.username
  permission = "BOM_UPLOAD"
}
