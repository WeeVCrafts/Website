package admin

import "github.com/wecratfs/commerce/internal/web/viewmodels"

func dashboardPreview(p viewmodels.AdminPage) viewmodels.AdminPage {
	pending := make([]viewmodels.AdminProduct, 0, 6)
	for _, product := range p.Products {
		if product.Status == "pending_review" {
			pending = append(pending, product)
			if len(pending) == 6 {
				break
			}
		}
	}
	p.Products = pending
	p.Sellers = p.Sellers[:min(6, len(p.Sellers))]
	p.Orders = p.Orders[:min(6, len(p.Orders))]
	p.Returns = p.Returns[:min(6, len(p.Returns))]
	p.Transactions = p.Transactions[:min(6, len(p.Transactions))]
	p.Settlements = p.Settlements[:min(6, len(p.Settlements))]
	p.AuditEntries = p.AuditEntries[:min(6, len(p.AuditEntries))]
	return p
}
