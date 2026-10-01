package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

const (
	// Public RSA key from RFC 7517, Appendix A.1, so that no network access is needed for inline keys.
	testJWKS = `jsonencode({
		keys = [{
			kty = "RSA"
			kid = "2011-04-29"
			alg = "RS256"
			use = "sig"
			e   = "AQAB"
			n   = join("", [
				"0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W",
				"-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4QyQ5v-65YGjQR0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbOpbISD08qN",
				"Lyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-csFCur-kEgU8awapJzKnqDKgw",
			])
		}]
	})`
)

// API 5.2+.
func TestAccWorkloadIdentityProviderResource(t *testing.T) {
	if apiSemver.Major < 5 || (apiSemver.Major == 5 && apiSemver.Minor < 2) {
		t.SkipNow()
	}
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing.
			{
				Config: providerConfig + `
resource "dependencytrack_workload_identity_provider" "test" {
	name = "Test_Provider"
	type = "OIDC"
	issuer = "https://issuer.example.com"
	audience = "https://dependencytrack.example.com"
	jwks = ` + testJWKS + `
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("dependencytrack_workload_identity_provider.test", "id", "Test_Provider"),
					resource.TestCheckResourceAttr("dependencytrack_workload_identity_provider.test", "name", "Test_Provider"),
					resource.TestCheckResourceAttr("dependencytrack_workload_identity_provider.test", "type", "OIDC"),
					resource.TestCheckResourceAttr("dependencytrack_workload_identity_provider.test", "issuer", "https://issuer.example.com"),
					resource.TestCheckResourceAttr("dependencytrack_workload_identity_provider.test", "audience", "https://dependencytrack.example.com"),
					resource.TestCheckNoResourceAttr("dependencytrack_workload_identity_provider.test", "jwks_url"),
					resource.TestCheckResourceAttrSet("dependencytrack_workload_identity_provider.test", "jwks"),
					resource.TestCheckResourceAttr("dependencytrack_workload_identity_provider.test", "jwks_key_ids.#", "1"),
					resource.TestCheckResourceAttr("dependencytrack_workload_identity_provider.test", "jwks_key_ids.0", "2011-04-29"),
					resource.TestCheckResourceAttr("dependencytrack_workload_identity_provider.test", "session_lifetime_seconds", "3600"),
					resource.TestCheckResourceAttrSet("dependencytrack_workload_identity_provider.test", "created_at"),
				),
			},
			// ImportState testing.
			{
				ResourceName:            "dependencytrack_workload_identity_provider.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"jwks"},
			},
			// Update and Read testing.
			{
				Config: providerConfig + `
resource "dependencytrack_workload_identity_provider" "test" {
	name = "Test_Provider"
	type = "OIDC"
	issuer = "https://issuer2.example.com"
	audience = "https://dependencytrack2.example.com"
	jwks = ` + testJWKS + `
	session_lifetime_seconds = 7200
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("dependencytrack_workload_identity_provider.test", "id", "Test_Provider"),
					resource.TestCheckResourceAttr("dependencytrack_workload_identity_provider.test", "name", "Test_Provider"),
					resource.TestCheckResourceAttr("dependencytrack_workload_identity_provider.test", "type", "OIDC"),
					resource.TestCheckResourceAttr("dependencytrack_workload_identity_provider.test", "issuer", "https://issuer2.example.com"),
					resource.TestCheckResourceAttr("dependencytrack_workload_identity_provider.test", "audience", "https://dependencytrack2.example.com"),
					resource.TestCheckNoResourceAttr("dependencytrack_workload_identity_provider.test", "jwks_url"),
					resource.TestCheckResourceAttr("dependencytrack_workload_identity_provider.test", "jwks_key_ids.#", "1"),
					resource.TestCheckResourceAttr("dependencytrack_workload_identity_provider.test", "session_lifetime_seconds", "7200"),
					resource.TestCheckResourceAttrSet("dependencytrack_workload_identity_provider.test", "created_at"),
				),
			},
		},
	})
}

