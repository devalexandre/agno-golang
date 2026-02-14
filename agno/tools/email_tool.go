package tools

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"net/smtp"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message"
)

// EmailTool provides comprehensive email integration.
// Supports sending emails via SMTP and reading emails via IMAP.
type EmailTool struct {
	toolkit.Toolkit
	smtpHost     string
	smtpPort     int
	smtpUsername string
	smtpPassword string
	imapHost     string
	imapPort     int
	imapUsername string
	imapPassword string
	fromAddress  string
}

// EmailConfig holds configuration for email operations.
type EmailConfig struct {
	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
	IMAPHost     string
	IMAPPort     int
	IMAPUsername string
	IMAPPassword string
	FromAddress  string
}

// Parameter structs

type SendEmailParams struct {
	To      []string `json:"to" description:"Recipient email addresses." required:"true"`
	Subject string   `json:"subject" description:"Email subject." required:"true"`
	Body    string   `json:"body" description:"Email body (plain text)." required:"true"`
	Cc      []string `json:"cc,omitempty" description:"CC recipients."`
	Bcc     []string `json:"bcc,omitempty" description:"BCC recipients."`
}

type SendHTMLEmailParams struct {
	To      []string `json:"to" description:"Recipient email addresses." required:"true"`
	Subject string   `json:"subject" description:"Email subject." required:"true"`
	HTML    string   `json:"html" description:"Email body (HTML)." required:"true"`
	Cc      []string `json:"cc,omitempty" description:"CC recipients."`
	Bcc     []string `json:"bcc,omitempty" description:"BCC recipients."`
}

type ListEmailsParams struct {
	Mailbox string `json:"mailbox,omitempty" description:"Mailbox name (default: INBOX)."`
	Limit   int    `json:"limit,omitempty" description:"Number of emails to return. Default: 20."`
	Unread  bool   `json:"unread,omitempty" description:"Only return unread emails."`
}

type ReadEmailParams struct {
	Mailbox string `json:"mailbox,omitempty" description:"Mailbox name (default: INBOX)."`
	UID     uint32 `json:"uid" description:"Email UID." required:"true"`
}

type SearchEmailsParams struct {
	Mailbox string `json:"mailbox,omitempty" description:"Mailbox name (default: INBOX)."`
	Query   string `json:"query" description:"Search query text." required:"true"`
	Limit   int    `json:"limit,omitempty" description:"Number of results. Default: 20."`
}

type ListMailboxesParams struct{}

type CreateMailboxParams struct {
	Name string `json:"name" description:"Mailbox name to create." required:"true"`
}

type DeleteMailboxParams struct {
	Name string `json:"name" description:"Mailbox name to delete." required:"true"`
}

type DeleteEmailParams struct {
	Mailbox string `json:"mailbox,omitempty" description:"Mailbox name (default: INBOX)."`
	UID     uint32 `json:"uid" description:"Email UID." required:"true"`
}

type MarkAsReadParams struct {
	Mailbox string `json:"mailbox,omitempty" description:"Mailbox name (default: INBOX)."`
	UID     uint32 `json:"uid" description:"Email UID." required:"true"`
}

type MarkAsUnreadParams struct {
	Mailbox string `json:"mailbox,omitempty" description:"Mailbox name (default: INBOX)."`
	UID     uint32 `json:"uid" description:"Email UID." required:"true"`
}

type MoveEmailParams struct {
	SourceMailbox string `json:"source_mailbox" description:"Source mailbox name." required:"true"`
	TargetMailbox string `json:"target_mailbox" description:"Target mailbox name." required:"true"`
	UID           uint32 `json:"uid" description:"Email UID." required:"true"`
}

type WatchEmailsParams struct {
	Mailbox       string `json:"mailbox,omitempty" description:"Mailbox to watch (default: INBOX)."`
	SubjectFilter string `json:"subject_filter,omitempty" description:"Only return emails whose subject contains this text."`
	SenderFilter  string `json:"sender_filter,omitempty" description:"Only return emails from this sender address."`
	SinceMinutes  int    `json:"since_minutes,omitempty" description:"Only emails received in the last N minutes. Default: 60."`
	Limit         int    `json:"limit,omitempty" description:"Max emails to return. Default: 10."`
}

