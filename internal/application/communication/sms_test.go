package communication

import (
	"context"
	"errors"
	"testing"

	"github.com/wecratfs/commerce/internal/ports"
)

type smsFixture struct {
	claimed  bool
	sends    int
	outcome  string
	saved    ports.SMSResult
	failSave bool
}

func (f *smsFixture) ClaimSMS(context.Context) (ports.SMSJob, bool, error) {
	if f.claimed {
		return ports.SMSJob{}, false, nil
	}
	f.claimed = true
	return ports.SMSJob{ID: 1}, true, nil
}
func (f *smsFixture) FinishSMS(_ context.Context, _ int64, r ports.SMSResult) error {
	f.saved = r
	if f.failSave {
		return errors.New("storage unavailable")
	}
	return nil
}
func (f *smsFixture) SendTransactionalSMS(context.Context, ports.SMSJob) (ports.SMSResult, error) {
	f.sends++
	if f.outcome == "accepted" {
		return ports.SMSResult{Outcome: f.outcome, ProviderID: "provider-request"}, nil
	}
	return ports.SMSResult{Outcome: f.outcome}, errors.New("provider transport failed")
}

func TestSMSWorkerDoesNotReplayAnUncertainOrUnpersistedSubmission(t *testing.T) {
	for _, failSave := range []bool{false, true} {
		f := &smsFixture{outcome: "uncertain", failSave: failSave}
		p := SMSProcessor{Store: f, Sender: f}
		p.Process(context.Background(), 5)
		p.Process(context.Background(), 5)
		if f.sends != 1 || f.saved.Outcome != "uncertain" {
			t.Fatal("uncertain send was replayed")
		}
	}
}
func TestSMSDisabledDoesNotClaimOrAcknowledgeDelivery(t *testing.T) {
	f := &smsFixture{}
	p := SMSProcessor{Store: f}
	n, err := p.Process(context.Background(), 5)
	if n != 0 || err != nil || f.claimed {
		t.Fatal("disabled sender changed delivery state")
	}
}
