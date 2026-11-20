package components

import "github.com/wecratfs/commerce/internal/web/viewmodels"

type portalTask struct {
	Title, Detail, Link, URL string
}

func sellerPortalTask(p viewmodels.SellerAdminPage) portalTask {
	switch p.Active {
	case "dashboard":
		if p.SellerStatus != "" && p.SellerStatus != "active" {
			return portalTask{"Check your application", "Track approval and any changes requested before your shop can start selling.", "View application", "/seller-admin/onboarding"}
		}
		if p.CanManageStore && p.SellerStatus == "active" {
			return portalTask{"Get your catalog ready to ship", "Complete shared seller details, then resolve any product options that need attention.", "Open fulfillment setup", "/seller-admin/setup"}
		}
	case "products":
		if p.CanProductWrite {
			return portalTask{"Prepare your products for checkout", "After a listing is approved, use fulfillment setup to check packing, tax, and return readiness.", "Open fulfillment setup", "/seller-admin/setup"}
		}
	case "inventory":
		if p.CanInventoryWrite {
			return portalTask{"Check stock by product option", "Use the inventory filters to find low or unavailable stock. Open an option to review its movement history before adjusting it.", "", ""}
		}
	case "orders":
		return portalTask{"Open an order to see what is due", "The order shows available actions for acceptance, package preparation, delivery, and support.", "", ""}
	case "returns":
		return portalTask{"Check who owns the next step", "Open a request to see its current status and the actions available for that order.", "", ""}
	case "settings":
		if p.CanManageStore {
			return portalTask{"Review what buyers see", "Preview your studio details before saving. Confirm the shared business details for new orders in fulfillment setup.", "Open fulfillment setup", "/seller-admin/setup"}
		}
	case "onboarding":
		return portalTask{"Review your application status", "Open the storefront profile once the application is approved. Fulfillment setup then shows what you need to sell.", "", ""}
	case "support", "messages":
		return portalTask{"Find the conversation for your issue", "Open the existing case or order before posting a reply so the support team has the right context.", "", ""}
	case "earnings", "commission", "payments", "payouts":
		return portalTask{"Trace an amount to its order", "Use the ledger for commission and order activity, then check the matching settlement for payout status.", "", ""}
	case "analytics", "audit":
		return portalTask{"Choose a date range", "Compare like periods and open the underlying records when investigating a change.", "", ""}
	case "reviews":
		return portalTask{"Check feedback by product", "Use your product list to find the listing and open its customer-facing reviews.", "Open products", "/seller-admin/products"}
	case "promotions":
		return portalTask{"Check product readiness", "Verify the listing, stock, and fulfillment setup before coordinating a marketplace promotion.", "", ""}
	case "team":
		return portalTask{"Review access by responsibility", "Check what the person needs to do before requesting a change to their store access.", "", ""}
	case "terms":
		return portalTask{"Review the responsibilities for selling", "Use fulfillment setup when you are ready to prepare your catalog for checkout.", "", ""}
	}
	return portalTask{}
}