// NewEmailTool creates a new Email tool instance.
func NewEmailTool(config EmailConfig) (*EmailTool, error) {
	if config.SMTPHost == "" && config.IMAPHost == "" {
		return nil, fmt.Errorf("at least one of SMTPHost or IMAPHost is required")
	}
	if config.SMTPPort == 0 {
		config.SMTPPort = 587
	}
	if config.IMAPPort == 0 {
		config.IMAPPort = 993
	}

	tk := toolkit.NewToolkit()
	tk.Name = "EmailTool"
	tk.Description = "Comprehensive email integration: send emails via SMTP, read and monitor emails via IMAP, manage mailboxes."

	tool := &EmailTool{
		Toolkit:      tk,
		smtpHost:     config.SMTPHost,
		smtpPort:     config.SMTPPort,
		smtpUsername: config.SMTPUsername,
		smtpPassword: config.SMTPPassword,
		imapHost:     config.IMAPHost,
		imapPort:     config.IMAPPort,
		imapUsername: config.IMAPUsername,
		imapPassword: config.IMAPPassword,
		fromAddress:  config.FromAddress,
	}

	tool.registerMethods()

	return tool, nil
}

func (t *EmailTool) registerMethods() {
	t.Register("SendEmail", "Send a plain text email.", t, t.SendEmail, SendEmailParams{})
	t.Register("SendHTMLEmail", "Send an HTML email.", t, t.SendHTMLEmail, SendHTMLEmailParams{})
	t.Register("ListEmails", "List emails from a mailbox.", t, t.ListEmails, ListEmailsParams{})
	t.Register("ReadEmail", "Read a specific email by UID.", t, t.ReadEmail, ReadEmailParams{})
	t.Register("SearchEmails", "Search for emails by text query.", t, t.SearchEmails, SearchEmailsParams{})
	t.Register("ListMailboxes", "List all mailboxes.", t, t.ListMailboxes, ListMailboxesParams{})
	t.Register("CreateMailbox", "Create a new mailbox.", t, t.CreateMailbox, CreateMailboxParams{})
	t.RegisterWithOptions("DeleteMailbox", "Delete a mailbox.", t, t.DeleteMailbox, DeleteMailboxParams{}, toolkit.WithConfirmation())
	t.RegisterWithOptions("DeleteEmail", "Delete an email.", t, t.DeleteEmail, DeleteEmailParams{}, toolkit.WithConfirmation())
	t.Register("MarkAsRead", "Mark an email as read.", t, t.MarkAsRead, MarkAsReadParams{})
	t.Register("MarkAsUnread", "Mark an email as unread.", t, t.MarkAsUnread, MarkAsUnreadParams{})
	t.Register("MoveEmail", "Move an email to another mailbox.", t, t.MoveEmail, MoveEmailParams{})
	t.Register("WatchEmails", "Check for new unread emails, optionally filtered by subject or sender. Use this to monitor incoming emails and trigger actions.", t, t.WatchEmails, WatchEmailsParams{})
}

// --- Send Operations ---

func (t *EmailTool) SendEmail(params SendEmailParams) (interface{}, error) {
	if len(params.To) == 0 {
		return nil, fmt.Errorf("to is required")
	}

	msg := t.buildEmailMessage(params.To, params.Cc, params.Subject, params.Body, false)

	allRecipients := make([]string, 0, len(params.To)+len(params.Cc)+len(params.Bcc))
	allRecipients = append(allRecipients, params.To...)
	allRecipients = append(allRecipients, params.Cc...)
	allRecipients = append(allRecipients, params.Bcc...)

	if err := t.sendSMTP(allRecipients, msg); err != nil {
		return nil, fmt.Errorf("failed to send email: %w", err)
	}

	result := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Email sent to %d recipients", len(allRecipients)),
		"to":      params.To,
	}
	output, _ := json.Marshal(result)
	return string(output), nil
}

