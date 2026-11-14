package shiprocket

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/wecratfs/commerce/internal/ports"
)

func (g *Gateway) Assign(ctx context.Context, shipment ports.Shipment, courierID int64) (ports.Shipment, error) {
	input := struct {
		ShipmentID int64 `json:"shipment_id"`
		CourierID  int64 `json:"courier_id,omitempty"`
	}{shipment.ProviderShipmentID, courierID}
	var response struct {
		Data struct {
			AWB     string `json:"awb_code"`
			Courier string `json:"courier_name"`
		} `json:"data"`
	}
	// Error responses can contain either a string or an object in response.data.
	var envelope struct {
		Assigned *int            `json:"awb_assign_status"`
		Response json.RawMessage `json:"response"`
	}
	if err := g.call(ctx, http.MethodPost, "/courier/assign/awb", input, &envelope); err != nil {
		return shipment, err
	}
	if envelope.Assigned == nil {
		return shipment, ports.ErrShippingPending
	}
	if *envelope.Assigned != 1 {
		return shipment, ports.ErrShippingRejected
	}
	if json.Unmarshal(envelope.Response, &response) != nil || len(response.Data.AWB) < 3 || len(response.Data.AWB) > 120 {
		return shipment, ports.ErrShippingPending
	}
	shipment.AWB, shipment.Courier = response.Data.AWB, response.Data.Courier
	return shipment, nil
}

type shipmentIDs struct {
	IDs []int64 `json:"shipment_id"`
}

func (g *Gateway) Pickup(ctx context.Context, shipment ports.Shipment) (ports.Shipment, error) {
	var response struct {
		Status   *int `json:"pickup_status"`
		Response struct {
			Reference string `json:"pickup_token_number"`
		} `json:"response"`
	}
	if err := g.call(ctx, http.MethodPost, "/courier/generate/pickup", shipmentIDs{[]int64{shipment.ProviderShipmentID}}, &response); err != nil {
		return shipment, err
	}
	if response.Status == nil {
		return shipment, ports.ErrShippingPending
	}
	if *response.Status != 1 {
		return shipment, ports.ErrShippingRejected
	}
	if response.Response.Reference == "" {
		return shipment, ports.ErrShippingPending
	}
	shipment.PickupReference = response.Response.Reference
	return shipment, nil
}

func (g *Gateway) Label(ctx context.Context, shipment ports.Shipment) (ports.Shipment, error) {
	var response struct {
		Created *int   `json:"label_created"`
		URL     string `json:"label_url"`
	}
	if err := g.call(ctx, http.MethodPost, "/courier/generate/label", shipmentIDs{[]int64{shipment.ProviderShipmentID}}, &response); err != nil {
		return shipment, err
	}
	if response.Created == nil {
		return shipment, ports.ErrShippingPending
	}
	if *response.Created != 1 {
		return shipment, ports.ErrShippingRejected
	}
	link, err := documentURL(response.URL, g.base == SandboxURL)
	shipment.LabelURL = link
	return shipment, err
}

func (g *Gateway) Manifest(ctx context.Context, shipment ports.Shipment) (ports.Shipment, error) {
	var response struct {
		Status *int   `json:"status"`
		URL    string `json:"manifest_url"`
	}
	if err := g.call(ctx, http.MethodPost, "/manifests/generate", shipmentIDs{[]int64{shipment.ProviderShipmentID}}, &response); err != nil {
		return shipment, err
	}
	if response.Status == nil {
		return shipment, ports.ErrShippingPending
	}
	if *response.Status != 1 {
		return shipment, ports.ErrShippingRejected
	}
	link, err := documentURL(response.URL, g.base == SandboxURL)
	shipment.ManifestURL = link
	return shipment, err
}

func documentURL(raw string, sandbox bool) (string, error) {
	link, err := url.Parse(raw)
	if err != nil || len(raw) > 2048 || link.Scheme != "https" || link.User != nil {
		return "", ports.ErrShippingPending
	}
	host := link.Hostname()
	legacyBucket := host == "s3-ap-southeast-1.amazonaws.com" && strings.HasPrefix(link.EscapedPath(), "/kr-shipmultichannel/")
	sandboxCDN := sandbox && host == "sr-core-cdn-stage.kartrocket.com"
	if !documentHost(host) && !legacyBucket && !sandboxCDN {
		return "", ports.ErrShippingPending
	}
	return link.String(), nil
}

func documentHost(host string) bool {
	return host == "shiprocket.in" || strings.HasSuffix(host, ".shiprocket.in") || host == "shiprocket.com" || strings.HasSuffix(host, ".shiprocket.com") ||
		strings.HasPrefix(host, "kr-shipmultichannel.s3.") && strings.HasSuffix(host, ".amazonaws.com") || host == "kr-shipmultichannel.s3.amazonaws.com"
}
