package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

const (
	testWorkloadIdentityBindingDependencies = `
resource "dependencytrack_service_account" "test" {
	name = "Test_Binding_Service_Account"
}

resource "dependencytrack_workload_identity_provider" "test" {
	name = "Test_Binding_Provider"
	type = "OIDC"
	issuer = "https://issuer.example.com"
	audience = "https://dependencytrack.example.com"
	jwks = ` + testJWKS + `
}
`
)

// API 5.2+.
func TestAccWorkloadIdentityBindingResource(t *testing.T) {
	if apiSemver.Major < 5 || (apiSemver.Major == 5 && apiSemver.Minor < 2) {
		t.SkipNow()
	}
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing.
			{
				Config: providerConfig + testWorkloadIdentityBindingDependencies + `
resource "dependencytrack_workload_identity_binding" "test" {
	service_account = dependencytrack_service_account.test.name
	provider_name = dependencytrack_workload_identity_provider.test.name
	subject = "repo:acme/app:*"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("dependencytrack_workload_identity_binding.test", "id"),
					resource.TestCheckResourceAttrSet("dependencytrack_workload_identity_binding.test", "uuid"),
					resource.TestCheckResourceAttr("dependencytrack_workload_identity_binding.test", "service_account", "Test_Binding_Service_Account"),
					resource.TestCheckResourceAttr("dependencytrack_workload_identity_binding.test", "provider_name", "Test_Binding_Provider"),
					resource.TestCheckResourceAttr("dependencytrack_workload_identity_binding.test", "subject", "repo:acme/app:*"),
					resource.TestCheckNoResourceAttr("dependencytrack_workload_identity_binding.test", "condition"),
					resource.TestCheckResourceAttrSet("dependencytrack_workload_identity_binding.test", "created_at"),
				),
			},
			// ImportState testing.
			{
				ResourceName:      "dependencytrack_workload_identity_binding.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Replace testing, since bindings are immutable.
			{
				Config: providerConfig + testWorkloadIdentityBindingDependencies + `
resource "dependencytrack_workload_identity_binding" "test" {
	service_account = dependencytrack_service_account.test.name
	provider_name = dependencytrack_workload_identity_provider.test.name
	subject = "repo:acme/app:ref:refs/heads/main"
	condition = "claims.ref == \"refs/heads/main\""
}
`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("dependencytrack_workload_identity_binding.test", plancheck.ResourceActionReplace),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("dependencytrack_workload_identity_binding.test", "id"),
					resource.TestCheckResourceAttr("dependencytrack_workload_identity_binding.test", "subject", "repo:acme/app:ref:refs/heads/main"),
					resource.TestCheckResourceAttr("dependencytrack_workload_identity_binding.test", "condition", "claims.ref == \"refs/heads/main\""),
					resource.TestCheckResourceAttrSet("dependencytrack_workload_identity_binding.test", "created_at"),
				),
			},
			// ImportState testing, with a condition.
			{
				ResourceName:      "dependencytrack_workload_identity_binding.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccWorkloadIdentityBindingResourceRegression236(t *testing.T) {
	// Regression test for https://github.com/SolarFactories/terraform-provider-dependencytrack/issues/236
	if apiSemver.Major < 5 || (apiSemver.Major == 5 && apiSemver.Minor < 2) {
		t.SkipNow()
	}
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create.
			{
				Config: providerConfig + testWorkloadIdentityBindingDependencies + `
resource "dependencytrack_workload_identity_binding" "test" {
	service_account = dependencytrack_service_account.test.name
	provider_name = dependencytrack_workload_identity_provider.test.name
	subject = "repo:acme/regression236:*"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("dependencytrack_workload_identity_binding.test", "id"),
				),
			},
			// Import the same binding into a second resource.
			{
				Config: providerConfig + testWorkloadIdentityBindingDependencies + `
resource "dependencytrack_workload_identity_binding" "test" {
	service_account = dependencytrack_service_account.test.name
	provider_name = dependencytrack_workload_identity_provider.test.name
	subject = "repo:acme/regression236:*"
}

import {
	to = dependencytrack_workload_identity_binding.test2
	id = dependencytrack_workload_identity_binding.test.id
}

resource "dependencytrack_workload_identity_binding" "test2" {
	service_account = dependencytrack_service_account.test.name
	provider_name = dependencytrack_workload_identity_provider.test.name
	subject = "repo:acme/regression236:*"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(
						"dependencytrack_workload_identity_binding.test", "id",
						"dependencytrack_workload_identity_binding.test2", "id",
					),
				),
			},
			// Remove the first resource, which deletes the binding, so the second is recreated.
			{
				Config: providerConfig + testWorkloadIdentityBindingDependencies + `
resource "dependencytrack_workload_identity_binding" "test2" {
	service_account = dependencytrack_service_account.test.name
	provider_name = dependencytrack_workload_identity_provider.test.name
	subject = "repo:acme/regression236:*"
}
`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("dependencytrack_workload_identity_binding.test2", plancheck.ResourceActionCreate),
					},
				},
				ExpectNonEmptyPlan: true,
			},
		},
	})
}