func (t *EmailTool) SendHTMLEmail(params SendHTMLEmailParams) (interface{}, error) {
	if len(params.To) == 0 {
		return nil, fmt.Errorf("to is required")
	}

	msg := t.buildEmailMessage(params.To, params.Cc, params.Subject, params.HTML, true)

	allRecipients := make([]string, 0, len(params.To)+len(params.Cc)+len(params.Bcc))
	allRecipients = append(allRecipients, params.To...)
	allRecipients = append(allRecipients, params.Cc...)
	allRecipients = append(allRecipients, params.Bcc...)

	if err := t.sendSMTP(allRecipients, msg); err != nil {
		return nil, fmt.Errorf("failed to send email: %w", err)
	}

	result := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("HTML email sent to %d recipients", len(allRecipients)),
		"to":      params.To,
	}
	output, _ := json.Marshal(result)
	return string(output), nil
}

// --- Read Operations ---

func (t *EmailTool) ListEmails(params ListEmailsParams) (interface{}, error) {
	mailbox := params.Mailbox
	if mailbox == "" {
		mailbox = "INBOX"
	}
	limit := params.Limit
	if limit <= 0 {
		limit = 20
	}

	c, err := t.connectIMAP()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to IMAP: %w", err)
	}
	defer c.Logout()

	mbox, err := c.Select(mailbox, false)
	if err != nil {
		return nil, fmt.Errorf("failed to select mailbox: %w", err)
	}

	if mbox.Messages == 0 {
		return t.emptyEmailResult(), nil
	}

	criteria := imap.NewSearchCriteria()
	if params.Unread {
		criteria.WithoutFlags = []string{imap.SeenFlag}
	}

	uids, err := c.Search(criteria)
	if err != nil {
		return nil, fmt.Errorf("failed to search emails: %w", err)
	}

	if len(uids) == 0 {
		return t.emptyEmailResult(), nil
	}

	if start := len(uids) - limit; start > 0 {
		uids = uids[start:]
	}

	emails, err := t.fetchEmailSummaries(c, uids)
	if err != nil {
		return nil, err
	}

	result := map[string]interface{}{
		"success": true,
		"emails":  emails,
		"count":   len(emails),
	}
	output, _ := json.Marshal(result)
	return string(output), nil
}

func (t *EmailTool) ReadEmail(params ReadEmailParams) (interface{}, error) {
	mailbox := params.Mailbox
	if mailbox == "" {
		mailbox = "INBOX"
	}

	c, err := t.connectIMAP()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to IMAP: %w", err)
	}
	defer c.Logout()

	if _, err = c.Select(mailbox, false); err != nil {
		return nil, fmt.Errorf("failed to select mailbox: %w", err)
	}

	seqset := new(imap.SeqSet)
	seqset.AddNum(params.UID)

	messages := make(chan *imap.Message, 1)
	done := make(chan error, 1)
	go func() {
		done <- c.Fetch(seqset, []imap.FetchItem{imap.FetchEnvelope, imap.FetchFlags, imap.FetchUid, imap.FetchRFC822}, messages)
	}()

	msg := <-messages
	if msg == nil {
		return nil, fmt.Errorf("email not found")
	}

	if err := <-done; err != nil {
		return nil, fmt.Errorf("failed to fetch email: %w", err)
	}

	section := &imap.BodySectionName{}
	r := msg.GetBody(section)
	if r == nil {
		return nil, fmt.Errorf("failed to get email body")
	}

	mr, err := message.Read(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read email: %w", err)
	}

	body, err := t.extractBody(mr)
	if err != nil {
		body = "Failed to extract body"
	}

	result := map[string]interface{}{
		"success": true,
		"email": map[string]interface{}{
			"uid":     msg.Uid,
			"subject": msg.Envelope.Subject,
			"from":    t.formatAddresses(msg.Envelope.From),
			"to":      t.formatAddresses(msg.Envelope.To),
			"cc":      t.formatAddresses(msg.Envelope.Cc),
			"date":    msg.Envelope.Date.Format(time.RFC3339),
			"body":    body,
			"unread":  !t.hasFlag(msg.Flags, imap.SeenFlag),
		},
	}
	output, _ := json.Marshal(result)
	return string(output), nil
}

