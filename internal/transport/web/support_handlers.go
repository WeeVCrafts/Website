package web

import (
	"errors"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	applicationcommerce "github.com/wecratfs/commerce/internal/application/commerce"
	domaincommerce "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/identity"
	"github.com/wecratfs/commerce/internal/ports"
)

func (h *Handler) supportPage(w http.ResponseWriter, r *http.Request) {
	if h.supportService == nil || r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	session, ok := h.sessionFromRequest(r)
	if !ok {
		h.redirect(w, r, "/login")
		return
	}
	seller := r.URL.Path == "/seller/support"
	var tickets []domaincommerce.SupportTicket
	var err error
	agent := false
	if seller {
		tickets, err = h.supportService.ListSeller(r.Context(), session.UserID)
	} else {
		tickets, err = h.supportService.ListAgent(r.Context(), session.UserID)
		agent = err == nil
		if errors.Is(err, ports.ErrForbidden) {
			tickets, err = h.supportService.ListCustomer(r.Context(), session.UserID)
		}
	}
	if errors.Is(err, ports.ErrForbidden) {
		message := "Sign in as a customer or support agent to use this workspace."
		if seller {
			message = "Only an approved seller owner can use seller support."
		}
		h.renderError(w, http.StatusForbidden, "Support access denied", message)
		return
	}
	if err != nil {
		h.renderError(w, http.StatusServiceUnavailable, "Support unavailable", "We could not load the live support requests.")
		return
	}
	if h.supportAttachments != nil {
		for index := range tickets {
			tickets[index].Attachments, err = h.supportAttachments.List(r.Context(), tickets[index].ID)
			if err != nil {
				h.renderError(w, http.StatusServiceUnavailable, "Support unavailable", "We could not load support attachments.")
				return
			}
		}
	}
	h.render(w, http.StatusOK, "support", pageData{
		Brand: identity.Name, Tagline: identity.Tagline, Description: identity.Description,
		Title: "Support · " + identity.Name, Authenticated: true, AccountUserID: session.UserID,
		CSRFToken: session.CSRFToken, SupportTickets: tickets, SupportAgent: agent, SupportSeller: seller,
	})
}

