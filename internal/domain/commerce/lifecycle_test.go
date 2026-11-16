package commerce

import "testing"

func TestLifecycleBoundaries(t *testing.T) {
	for _, x := range []struct {
		s, f, to string
		allowed  bool
	}{
		{"seller", "new", "accepted", true}, {"seller", "new", "shipped", false}, {"shipment", "delivered", "created", false}, {"shipment", "ndr", "out_for_delivery", true}, {"shipment", "rto", "delivered", false}, {"return", "received", "inspected", true}, {"return", "requested", "completed", false}, {"settlement", "pending", "paid", false}, {"settlement", "processing", "paid", true},
	} {
		if got := CanTransition(x.s, x.f, x.to); got != x.allowed {
			t.Errorf("%+v got %v", x, got)
		}
	}
}
func TestPartialRefundConservesPaidAmount(t *testing.T) {
	var sum int64
	for i := 0; i < 3; i++ {
		n, e := AllocateRefund(100, 3, i, 1)
		if e != nil {
			t.Fatal(e)
		}
		sum += n
	}
	if sum != 100 {
		t.Fatal(sum)
	}
	if _, e := AllocateRefund(100, 3, 2, 2); e == nil {
		t.Fatal("over-refund accepted")
	}
}