func (t *EmailTool) SearchEmails(params SearchEmailsParams) (interface{}, error) {
	mailbox := params.Mailbox
	if mailbox == "" {
		mailbox = "INBOX"
	}
	limit := params.Limit
	if limit <= 0 {
		limit = 20
	}

	c, err := t.connectIMAP()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to IMAP: %w", err)
	}
	defer c.Logout()

	if _, err = c.Select(mailbox, false); err != nil {
		return nil, fmt.Errorf("failed to select mailbox: %w", err)
	}

	criteria := imap.NewSearchCriteria()
	criteria.Text = []string{params.Query}

	uids, err := c.Search(criteria)
	if err != nil {
		return nil, fmt.Errorf("failed to search emails: %w", err)
	}

	if len(uids) == 0 {
		return t.emptyEmailResult(), nil
	}

	if len(uids) > limit {
		uids = uids[len(uids)-limit:]
	}

	emails, err := t.fetchEmailSummaries(c, uids)
	if err != nil {
		return nil, err
	}

	result := map[string]interface{}{
		"success": true,
		"emails":  emails,
		"count":   len(emails),
		"query":   params.Query,
	}
	output, _ := json.Marshal(result)
	return string(output), nil
}

// --- Watch / Monitor ---

// WatchEmails checks for new unread emails with optional filters.
// The agent can call this periodically to monitor incoming emails
// and trigger actions based on subject or sender.
func (t *EmailTool) WatchEmails(params WatchEmailsParams) (interface{}, error) {
	mailbox := params.Mailbox
	if mailbox == "" {
		mailbox = "INBOX"
	}
	sinceMinutes := params.SinceMinutes
	if sinceMinutes <= 0 {
		sinceMinutes = 60
	}
	limit := params.Limit
	if limit <= 0 {
		limit = 10
	}

	c, err := t.connectIMAP()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to IMAP: %w", err)
	}
	defer c.Logout()

	if _, err = c.Select(mailbox, false); err != nil {
		return nil, fmt.Errorf("failed to select mailbox: %w", err)
	}

	// Build search criteria: unseen + since date + optional subject/sender
	criteria := imap.NewSearchCriteria()
	criteria.WithoutFlags = []string{imap.SeenFlag}
	criteria.Since = time.Now().Add(-time.Duration(sinceMinutes) * time.Minute)

	if params.SubjectFilter != "" {
		criteria.Header.Set("Subject", params.SubjectFilter)
	}
	if params.SenderFilter != "" {
		criteria.Header.Set("From", params.SenderFilter)
	}

	uids, err := c.Search(criteria)
	if err != nil {
		return nil, fmt.Errorf("failed to search emails: %w", err)
	}

	checkedAt := time.Now().Format(time.RFC3339)
	since := time.Now().Add(-time.Duration(sinceMinutes) * time.Minute).Format(time.RFC3339)

	if len(uids) == 0 {
		result := map[string]interface{}{
			"success":     true,
			"new_emails":  []interface{}{},
			"count":       0,
			"checked_at":  checkedAt,
			"since":       since,
			"has_matches": false,
		}
		output, _ := json.Marshal(result)
		return string(output), nil
	}

	if len(uids) > limit {
		uids = uids[len(uids)-limit:]
	}

	// Fetch with body for preview
	seqset := new(imap.SeqSet)
	seqset.AddNum(uids...)

	messages := make(chan *imap.Message, limit)
	done := make(chan error, 1)
	go func() {
		done <- c.Fetch(seqset, []imap.FetchItem{imap.FetchEnvelope, imap.FetchFlags, imap.FetchUid, imap.FetchRFC822}, messages)
	}()

	var emails []map[string]interface{}
	for msg := range messages {
		emailInfo := map[string]interface{}{
			"uid":     msg.Uid,
			"subject": msg.Envelope.Subject,
			"from":    t.formatAddresses(msg.Envelope.From),
			"to":      t.formatAddresses(msg.Envelope.To),
			"date":    msg.Envelope.Date.Format(time.RFC3339),
		}

		// Extract body preview
		section := &imap.BodySectionName{}
		r := msg.GetBody(section)
		if r != nil {
			if mr, err := message.Read(r); err == nil {
				if body, err := t.extractBody(mr); err == nil {
					preview := body
					if len(preview) > 500 {
						preview = preview[:500] + "..."
					}
					emailInfo["body_preview"] = preview
				}
			}
		}

		emails = append(emails, emailInfo)
	}

	if err := <-done; err != nil {
		return nil, fmt.Errorf("failed to fetch emails: %w", err)
	}

	result := map[string]interface{}{
		"success":     true,
		"new_emails":  emails,
		"count":       len(emails),
		"checked_at":  checkedAt,
		"since":       since,
		"has_matches": len(emails) > 0,
	}
	output, _ := json.Marshal(result)
	return string(output), nil
}