func (h *Handler) supportCreate(w http.ResponseWriter, r *http.Request) {
	if h.supportService == nil || r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	session, ok := h.sessionFromRequest(r)
	if !ok {
		h.redirect(w, r, "/login")
		return
	}
	if !h.validSessionCSRF(r, session) {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	if err := parseSupportMultipart(r); err != nil {
		h.renderError(w, http.StatusRequestEntityTooLarge, "Support upload too large", "Please keep support attachments within the allowed file limits.")
		return
	}
	files := supportAttachmentFiles(r)
	if len(files) > 0 && (h.supportAttachments == nil || h.supportAttachments.Validate(files) != nil) {
		h.renderError(w, http.StatusUnprocessableEntity, "Support upload invalid", "Use up to three JPG, PNG, or WebP images, with each file no larger than 10 MB.")
		return
	}
	seller := r.URL.Path == "/seller/support/create"
	var createErr error
	var ticket domaincommerce.SupportTicket
	if seller {
		ticket, createErr = h.supportService.CreateSeller(r.Context(), session.UserID, r.FormValue("order_number"), r.FormValue("subject"), r.FormValue("message"))
	} else {
		ticket, createErr = h.supportService.Create(r.Context(), session.UserID, r.FormValue("order_number"), r.FormValue("subject"), r.FormValue("message"))
	}
	if createErr != nil {
		switch {
		case errors.Is(createErr, applicationcommerce.ErrInvalidSupportTicket):
			h.renderError(w, http.StatusUnprocessableEntity, "Support request invalid", "Use a subject of 4–160 characters and a message of at least 10 characters.")
		case errors.Is(createErr, ports.ErrOrderNotFound):
			h.renderError(w, http.StatusNotFound, "Order unavailable", "That order does not belong to your account.")
		case errors.Is(createErr, ports.ErrForbidden):
			h.renderError(w, http.StatusForbidden, "Support access denied", "Sign in with a customer account to create a support request.")
		default:
			h.renderError(w, http.StatusServiceUnavailable, "Support unavailable", "We could not create the support request.")
		}
		return
	}
	if len(files) > 0 {
		if h.supportAttachments == nil || h.supportAttachments.Store(r.Context(), session.UserID, ticket.ID, 0, files) != nil {
			h.renderError(w, http.StatusUnprocessableEntity, "Support upload failed", "The request was created, but the attachments could not be stored. Please try again without files or submit them again.")
			return
		}
	}
	if seller {
		h.redirect(w, r, "/seller/support")
	} else {
		h.redirect(w, r, "/account/support?notice=Support+request+created")
	}
}

func (h *Handler) supportStatus(w http.ResponseWriter, r *http.Request) {
	if h.supportService == nil || r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	session, ok := h.sessionFromRequest(r)
	if !ok {
		h.redirect(w, r, "/login")
		return
	}
	if !h.validSessionCSRF(r, session) {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	ticketID, err := strconv.ParseInt(r.FormValue("ticket_id"), 10, 64)
	if err != nil || ticketID <= 0 {
		h.renderError(w, http.StatusUnprocessableEntity, "Support update invalid", "Choose a valid support ticket.")
		return
	}
	if err := h.supportService.Update(r.Context(), session.UserID, ticketID, r.FormValue("status"), r.FormValue("note")); err != nil {
		switch {
		case errors.Is(err, applicationcommerce.ErrInvalidSupportTicket):
			h.renderError(w, http.StatusUnprocessableEntity, "Support update invalid", "Choose a supported status and keep the note concise.")
		case errors.Is(err, ports.ErrForbidden):
			h.renderError(w, http.StatusForbidden, "Support access denied", "Only support agents and super administrators can update tickets.")
		case errors.Is(err, ports.ErrSupportNotFound):
			h.renderError(w, http.StatusNotFound, "Ticket unavailable", "That support ticket no longer exists.")
		case errors.Is(err, ports.ErrSupportState):
			h.renderError(w, http.StatusConflict, "Support state changed", "Reload the queue before applying another status update.")
		default:
			h.renderError(w, http.StatusServiceUnavailable, "Support unavailable", "We could not save that support update.")
		}
		return
	}
	h.redirect(w, r, "/support/cases?ticket_id="+strconv.FormatInt(ticketID, 10))
}

func (h *Handler) supportReply(w http.ResponseWriter, r *http.Request) {
	if h.supportService == nil || r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	session, ok := h.sessionFromRequest(r)
	if !ok {
		h.redirect(w, r, "/login")
		return
	}
	if !h.validSessionCSRF(r, session) {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	if err := parseSupportMultipart(r); err != nil {
		h.renderError(w, http.StatusRequestEntityTooLarge, "Support upload too large", "Please keep support attachments within the allowed file limits.")
		return
	}
	files := supportAttachmentFiles(r)
	if len(files) > 0 && (h.supportAttachments == nil || h.supportAttachments.Validate(files) != nil) {
		h.renderError(w, http.StatusUnprocessableEntity, "Support upload invalid", "Use up to three JPG, PNG, or WebP images, with each file no larger than 10 MB.")
		return
	}
	ticketID, err := strconv.ParseInt(r.FormValue("ticket_id"), 10, 64)
	if err != nil || ticketID <= 0 {
		h.renderError(w, http.StatusUnprocessableEntity, "Support reply invalid", "Choose a valid support ticket.")
		return
	}
	messageID, err := h.supportService.ReplyWithMessageID(r.Context(), session.UserID, ticketID, r.FormValue("message"))
	if err != nil {
		switch {
		case errors.Is(err, applicationcommerce.ErrInvalidSupportTicket):
			h.renderError(w, http.StatusUnprocessableEntity, "Support reply invalid", "Enter a response of 1 to 4,000 characters.")
		case errors.Is(err, ports.ErrForbidden):
			h.renderError(w, http.StatusForbidden, "Support access denied", "Only support agents can send customer responses.")
		case errors.Is(err, ports.ErrSupportNotFound):
			h.renderError(w, http.StatusNotFound, "Ticket unavailable", "That support ticket no longer exists.")
		default:
			h.renderError(w, http.StatusServiceUnavailable, "Support unavailable", "We could not send that response.")
		}
		return
	}
	if len(files) > 0 && (h.supportAttachments == nil || h.supportAttachments.Store(r.Context(), session.UserID, ticketID, messageID, files) != nil) {
		h.renderError(w, http.StatusUnprocessableEntity, "Support upload failed", "Your reply was sent, but the attachments could not be stored. Please try again without files.")
		return
	}
	h.redirect(w, r, "/support/cases?ticket_id="+strconv.FormatInt(ticketID, 10))
}

func (h *Handler) supportCustomerReply(w http.ResponseWriter, r *http.Request) {
	if h.supportService == nil || r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	session, ok := h.sessionFromRequest(r)
	if !ok {
		h.redirect(w, r, "/login?next=/account/support")
		return
	}
	if !h.validSessionCSRF(r, session) {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	if err := parseSupportMultipart(r); err != nil {
		h.renderError(w, http.StatusRequestEntityTooLarge, "Support upload too large", "Please keep support attachments within the allowed file limits.")
		return
	}
	files := supportAttachmentFiles(r)
	if len(files) > 0 && (h.supportAttachments == nil || h.supportAttachments.Validate(files) != nil) {
		h.renderError(w, http.StatusUnprocessableEntity, "Support upload invalid", "Use up to three JPG, PNG, or WebP images, with each file no larger than 10 MB.")
		return
	}
	ticketID, err := strconv.ParseInt(r.FormValue("ticket_id"), 10, 64)
	if err != nil || ticketID <= 0 {
		h.renderError(w, http.StatusUnprocessableEntity, "Support reply invalid", "Choose a valid support ticket.")
		return
	}
	messageID, err := h.supportService.ReplyCustomerWithMessageID(r.Context(), session.UserID, ticketID, r.FormValue("message"))
	if err != nil {
		switch {
		case errors.Is(err, applicationcommerce.ErrInvalidSupportTicket):
			h.renderError(w, http.StatusUnprocessableEntity, "Support reply invalid", "Enter a response of 1 to 4,000 characters.")
		case errors.Is(err, ports.ErrForbidden):
			h.renderError(w, http.StatusForbidden, "Support access denied", "You can only reply to your own support tickets.")
		case errors.Is(err, ports.ErrSupportNotFound):
			h.renderError(w, http.StatusNotFound, "Ticket unavailable", "That support ticket no longer exists.")
		case errors.Is(err, ports.ErrSupportState):
			h.renderError(w, http.StatusConflict, "Ticket closed", "This ticket is closed. Open a new support request if you still need help.")
		default:
			h.renderError(w, http.StatusServiceUnavailable, "Support unavailable", "We could not send that response.")
		}
		return
	}
	if len(files) > 0 && (h.supportAttachments == nil || h.supportAttachments.Store(r.Context(), session.UserID, ticketID, messageID, files) != nil) {
		h.renderError(w, http.StatusUnprocessableEntity, "Support upload failed", "Your reply was sent, but the attachments could not be stored. Please try again without files.")
		return
	}
	h.redirect(w, r, "/account/support?ticket_id="+strconv.FormatInt(ticketID, 10)+"&notice=Reply+sent")
}

func supportAttachmentFiles(r *http.Request) []*multipart.FileHeader {
	if r.MultipartForm == nil {
		return nil
	}
	files := r.MultipartForm.File["attachments[]"]
	if len(files) == 0 {
		files = r.MultipartForm.File["attachments"]
	}
	return files
}

func parseSupportMultipart(r *http.Request) error {
	if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "multipart/form-data") {
		return nil
	}
	return r.ParseMultipartForm(32 << 20)
}
