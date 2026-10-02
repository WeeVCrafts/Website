package selleradmin

import (
	"github.com/wecratfs/commerce/internal/web/viewmodels"
	"testing"
)

func TestSellerOrdersByStageGroupsFulfillmentStates(t *testing.T) {
	orders := []viewmodels.SellerAdminOrder{{Number: "new", Status: "pending"}, {Number: "packing", Status: "processing"}, {Number: "shipped", Status: "shipped"}, {Number: "complete", Status: "delivered"}}
	for _, test := range []struct{ stage, want string }{{"new", "new"}, {"processing", "packing"}, {"shipped", "shipped"}, {"delivered", "complete"}} {
		got := sellerOrdersByStage(orders, test.stage)
		if len(got) != 1 || got[0].Number != test.want {
			t.Fatalf("stage %q got %#v", test.stage, got)
		}
	}
}
