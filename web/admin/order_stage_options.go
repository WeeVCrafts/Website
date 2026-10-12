package admin

import domaincommerce "github.com/wecratfs/commerce/internal/domain/commerce"

type orderStageOption struct{ Value, Label string }

func orderStageOptions() []orderStageOption {
	options := make([]orderStageOption, 0, len(domaincommerce.FulfillmentStages))
	for _, stage := range domaincommerce.FulfillmentStages {
		options = append(options, orderStageOption{Value: string(stage), Label: stage.Label()})
	}
	return options
}