func superAdminPortalTask(p viewmodels.AdminPage) portalTask {
	switch adminTaskSection(p) {
	case "dashboard":
		return portalTask{"Start with the work waiting on you", "Review new seller submissions and fulfillment exceptions before opening a record.", "Open fulfillment", "/super-admin/fulfillment"}
	case "orders", "delivery-exceptions":
		return portalTask{"Identify the next delivery action", "Open the order or exception and check its payment and carrier state before assigning a task.", "Open fulfillment operations", "/super-admin/fulfillment?tab=operations"}
	case "sellers", "seller-approvals", "seller-details", "seller-suspensions":
		return portalTask{"Verify the seller’s setup", "Review account approval separately from the seller’s shared invoice and pickup details.", "Open seller setup", "/super-admin/fulfillment"}
	case "products", "brands", "product-details", "category-management", "inventory":
		return portalTask{"Review product readiness", "Check approval and inventory, then use the seller’s fulfillment record for checkout requirements tied to this option.", "Open fulfillment setup", "/super-admin/fulfillment"}
	case "returns", "disputes", "refunds", "payment-reviews":
		return portalTask{"Open the request and review its evidence", "Check eligibility, inspection and payment status before taking the available action. Check provider status before retrying a refund.", "", ""}
	case "support", "customers", "reviews":
		return portalTask{"Open the record for context", "Review the conversation, order, or listing before responding, escalating, or moderating.", "", ""}
	case "marketing", "coupons", "storefront":
		return portalTask{"Review changes before publishing", "Preview the storefront or promotion and verify what buyers will see.", "Open storefront editor", "/super-admin/storefront"}
	case "finance", "payments", "failed-payments", "finance-reconciliation", "commission-rules", "settlements", "payouts":
		return portalTask{"Reconcile before approving a financial change", "Compare the order, provider state, and ledger evidence. Open the record to see its current actions.", "", ""}
	case "users", "roles", "security", "security-events":
		return portalTask{"Check the account and its access", "Confirm the account identity and current permissions before submitting a recorded change.", "Open audit logs", "/super-admin/audit-logs"}
	case "audit-logs", "analytics", "system-health", "search-indexing-health", "worker-queue-health", "settings", "email-delivery":
		return portalTask{"Start with the affected record", "Use filters to narrow the period and resource, then review the recorded event or service status.", "", ""}
	}
	return portalTask{}
}

func adminTaskSection(p viewmodels.AdminPage) string {
	if p.Workspace != "" {
		return p.Workspace
	}
	return p.Active
}

func sellerPortalSteps(p viewmodels.SellerAdminPage) []string {
	switch p.Active {
	case "products":
		if p.CanProductWrite {
			return []string{"Save a product draft with its listing information, photos, and options.", "Submit the draft for marketplace approval.", "After approval, check fulfillment readiness and stock for the relevant options."}
		}
	case "orders":
		if p.CanFulfill {
			return []string{"Open the order and review the current status.", "Accept and pack the items when those actions become available.", "Confirm the actual parcels and follow their booking or pickup status."}
		}
	case "returns":
		return []string{"Review the request and its recorded evidence.", "Follow the displayed pickup, inspection, or support step.", "Check the request status for the company decision."}
	case "support", "messages":
		return []string{"Find the existing case or start one from the related order.", "Describe what needs a decision and attach relevant evidence.", "Use the same case for replies and status updates."}
	}
	return nil
}

func superAdminPortalSteps(p viewmodels.AdminPage) []string {
	switch adminTaskSection(p) {
	case "sellers", "seller-approvals", "seller-details", "seller-suspensions":
		return []string{"Open the seller application and verify the supporting records.", "Record the decision or request specific changes.", "Review invoice and pickup details in Fulfillment before evaluating checkout readiness."}
	case "products", "product-details":
		return []string{"Open the listing and verify its product information.", "Record the catalog decision.", "Check the affected option in seller fulfillment when approval makes it eligible for setup."}
	case "orders", "delivery-exceptions":
		return []string{"Confirm the recorded payment and shipment state.", "Open any exception to review its evidence and assigned owner.", "Use the available action and check the provider result before closing the exception."}
	case "returns", "disputes", "refunds", "payment-reviews":
		return []string{"Open the request and compare it with the original order and provider state.", "Review the evidence and any pending action.", "Record an explicit decision and check its resulting status before retrying."}
	case "marketing", "coupons", "storefront":
		return []string{"Open or save a draft.", "Preview the affected storefront pages and review the proposed differences.", "Publish the reviewed version and confirm the live result."}
	case "finance", "payments", "failed-payments", "finance-reconciliation", "settlements", "payouts":
		return []string{"Open the relevant payment, ledger, or payout record.", "Reconcile the required evidence and approval state.", "Record the authorized action and verify the updated status."}
	case "users", "roles", "security", "security-events":
		return []string{"Open the account and confirm its current access.", "Review affected capabilities and record the reason before saving.", "Check the resulting account state and audit record."}
	}
	return nil
}
