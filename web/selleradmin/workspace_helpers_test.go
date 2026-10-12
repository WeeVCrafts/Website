package selleradmin

import (
	"github.com/wecratfs/commerce/internal/web/viewmodels"
	"testing"
)

func TestSellerOrdersByStageGroupsFulfillmentStates(t *testing.T) {
	orders := []viewmodels.SellerAdminOrder{{Number: "new", Status: "pending"}, {Number: "processing", Status: "processing"}, {Number: "packing", Status: "packing"}, {Number: "shipped", Status: "shipped"}, {Number: "ready", Status: "out_for_delivery"}, {Number: "complete", Status: "delivered"}}
	for _, test := range []struct{ stage, want string }{{"new", "new"}, {"processing", "processing"}, {"packing", "packing"}, {"shipped", "shipped"}, {"out_for_delivery", "ready"}, {"delivered", "complete"}} {
		got := sellerOrdersByStage(orders, test.stage)
		if len(got) != 1 || got[0].Number != test.want {
			t.Fatalf("stage %q got %#v", test.stage, got)
		}
	}
}