// --- Mailbox Operations ---

func (t *EmailTool) ListMailboxes(params ListMailboxesParams) (interface{}, error) {
	c, err := t.connectIMAP()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to IMAP: %w", err)
	}
	defer c.Logout()

	mailboxes := make(chan *imap.MailboxInfo, 10)
	done := make(chan error, 1)
	go func() {
		done <- c.List("", "*", mailboxes)
	}()

	var mailboxList []map[string]interface{}
	for m := range mailboxes {
		mailboxList = append(mailboxList, map[string]interface{}{
			"name":       m.Name,
			"attributes": m.Attributes,
		})
	}

	if err := <-done; err != nil {
		return nil, fmt.Errorf("failed to list mailboxes: %w", err)
	}

	result := map[string]interface{}{
		"success":   true,
		"mailboxes": mailboxList,
		"count":     len(mailboxList),
	}
	output, _ := json.Marshal(result)
	return string(output), nil
}

func (t *EmailTool) CreateMailbox(params CreateMailboxParams) (interface{}, error) {
	c, err := t.connectIMAP()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to IMAP: %w", err)
	}
	defer c.Logout()

	if err = c.Create(params.Name); err != nil {
		return nil, fmt.Errorf("failed to create mailbox: %w", err)
	}

	result := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Mailbox '%s' created successfully", params.Name),
	}
	output, _ := json.Marshal(result)
	return string(output), nil
}

func (t *EmailTool) DeleteMailbox(params DeleteMailboxParams) (interface{}, error) {
	c, err := t.connectIMAP()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to IMAP: %w", err)
	}
	defer c.Logout()

	if err = c.Delete(params.Name); err != nil {
		return nil, fmt.Errorf("failed to delete mailbox: %w", err)
	}

	result := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Mailbox '%s' deleted successfully", params.Name),
	}
	output, _ := json.Marshal(result)
	return string(output), nil
}

// --- Message Operations ---

func (t *EmailTool) DeleteEmail(params DeleteEmailParams) (interface{}, error) {
	mailbox := params.Mailbox
	if mailbox == "" {
		mailbox = "INBOX"
	}

	c, err := t.connectIMAP()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to IMAP: %w", err)
	}
	defer c.Logout()

	if _, err = c.Select(mailbox, false); err != nil {
		return nil, fmt.Errorf("failed to select mailbox: %w", err)
	}

	seqset := new(imap.SeqSet)
	seqset.AddNum(params.UID)

	item := imap.FormatFlagsOp(imap.AddFlags, true)
	flags := []interface{}{imap.DeletedFlag}
	if err = c.Store(seqset, item, flags, nil); err != nil {
		return nil, fmt.Errorf("failed to mark email as deleted: %w", err)
	}

	if err = c.Expunge(nil); err != nil {
		return nil, fmt.Errorf("failed to expunge: %w", err)
	}

	result := map[string]interface{}{
		"success": true,
		"message": "Email deleted successfully",
	}
	output, _ := json.Marshal(result)
	return string(output), nil
}

func (t *EmailTool) MarkAsRead(params MarkAsReadParams) (interface{}, error) {
	return t.setFlag(params.Mailbox, params.UID, imap.SeenFlag, true)
}

func (t *EmailTool) MarkAsUnread(params MarkAsUnreadParams) (interface{}, error) {
	return t.setFlag(params.Mailbox, params.UID, imap.SeenFlag, false)
}

