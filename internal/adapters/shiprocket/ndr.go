package shiprocket

import (
	"context"
	"encoding/json"
	"github.com/wecratfs/commerce/internal/ports"
	"net/http"
	"net/url"
	"strings"
)

func (g *Gateway) ActNDR(ctx context.Context, s ports.Shipment, action, date, comments string) error {
	if s.AWB == "" || (action != "re-attempt" && action != "return") || len(comments) < 4 || len(comments) > 1000 {
		return ports.ErrShippingInput
	}
	input := struct {
		Action   string `json:"action"`
		Comments string `json:"comments"`
		Date     string `json:"deferred_date,omitempty"`
	}{action, comments, date}
	var result struct {
		Success *bool           `json:"success"`
		Status  json.RawMessage `json:"status"`
	}
	if err := g.call(ctx, http.MethodPost, "/ndr/"+url.PathEscape(s.AWB)+"/action", input, &result); err != nil {
		return err
	}
	if result.Success == nil {
		var accepted *bool
		if json.Unmarshal(result.Status, &accepted) == nil && accepted != nil {
			if *accepted {
				return nil
			}
			return ports.ErrShippingRejected
		}
		var status string
		if json.Unmarshal(result.Status, &status) == nil {
			switch strings.ToLower(strings.Join(strings.Fields(status), " ")) {
			case "data updated sucessfully", "data updated successfully":
				return nil
			}
		}
		return ports.ErrShippingPending
	}
	if !*result.Success {
		return ports.ErrShippingRejected
	}
	return nil
}
