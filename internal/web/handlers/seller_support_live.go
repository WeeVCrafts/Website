package handlers

import (
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/wecratfs/commerce/internal/adapters/postgres"
	applicationcommerce "github.com/wecratfs/commerce/internal/application/commerce"
	domaincommerce "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
	sellerpages "github.com/wecratfs/commerce/web/selleradmin"
)

// serveSellerSupport owns the seller support screen and its persistence routes.
func (h *LivePortalHandler) serveSellerSupport(w http.ResponseWriter, r *http.Request) bool {
	path := r.URL.Path
	if path != "/seller-admin/support" && path != "/seller-admin/support/create" && path != "/seller-admin/support/reply" && path != "/seller/support/create" && path != "/seller/support/reply" {
		return false
	}
	session, ok := h.session(r)
	if !ok {
		redirectToLogin(w, r)
		return true
	}
	if h.pool == nil {
		http.Error(w, "support unavailable", http.StatusServiceUnavailable)
		return true
	}
	var allowed bool
	err := h.pool.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM sellers s JOIN users u ON u.id=s.owner_user_id JOIN user_roles ur ON ur.user_id=u.id AND ur.role_slug='seller' WHERE s.owner_user_id=$1 AND u.status='active' AND s.status NOT IN ('suspended','closed'))`, session.UserID).Scan(&allowed)
	if err != nil {
		http.Error(w, "support unavailable", http.StatusServiceUnavailable)
		return true
	}
	if !allowed {
		http.Error(w, "seller owner access required", http.StatusForbidden)
		return true
	}
	if path == "/seller-admin/support" {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return true
		}
		h.renderSellerSupportLive(w, r, session.UserID, session.CSRFToken)
		return true
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return true
	}
	r.Body = http.MaxBytesReader(w, r.Body, 32<<20)
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			http.Error(w, "support upload too large or invalid", http.StatusRequestEntityTooLarge)
			return true
		}
		if r.MultipartForm != nil {
			defer r.MultipartForm.RemoveAll()
		}
	} else if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid support request", http.StatusBadRequest)
		return true
	}
	token := r.FormValue("csrf_token")
	if token == "" || session.CSRFToken == "" || subtle.ConstantTimeCompare([]byte(token), []byte(session.CSRFToken)) != 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return true
	}
	var files []*multipart.FileHeader
	if r.MultipartForm != nil {
		files = r.MultipartForm.File["attachments"]
	}
	if len(files) > 0 && (h.supportAttachments == nil || h.supportAttachments.Validate(files) != nil) {
		http.Error(w, "use up to three JPG, PNG or WebP attachments, each under 10 MB", http.StatusUnprocessableEntity)
		return true
	}
	service := applicationcommerce.NewSupportService(postgres.NewCommerceRepository(h.pool), postgres.NewRoleAuthorizer(h.pool))
	var ticketID, messageID int64
	if strings.HasSuffix(path, "/create") {
		ticket, createErr := service.CreateSellerRequest(r.Context(), session.UserID, r.FormValue("order_number"), r.FormValue("subject"), r.FormValue("message"))
		err = createErr
		ticketID = ticket.ID
	} else {
		ticketID, _ = strconv.ParseInt(r.FormValue("ticket_id"), 10, 64)
		messageID, err = service.ReplySellerWithMessageID(r.Context(), session.UserID, ticketID, r.FormValue("message"))
	}
	if err != nil {
		sellerSupportError(w, err)
		return true
	}
	if len(files) > 0 {
		if err := h.supportAttachments.Store(r.Context(), session.UserID, ticketID, messageID, files); err != nil {
			http.Error(w, "message saved, but attachments could not be stored; submit attachments again", http.StatusServiceUnavailable)
			return true
		}
	}
	http.Redirect(w, r, "/seller-admin/support?notice=Support+message+saved#ticket-"+strconv.FormatInt(ticketID, 10), http.StatusSeeOther)
	return true
}

func sellerSupportError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ports.ErrForbidden):
		http.Error(w, "forbidden", http.StatusForbidden)
	case errors.Is(err, ports.ErrSupportNotFound), errors.Is(err, ports.ErrOrderNotFound):
		http.Error(w, "support ticket or seller order not found", http.StatusNotFound)
	case errors.Is(err, ports.ErrSupportState):
		http.Error(w, "closed tickets cannot receive replies", http.StatusConflict)
	case errors.Is(err, applicationcommerce.ErrInvalidSupportTicket):
		http.Error(w, "use a subject of 4–160 characters and a message of 10–4000 characters; replies require 1–4000 characters", http.StatusUnprocessableEntity)
	default:
		http.Error(w, "support unavailable", http.StatusServiceUnavailable)
	}
}

func (h *LivePortalHandler) renderSellerSupportLive(w http.ResponseWriter, r *http.Request, userID int64, csrf string) {
	page := viewmodels.SellerSupportPage{Portal: h.sellerPage(r, userID, true, nil), Status: r.URL.Query().Get("status")}
	if page.Status != "" && page.Status != "open" && page.Status != "in_progress" && page.Status != "waiting_customer" && page.Status != "resolved" && page.Status != "closed" {
		http.Error(w, "invalid support status", http.StatusBadRequest)
		return
	}
	page.Portal.DisplayName = h.userDisplayName(r, userID)
	page.Portal.UserEmail = h.userEmail(r, userID)
	page.Portal.Initials = initials(page.Portal.DisplayName)
	page.Portal.CSRFToken = csrf
	page.Portal.RoleLabel = "Seller"
	page.RequestKey = rand.Text()
	ticketID, _ := strconv.ParseInt(r.URL.Query().Get("ticket_id"), 10, 64)
	before, _ := strconv.ParseInt(r.URL.Query().Get("coordination_before"), 10, 64)
	page.QueueStatus = r.URL.Query().Get("coordination_status")
	confirmations, confirmationErr := h.confirmationService().Queue(r.Context(), userID, ticketID, true, page.QueueStatus, before)
	if errors.Is(confirmationErr, applicationcommerce.ErrInvalidSupportTicket) {
		http.Error(w, "Invalid request status or cursor", http.StatusBadRequest)
		return
	}
	if confirmationErr != nil {
		http.Error(w, "seller confirmations could not be loaded", http.StatusServiceUnavailable)
		return
	}
	page.Confirmations = confirmations
	if len(page.Confirmations) > 50 {
		page.Confirmations = page.Confirmations[:50]
		values := r.URL.Query()
		values.Set("coordination_before", strconv.FormatInt(page.Confirmations[49].ID, 10))
		page.QueueNext = "/seller-admin/support?" + values.Encode()
	}
	page.Portal.Title = "Seller support"
	page.Portal.Subtitle = "Contact marketplace support and follow your saved conversations."
	page.Portal.CanProducts = true
	page.Portal.CanProductWrite = page.Portal.SellerStatus == "active"
	page.Portal.CanInventory = true
	page.Portal.CanInventoryWrite = true
	page.Portal.CanOrders = true
	page.Portal.CanFulfill = true
	page.Portal.CanReturns = true
	page.Portal.CanReports = true
	page.Portal.CanManageStore = true
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	filter := ` WHERE ($5::bigint=0 OR t.id=$5) AND t.user_id=$1 AND ($2='' OR t.status=$2) AND ($3='' OR POSITION(lower($3) IN lower(t.ticket_number||' '||t.subject||' '||t.message||' '||COALESCE(o.order_number,'')))>0 OR EXISTS(SELECT 1 FROM support_ticket_messages sm WHERE sm.ticket_id=t.id AND POSITION(lower($3) IN lower(sm.body))>0))`
	if err := h.pool.QueryRow(r.Context(), `SELECT COUNT(*) FROM support_tickets t LEFT JOIN orders o ON o.id=t.order_id`+strings.ReplaceAll(filter, "$5", "$4"), userID, page.Status, query, ticketID).Scan(&page.Total); err != nil {
		http.Error(w, "support tickets could not be loaded", http.StatusServiceUnavailable)
		return
	}
	number, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if number < 1 {
		number = 1
	}
	last := (page.Total + 19) / 20
	if last < 1 {
		last = 1
	}
	if number > last {
		number = last
	}
	rows, err := h.pool.Query(r.Context(), `SELECT t.id,t.ticket_number,t.subject,t.message,t.status,t.priority,t.created_at,COALESCE(o.order_number,'') FROM support_tickets t LEFT JOIN orders o ON o.id=t.order_id`+filter+` ORDER BY t.updated_at DESC,t.id DESC LIMIT 20 OFFSET $4`, userID, page.Status, query, (number-1)*20, ticketID)
	if err != nil {
		http.Error(w, "support tickets could not be loaded", http.StatusServiceUnavailable)
		return
	}
	for rows.Next() {
		var item viewmodels.CustomerSupportTicket
		var created time.Time
		if err := rows.Scan(&item.ID, &item.Number, &item.Subject, &item.InitialMessage, &item.Status, &item.Priority, &created, &item.OrderNumber); err != nil {
			rows.Close()
			http.Error(w, "support tickets could not be loaded", http.StatusServiceUnavailable)
			return
		}
		item.Created = created.Format("2 Jan 2006, 15:04")
		item.Messages = []viewmodels.CustomerSupportMessage{{Author: page.Portal.DisplayName, Role: "You", Body: item.InitialMessage, Created: item.Created}}
		page.Tickets = append(page.Tickets, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		http.Error(w, "support tickets could not be loaded", http.StatusServiceUnavailable)
		return
	}
	for i := range page.Tickets {
		item := &page.Tickets[i]
		var ticketAttachments []domaincommerce.SupportAttachment
		var attachmentsErr error
		if h.supportAttachments != nil {
			attachments, err := h.supportAttachments.List(r.Context(), item.ID)
			attachmentsErr = err
			ticketAttachments = attachments
			item.Attachments = supportAttachmentViews(attachments)
			item.Messages[0].Attachments = supportAttachmentViews(filterSupportMessageAttachments(attachments, 0))
		}
		if attachmentsErr != nil {
			http.Error(w, "support attachments could not be loaded", http.StatusServiceUnavailable)
			return
		}
		messages, err := h.pool.Query(r.Context(), `SELECT m.id,u.display_name,m.author_role,m.body,m.created_at FROM support_ticket_messages m JOIN users u ON u.id=m.author_id WHERE m.ticket_id=$1 ORDER BY m.created_at,m.id`, item.ID)
		if err != nil {
			http.Error(w, "support conversation could not be loaded", http.StatusServiceUnavailable)
			return
		}
		for messages.Next() {
			var messageID int64
			var created time.Time
			var message viewmodels.CustomerSupportMessage
			var role string
			if err := messages.Scan(&messageID, &message.Author, &role, &message.Body, &created); err != nil {
				messages.Close()
				http.Error(w, "support conversation could not be loaded", http.StatusServiceUnavailable)
				return
			}
			message.Created = created.Format("2 Jan 2006, 15:04")
			message.Role = "You"
			if role == "admin" {
				message.Role = "Support agent"
			}
			message.Attachments = supportAttachmentViews(filterSupportMessageAttachments(ticketAttachments, messageID))
			item.Messages = append(item.Messages, message)
		}
		err = messages.Err()
		messages.Close()
		if err != nil {
			http.Error(w, "support conversation could not be loaded", http.StatusServiceUnavailable)
			return
		}
	}
	link := func(n int) string {
		values := url.Values{"q": {query}, "status": {page.Status}, "page": {strconv.Itoa(n)}}
		return "/seller-admin/support?" + values.Encode()
	}
	if number > 1 {
		page.PreviousURL = link(number - 1)
	}
	if number < last {
		page.NextURL = link(number + 1)
	}
	h.render(w, r, sellerpages.SupportLive(page))
}