func (t *EmailTool) MoveEmail(params MoveEmailParams) (interface{}, error) {
	c, err := t.connectIMAP()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to IMAP: %w", err)
	}
	defer c.Logout()

	if _, err = c.Select(params.SourceMailbox, false); err != nil {
		return nil, fmt.Errorf("failed to select source mailbox: %w", err)
	}

	seqset := new(imap.SeqSet)
	seqset.AddNum(params.UID)

	if err = c.Copy(seqset, params.TargetMailbox); err != nil {
		return nil, fmt.Errorf("failed to copy email: %w", err)
	}

	item := imap.FormatFlagsOp(imap.AddFlags, true)
	flags := []interface{}{imap.DeletedFlag}
	if err = c.Store(seqset, item, flags, nil); err != nil {
		return nil, fmt.Errorf("failed to mark original as deleted: %w", err)
	}

	if err = c.Expunge(nil); err != nil {
		return nil, fmt.Errorf("failed to expunge: %w", err)
	}

	result := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Email moved from '%s' to '%s'", params.SourceMailbox, params.TargetMailbox),
	}
	output, _ := json.Marshal(result)
	return string(output), nil
}

// Execute implements the toolkit.Tool interface.
func (t *EmailTool) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return t.Toolkit.Execute(methodName, input)
}

// --- Internal helpers ---

func (t *EmailTool) emptyEmailResult() string {
	result := map[string]interface{}{
		"success": true,
		"emails":  []interface{}{},
		"count":   0,
	}
	output, _ := json.Marshal(result)
	return string(output)
}

func (t *EmailTool) fetchEmailSummaries(c *client.Client, uids []uint32) ([]map[string]interface{}, error) {
	seqset := new(imap.SeqSet)
	seqset.AddNum(uids...)

	messages := make(chan *imap.Message, len(uids))
	done := make(chan error, 1)
	go func() {
		done <- c.Fetch(seqset, []imap.FetchItem{imap.FetchEnvelope, imap.FetchFlags, imap.FetchUid}, messages)
	}()

	var emails []map[string]interface{}
	for msg := range messages {
		email := map[string]interface{}{
			"uid":     msg.Uid,
			"subject": msg.Envelope.Subject,
			"from":    t.formatAddresses(msg.Envelope.From),
			"to":      t.formatAddresses(msg.Envelope.To),
			"date":    msg.Envelope.Date.Format(time.RFC3339),
			"unread":  !t.hasFlag(msg.Flags, imap.SeenFlag),
		}
		emails = append(emails, email)
	}

	if err := <-done; err != nil {
		return nil, fmt.Errorf("failed to fetch emails: %w", err)
	}

	return emails, nil
}

func (t *EmailTool) buildEmailMessage(to, cc []string, subject, body string, isHTML bool) []byte {
	from := mail.Address{Address: t.fromAddress}

	headers := make(map[string]string)
	headers["From"] = from.String()
	headers["To"] = strings.Join(to, ", ")
	if len(cc) > 0 {
		headers["Cc"] = strings.Join(cc, ", ")
	}
	headers["Subject"] = subject
	headers["MIME-Version"] = "1.0"
	headers["Date"] = time.Now().Format(time.RFC1123Z)

	if isHTML {
		headers["Content-Type"] = "text/html; charset=UTF-8"
	} else {
		headers["Content-Type"] = "text/plain; charset=UTF-8"
	}

	var msg strings.Builder
	for k, v := range headers {
		msg.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	msg.WriteString("\r\n")
	msg.WriteString(body)

	return []byte(msg.String())
}

func (t *EmailTool) sendSMTP(to []string, msg []byte) error {
	auth := smtp.PlainAuth("", t.smtpUsername, t.smtpPassword, t.smtpHost)

	addr := fmt.Sprintf("%s:%d", t.smtpHost, t.smtpPort)

	conn, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %w", err)
	}
	defer conn.Close()

	tlsConfig := &tls.Config{
		ServerName: t.smtpHost,
	}
	if err = conn.StartTLS(tlsConfig); err != nil {
		return fmt.Errorf("failed to start TLS: %w", err)
	}

	if err = conn.Auth(auth); err != nil {
		return fmt.Errorf("failed to authenticate: %w", err)
	}

	if err = conn.Mail(t.fromAddress); err != nil {
		return fmt.Errorf("failed to set sender: %w", err)
	}

	for _, rcpt := range to {
		if err = conn.Rcpt(rcpt); err != nil {
			return fmt.Errorf("failed to set recipient %s: %w", rcpt, err)
		}
	}

	w, err := conn.Data()
	if err != nil {
		return fmt.Errorf("failed to get data writer: %w", err)
	}
	defer w.Close()

	if _, err = w.Write(msg); err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	return nil
}

