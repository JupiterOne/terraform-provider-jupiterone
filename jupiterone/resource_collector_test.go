package jupiterone

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccCollectorResource_basic(t *testing.T) {
	ctx := context.Background()
	recordingClient, _, cleanup := setupTestClients(ctx, t)
	defer cleanup(t)

	resourceName := "jupiterone_collector.test"
	rName := "tf-acc-collector"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(recordingClient),
		Steps: []resource.TestStep{
			{
				Config: testCollectorConfig(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "name", rName),
					resource.TestMatchResourceAttr(resourceName, "account_id", regexp.MustCompile(`.+`)),
				),
			},
			{
				// Update name
				Config: testCollectorConfig(rName + "-upd"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", rName+"-upd"),
				),
			},
		},
	})
}

func testCollectorConfig(name string) string {
	return fmt.Sprintf(`
provider "jupiterone" {}

resource "jupiterone_collector" "test" {
  name = "%s"
}
`, name)
}