// API 5.2+.
func TestAccWorkloadIdentityProviderResourceDiscovery(t *testing.T) {
	if apiSemver.Major < 5 || (apiSemver.Major == 5 && apiSemver.Minor < 2) {
		t.SkipNow()
	}
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing, resolving the keys from the issuer's discovery document.
			{
				Config: providerConfig + `
resource "dependencytrack_workload_identity_provider" "test" {
	name = "Test_Provider_Discovery"
	type = "OIDC"
	issuer = "https://token.actions.githubusercontent.com"
	audience = "https://dependencytrack.example.com"
	session_lifetime_seconds = 600
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("dependencytrack_workload_identity_provider.test", "id", "Test_Provider_Discovery"),
					resource.TestCheckResourceAttr("dependencytrack_workload_identity_provider.test", "type", "OIDC"),
					resource.TestCheckResourceAttr("dependencytrack_workload_identity_provider.test", "issuer", "https://token.actions.githubusercontent.com"),
					resource.TestCheckResourceAttr(
						"dependencytrack_workload_identity_provider.test", "jwks_url", "https://token.actions.githubusercontent.com/.well-known/jwks",
					),
					resource.TestCheckNoResourceAttr("dependencytrack_workload_identity_provider.test", "jwks"),
					resource.TestCheckNoResourceAttr("dependencytrack_workload_identity_provider.test", "jwks_key_ids"),
					resource.TestCheckResourceAttr("dependencytrack_workload_identity_provider.test", "session_lifetime_seconds", "600"),
					resource.TestCheckResourceAttrSet("dependencytrack_workload_identity_provider.test", "created_at"),
				),
			},
			// ImportState testing.
			{
				ResourceName:      "dependencytrack_workload_identity_provider.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update and Read testing, explicitly setting the URL.
			{
				Config: providerConfig + `
resource "dependencytrack_workload_identity_provider" "test" {
	name = "Test_Provider_Discovery"
	type = "OIDC"
	issuer = "https://token.actions.githubusercontent.com"
	audience = "https://dependencytrack2.example.com"
	jwks_url = "https://token.actions.githubusercontent.com/.well-known/jwks"
	session_lifetime_seconds = 600
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("dependencytrack_workload_identity_provider.test", "id", "Test_Provider_Discovery"),
					resource.TestCheckResourceAttr("dependencytrack_workload_identity_provider.test", "audience", "https://dependencytrack2.example.com"),
					resource.TestCheckResourceAttr(
						"dependencytrack_workload_identity_provider.test", "jwks_url", "https://token.actions.githubusercontent.com/.well-known/jwks",
					),
					resource.TestCheckNoResourceAttr("dependencytrack_workload_identity_provider.test", "jwks_key_ids"),
					resource.TestCheckResourceAttr("dependencytrack_workload_identity_provider.test", "session_lifetime_seconds", "600"),
				),
			},
		},
	})
}

// API 5.2+.
func TestAccWorkloadIdentityProviderResourceSPIFFE(t *testing.T) {
	if apiSemver.Major < 5 || (apiSemver.Major == 5 && apiSemver.Minor < 2) {
		t.SkipNow()
	}
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing.
			{
				Config: providerConfig + `
resource "dependencytrack_workload_identity_provider" "test" {
	name = "Test_Provider_SPIFFE"
	type = "SPIFFE"
	issuer = "example.org"
	audience = "https://dependencytrack.example.com"
	jwks = ` + testJWKS + `
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("dependencytrack_workload_identity_provider.test", "id", "Test_Provider_SPIFFE"),
					resource.TestCheckResourceAttr("dependencytrack_workload_identity_provider.test", "type", "SPIFFE"),
					resource.TestCheckResourceAttr("dependencytrack_workload_identity_provider.test", "issuer", "example.org"),
					resource.TestCheckNoResourceAttr("dependencytrack_workload_identity_provider.test", "jwks_url"),
					resource.TestCheckResourceAttr("dependencytrack_workload_identity_provider.test", "jwks_key_ids.#", "1"),
					resource.TestCheckResourceAttr("dependencytrack_workload_identity_provider.test", "session_lifetime_seconds", "3600"),
				),
			},
			// ImportState testing.
			{
				ResourceName:            "dependencytrack_workload_identity_provider.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"jwks"},
			},
			// Replace testing, since type is immutable.
			{
				Config: providerConfig + `
resource "dependencytrack_workload_identity_provider" "test" {
	name = "Test_Provider_SPIFFE"
	type = "OIDC"
	issuer = "https://issuer.example.com"
	audience = "https://dependencytrack.example.com"
	jwks = ` + testJWKS + `
}
`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("dependencytrack_workload_identity_provider.test", plancheck.ResourceActionReplace),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("dependencytrack_workload_identity_provider.test", "type", "OIDC"),
					resource.TestCheckResourceAttr("dependencytrack_workload_identity_provider.test", "issuer", "https://issuer.example.com"),
				),
			},
		},
	})
}

func TestAccWorkloadIdentityProviderResourceRegression236(t *testing.T) {
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
resource "dependencytrack_workload_identity_provider" "test" {
	name = "Test_Provider_Regression236"
	type = "OIDC"
	issuer = "https://issuer.example.com"
	audience = "https://dependencytrack.example.com"
	jwks = ` + testJWKS + `
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("dependencytrack_workload_identity_provider.test", "id", "Test_Provider_Regression236"),
				),
			},
			// Import the same provider into a second resource.
			{
				Config: providerConfig + `
resource "dependencytrack_workload_identity_provider" "test" {
	name = "Test_Provider_Regression236"
	type = "OIDC"
	issuer = "https://issuer.example.com"
	audience = "https://dependencytrack.example.com"
	jwks = ` + testJWKS + `
}

import {
	to = dependencytrack_workload_identity_provider.test2
	id = dependencytrack_workload_identity_provider.test.id
}

resource "dependencytrack_workload_identity_provider" "test2" {
	name = "Test_Provider_Regression236"
	type = "OIDC"
	issuer = "https://issuer.example.com"
	audience = "https://dependencytrack.example.com"
	jwks = ` + testJWKS + `
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(
						"dependencytrack_workload_identity_provider.test", "id",
						"dependencytrack_workload_identity_provider.test2", "id",
					),
				),
			},
			// Remove the first resource, which deletes the provider, so the second is recreated.
			{
				Config: providerConfig + `
resource "dependencytrack_workload_identity_provider" "test2" {
	name = "Test_Provider_Regression236"
	type = "OIDC"
	issuer = "https://issuer.example.com"
	audience = "https://dependencytrack.example.com"
	jwks = ` + testJWKS + `
}
`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("dependencytrack_workload_identity_provider.test2", plancheck.ResourceActionCreate),
					},
				},
				ExpectNonEmptyPlan: true,
			},
		},
	})
}