func (t *EmailTool) connectIMAP() (*client.Client, error) {
	addr := fmt.Sprintf("%s:%d", t.imapHost, t.imapPort)

	tlsConfig := &tls.Config{
		ServerName: t.imapHost,
	}

	c, err := client.DialTLS(addr, tlsConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to IMAP server: %w", err)
	}

	if err := c.Login(t.imapUsername, t.imapPassword); err != nil {
		c.Logout()
		return nil, fmt.Errorf("failed to login: %w", err)
	}

	return c, nil
}

func (t *EmailTool) formatAddresses(addrs []*imap.Address) []string {
	var result []string
	for _, addr := range addrs {
		if addr.MailboxName != "" && addr.HostName != "" {
			email := fmt.Sprintf("%s@%s", addr.MailboxName, addr.HostName)
			if addr.PersonalName != "" {
				result = append(result, fmt.Sprintf("%s <%s>", addr.PersonalName, email))
			} else {
				result = append(result, email)
			}
		}
	}
	return result
}

func (t *EmailTool) hasFlag(flags []string, flag string) bool {
	for _, f := range flags {
		if f == flag {
			return true
		}
	}
	return false
}

func (t *EmailTool) extractBody(mr *message.Entity) (string, error) {
	if mr.Header.Get("Content-Type") == "" {
		body, err := io.ReadAll(mr.Body)
		if err != nil {
			return "", err
		}
		return string(body), nil
	}

	mediaType, params, err := mr.Header.ContentType()
	if err != nil {
		return "", err
	}

	if strings.HasPrefix(mediaType, "text/") {
		body, err := io.ReadAll(mr.Body)
		if err != nil {
			return "", err
		}
		return string(body), nil
	}

	if strings.HasPrefix(mediaType, "multipart/") {
		boundary := params["boundary"]
		if boundary == "" {
			return "", fmt.Errorf("no boundary parameter found")
		}

		mpr := multipart.NewReader(mr.Body, boundary)
		for {
			part, err := mpr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				return "", err
			}

			partContentType := part.Header.Get("Content-Type")
			if partContentType == "" {
				continue
			}

			partMediaType, _, err := mime.ParseMediaType(partContentType)
			if err != nil {
				continue
			}

			if strings.HasPrefix(partMediaType, "text/plain") {
				body, err := io.ReadAll(part)
				if err != nil {
					continue
				}
				return string(body), nil
			}
		}
	}

	return "", fmt.Errorf("no text body found")
}

func (t *EmailTool) setFlag(mailbox string, uid uint32, flag string, add bool) (interface{}, error) {
	if mailbox == "" {
		mailbox = "INBOX"
	}

	c, err := t.connectIMAP()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to IMAP: %w", err)
	}
	defer c.Logout()

	if _, err = c.Select(mailbox, false); err != nil {
		return nil, fmt.Errorf("failed to select mailbox: %w", err)
	}

	seqset := new(imap.SeqSet)
	seqset.AddNum(uid)

	var item imap.StoreItem
	if add {
		item = imap.FormatFlagsOp(imap.AddFlags, true)
	} else {
		item = imap.FormatFlagsOp(imap.RemoveFlags, true)
	}

	flags := []interface{}{flag}
	if err = c.Store(seqset, item, flags, nil); err != nil {
		return nil, fmt.Errorf("failed to set flag: %w", err)
	}

	action := "added"
	if !add {
		action = "removed"
	}

	result := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Flag %s %s", flag, action),
	}
	output, _ := json.Marshal(result)
	return string(output), nil
}
