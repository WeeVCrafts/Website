package handlers

import (
	domaincommerce "github.com/wecratfs/commerce/internal/domain/commerce"
	"net/http"
)

func orderStageFilter(r *http.Request) string {
	stage, ok := domaincommerce.ParseFulfillmentStage(r.URL.Query().Get("stage"))
	if !ok {
		return ""
	}
	return string(stage)
}
