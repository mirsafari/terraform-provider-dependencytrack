package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// API 5.2+.
func TestAccServiceAccountResource(t *testing.T) {
	if apiSemver.Major < 5 || (apiSemver.Major == 5 && apiSemver.Minor < 2) {
		t.SkipNow()
	}
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing.
			{
				Config: providerConfig + `
resource "dependencytrack_service_account" "test" {
	name = "Test_Service_Account"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("dependencytrack_service_account.test", "id", "Test_Service_Account"),
					resource.TestCheckResourceAttr("dependencytrack_service_account.test", "name", "Test_Service_Account"),
					resource.TestCheckResourceAttr("dependencytrack_service_account.test", "username", "svc:Test_Service_Account"),
					resource.TestCheckNoResourceAttr("dependencytrack_service_account.test", "email"),
					resource.TestCheckResourceAttr("dependencytrack_service_account.test", "suspended", "false"),
				),
			},
			// ImportState testing.
			{
				ResourceName:      "dependencytrack_service_account.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update and Read testing.
			{
				Config: providerConfig + `
resource "dependencytrack_service_account" "test" {
	name = "Test_Service_Account"
	email = "service-account@example.com"
	suspended = true
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("dependencytrack_service_account.test", "id", "Test_Service_Account"),
					resource.TestCheckResourceAttr("dependencytrack_service_account.test", "username", "svc:Test_Service_Account"),
					resource.TestCheckResourceAttr("dependencytrack_service_account.test", "email", "service-account@example.com"),
					resource.TestCheckResourceAttr("dependencytrack_service_account.test", "suspended", "true"),
				),
			},
			// Update and Read testing, removing the email, and granting a permission and team to the service account.
			{
				Config: providerConfig + `
resource "dependencytrack_service_account" "test" {
	name = "Test_Service_Account"
	suspended = false
}

resource "dependencytrack_user_permission" "test" {
	username = dependencytrack_service_account.test.username
	permission = "BOM_UPLOAD"
}

resource "dependencytrack_team" "test" {
	name = "Test_Service_Account_Team"
}

resource "dependencytrack_user_team" "test" {
	username = dependencytrack_service_account.test.username
	team = dependencytrack_team.test.id
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("dependencytrack_service_account.test", "id", "Test_Service_Account"),
					resource.TestCheckNoResourceAttr("dependencytrack_service_account.test", "email"),
					resource.TestCheckResourceAttr("dependencytrack_service_account.test", "suspended", "false"),
					resource.TestCheckResourceAttr("dependencytrack_user_permission.test", "username", "svc:Test_Service_Account"),
					resource.TestCheckResourceAttr("dependencytrack_user_permission.test", "permission", "BOM_UPLOAD"),
					resource.TestCheckResourceAttr("dependencytrack_user_team.test", "username", "svc:Test_Service_Account"),
					resource.TestCheckResourceAttrPair("dependencytrack_user_team.test", "team", "dependencytrack_team.test", "id"),
				),
			},
		},
	})
}

// API 5.2+.
func TestAccServiceAccountResourceSuspendedOnCreate(t *testing.T) {
	if apiSemver.Major < 5 || (apiSemver.Major == 5 && apiSemver.Minor < 2) {
		t.SkipNow()
	}
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing.
			{
				Config: providerConfig + `
resource "dependencytrack_service_account" "test" {
	name = "Test_Service_Account_Suspended"
	email = "suspended@example.com"
	suspended = true
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("dependencytrack_service_account.test", "username", "svc:Test_Service_Account_Suspended"),
					resource.TestCheckResourceAttr("dependencytrack_service_account.test", "email", "suspended@example.com"),
					resource.TestCheckResourceAttr("dependencytrack_service_account.test", "suspended", "true"),
				),
			},
			// ImportState testing.
			{
				ResourceName:      "dependencytrack_service_account.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccServiceAccountResourceRegression236(t *testing.T) {
	// Regression test for https://github.com/SolarFactories/terraform-provider-dependencytrack/issues/236
	if apiSemver.Major < 5 || (apiSemver.Major == 5 && apiSemver.Minor < 2) {
		t.SkipNow()
	}
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create.
			{
				Config: providerConfig + `
resource "dependencytrack_service_account" "test" {
	name = "Test_Service_Account_Regression236"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("dependencytrack_service_account.test", "id", "Test_Service_Account_Regression236"),
				),
			},
			// Import the same service account into a second resource.
			{
				Config: providerConfig + `
resource "dependencytrack_service_account" "test" {
	name = "Test_Service_Account_Regression236"
}

import {
	to = dependencytrack_service_account.test2
	id = dependencytrack_service_account.test.id
}

resource "dependencytrack_service_account" "test2" {
	name = "Test_Service_Account_Regression236"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(
						"dependencytrack_service_account.test", "id",
						"dependencytrack_service_account.test2", "id",
					),
				),
			},
			// Remove the first resource, which deletes the service account, so the second is recreated.
			{
				Config: providerConfig + `
resource "dependencytrack_service_account" "test2" {
	name = "Test_Service_Account_Regression236"
}
`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("dependencytrack_service_account.test2", plancheck.ResourceActionCreate),
					},
				},
				ExpectNonEmptyPlan: true,
			},
		},
	})
}
