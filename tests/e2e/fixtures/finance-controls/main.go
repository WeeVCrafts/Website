// Loopback rendering fixture only. No database, payment, storage or credentials.
package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/wecratfs/commerce/internal/web/viewmodels"
	adminpages "github.com/wecratfs/commerce/web/admin"
)

func main() {
	mux := http.NewServeMux()
	mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir("web/assets"))))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/admin/finance" {
			http.NotFound(w, r)
			return
		}
		actor := int64(11)
		if r.URL.Query().Get("actor") == "approver" {
			actor = 12
		}
		item := viewmodels.AdminSettlement{ID: "51", Seller: "Independent seller with a long registered business name", Reference: "SETTLE-51-FIXTURE", Orders: "3", Gross: "INR 10,000.00", Commission: "INR 1,000.00", Amount: "INR 9,000.00", TransferAmount: "900000", Status: "pending", Period: "1 Nov 2026 – 4 Nov 2026", PreparedBy: 11, Revision: "1", PayeeVersion: "1", HasPayee: true, PayeeName: "Verified seller account holder", BankName: "Seller nominated bank", AccountLast4: "1234", RoutingCode: "ABCD0123456", VerificationNote: "Match privately supplied recipient document to this seller.", EvidenceState: "Recipient prepared"}
		switch r.URL.Query().Get("stage") {
		case "approved":
			item.Status = "approved"
			item.ApprovedBy = 12
		case "proof":
			item.Status = "approved"
			item.ApprovedBy = 12
			item.HasTransfer = true
			item.BankReference = "UTR-FIXTURE-51"
			item.TransferDate = "2026-11-04"
			item.EvidenceState = "Transfer evidence awaiting confirmation"
		case "legacy":
			item.Status = "paid"
			item.LegacyPaid = true
			item.HasPayee = false
			item.EvidenceState = "Historical paid record: evidence requires reconciliation"
		}
		page := viewmodels.AdminPage{ActorID: actor, Active: "finance", NavActive: "finance", Workspace: "settlements", Title: "Manual settlement fixture", PortalRoot: "/admin", SettlementRoot: "/admin/finance", PortalLabel: "Admin", RoleLabel: "Admin", DisplayName: "Isolated layout operator", CanFinance: true, CSRFToken: "fixture-only", Settlements: []viewmodels.AdminSettlement{item}}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := adminpages.PortalFinance(page).Render(r.Context(), w); err != nil {
			log.Print(err)
		}
	})
	log.Fatal(http.ListenAndServe(fmt.Sprint("127.0.0.1:18087"), mux))
}
