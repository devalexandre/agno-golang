package tools

import (
	"context"
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

// EmailTool provides comprehensive email integration
// Supports sending emails via SMTP and reading emails via IMAP
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

// EmailConfig holds configuration for email operations
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

// NewEmailTool creates a new Email tool instance
func NewEmailTool(config EmailConfig) *EmailTool {
	if config.SMTPHost == "" {
		panic("SMTP host is required")
	}
	if config.SMTPPort == 0 {
		config.SMTPPort = 587 // Default SMTP port
	}
	if config.IMAPPort == 0 {
		config.IMAPPort = 993 // Default IMAP port
	}

	tk := toolkit.NewToolkit()
	tk.Name = "EmailTool"
	tk.Description = "Comprehensive email integration - send emails via SMTP, read emails via IMAP, manage attachments"

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

	return tool
}

// registerMethods registers all available email operations
func (t *EmailTool) registerMethods() {
	// Send operations
	t.Register("send_email", "Send a plain text email", t, t.sendEmail, SendEmailParams{})
	t.Register("send_html_email", "Send an HTML email", t, t.sendHTMLEmail, SendHTMLEmailParams{})

	// Read operations
	t.Register("list_emails", "List emails from a mailbox", t, t.listEmails, ListEmailsParams{})
	t.Register("read_email", "Read a specific email by UID", t, t.readEmail, ReadEmailParams{})
	t.Register("search_emails", "Search for emails", t, t.searchEmails, SearchEmailsParams{})

	// Mailbox operations
	t.Register("list_mailboxes", "List all mailboxes", t, t.listMailboxes, ListMailboxesParams{})
	t.Register("create_mailbox", "Create a new mailbox", t, t.createMailbox, CreateMailboxParams{})
	t.Register("delete_mailbox", "Delete a mailbox", t, t.deleteMailbox, DeleteMailboxParams{})

	// Message operations
	t.Register("delete_email", "Delete an email", t, t.deleteEmail, DeleteEmailParams{})
	t.Register("mark_as_read", "Mark an email as read", t, t.markAsRead, MarkAsReadParams{})
	t.Register("mark_as_unread", "Mark an email as unread", t, t.markAsUnread, MarkAsUnreadParams{})
	t.Register("move_email", "Move an email to another mailbox", t, t.moveEmail, MoveEmailParams{})
}

// Parameter structs
type SendEmailParams struct {
	To      []string `json:"to" description:"Recipient email addresses"`
	Subject string   `json:"subject" description:"Email subject"`
	Body    string   `json:"body" description:"Email body (plain text)"`
	Cc      []string `json:"cc" description:"CC recipients (optional)"`
	Bcc     []string `json:"bcc" description:"BCC recipients (optional)"`
}

type SendHTMLEmailParams struct {
	To      []string `json:"to" description:"Recipient email addresses"`
	Subject string   `json:"subject" description:"Email subject"`
	HTML    string   `json:"html" description:"Email body (HTML)"`
	Cc      []string `json:"cc" description:"CC recipients (optional)"`
	Bcc     []string `json:"bcc" description:"BCC recipients (optional)"`
}

type ListEmailsParams struct {
	Mailbox string `json:"mailbox" description:"Mailbox name (default: INBOX)"`
	Limit   int    `json:"limit" description:"Number of emails to return"`
	Unread  bool   `json:"unread" description:"Only return unread emails"`
}

type ReadEmailParams struct {
	Mailbox string `json:"mailbox" description:"Mailbox name"`
	UID     uint32 `json:"uid" description:"Email UID"`
}

type SearchEmailsParams struct {
	Mailbox string `json:"mailbox" description:"Mailbox name (default: INBOX)"`
	Query   string `json:"query" description:"Search query"`
	Limit   int    `json:"limit" description:"Number of results"`
}

type ListMailboxesParams struct{}

type CreateMailboxParams struct {
	Name string `json:"name" description:"Mailbox name"`
}

type DeleteMailboxParams struct {
	Name string `json:"name" description:"Mailbox name"`
}

type DeleteEmailParams struct {
	Mailbox string `json:"mailbox" description:"Mailbox name"`
	UID     uint32 `json:"uid" description:"Email UID"`
}

type MarkAsReadParams struct {
	Mailbox string `json:"mailbox" description:"Mailbox name"`
	UID     uint32 `json:"uid" description:"Email UID"`
}

type MarkAsUnreadParams struct {
	Mailbox string `json:"mailbox" description:"Mailbox name"`
	UID     uint32 `json:"uid" description:"Email UID"`
}

type MoveEmailParams struct {
	SourceMailbox string `json:"source_mailbox" description:"Source mailbox name"`
	TargetMailbox string `json:"target_mailbox" description:"Target mailbox name"`
	UID           uint32 `json:"uid" description:"Email UID"`
}

// sendEmail sends a plain text email
func (t *EmailTool) sendEmail(ctx context.Context, args map[string]interface{}) (string, error) {
	to, ok := args["to"].([]interface{})
	if !ok || len(to) == 0 {
		return "", fmt.Errorf("to is required and must be an array")
	}

	subject, ok := args["subject"].(string)
	if !ok {
		return "", fmt.Errorf("subject is required")
	}

	body, ok := args["body"].(string)
	if !ok {
		return "", fmt.Errorf("body is required")
	}

	// Convert to string slice
	toAddrs := make([]string, 0, len(to))
	for _, addr := range to {
		if addrStr, ok := addr.(string); ok {
			toAddrs = append(toAddrs, addrStr)
		}
	}

	var ccAddrs, bccAddrs []string
	if cc, ok := args["cc"].([]interface{}); ok {
		for _, addr := range cc {
			if addrStr, ok := addr.(string); ok {
				ccAddrs = append(ccAddrs, addrStr)
			}
		}
	}
	if bcc, ok := args["bcc"].([]interface{}); ok {
		for _, addr := range bcc {
			if addrStr, ok := addr.(string); ok {
				bccAddrs = append(bccAddrs, addrStr)
			}
		}
	}

	// Build email message
	msg := t.buildEmailMessage(toAddrs, ccAddrs, subject, body, false)

	// Send email
	allRecipients := append(toAddrs, ccAddrs...)
	allRecipients = append(allRecipients, bccAddrs...)

	err := t.sendSMTP(allRecipients, msg)
	if err != nil {
		return "", fmt.Errorf("failed to send email: %w", err)
	}

	result := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Email sent to %d recipients", len(allRecipients)),
		"to":      toAddrs,
	}

	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}

// sendHTMLEmail sends an HTML email
func (t *EmailTool) sendHTMLEmail(ctx context.Context, args map[string]interface{}) (string, error) {
	to, ok := args["to"].([]interface{})
	if !ok || len(to) == 0 {
		return "", fmt.Errorf("to is required and must be an array")
	}

	subject, ok := args["subject"].(string)
	if !ok {
		return "", fmt.Errorf("subject is required")
	}

	html, ok := args["html"].(string)
	if !ok {
		return "", fmt.Errorf("html is required")
	}

	// Convert to string slice
	toAddrs := make([]string, 0, len(to))
	for _, addr := range to {
		if addrStr, ok := addr.(string); ok {
			toAddrs = append(toAddrs, addrStr)
		}
	}

	var ccAddrs, bccAddrs []string
	if cc, ok := args["cc"].([]interface{}); ok {
		for _, addr := range cc {
			if addrStr, ok := addr.(string); ok {
				ccAddrs = append(ccAddrs, addrStr)
			}
		}
	}
	if bcc, ok := args["bcc"].([]interface{}); ok {
		for _, addr := range bcc {
			if addrStr, ok := addr.(string); ok {
				bccAddrs = append(bccAddrs, addrStr)
			}
		}
	}

	// Build HTML email message
	msg := t.buildEmailMessage(toAddrs, ccAddrs, subject, html, true)

	// Send email
	allRecipients := append(toAddrs, ccAddrs...)
	allRecipients = append(allRecipients, bccAddrs...)

	err := t.sendSMTP(allRecipients, msg)
	if err != nil {
		return "", fmt.Errorf("failed to send email: %w", err)
	}

	result := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("HTML email sent to %d recipients", len(allRecipients)),
		"to":      toAddrs,
	}

	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}

// listEmails lists emails from a mailbox
func (t *EmailTool) listEmails(ctx context.Context, args map[string]interface{}) (string, error) {
	mailbox := "INBOX"
	if mb, ok := args["mailbox"].(string); ok && mb != "" {
		mailbox = mb
	}

	limit := 20
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}

	unreadOnly := false
	if u, ok := args["unread"].(bool); ok {
		unreadOnly = u
	}

	c, err := t.connectIMAP()
	if err != nil {
		return "", fmt.Errorf("failed to connect to IMAP: %w", err)
	}
	defer c.Logout()

	// Select mailbox
	mbox, err := c.Select(mailbox, false)
	if err != nil {
		return "", fmt.Errorf("failed to select mailbox: %w", err)
	}

	if mbox.Messages == 0 {
		result := map[string]interface{}{
			"success": true,
			"emails":  []interface{}{},
			"count":   0,
		}
		resultJSON, _ := json.Marshal(result)
		return string(resultJSON), nil
	}

	// Build search criteria
	criteria := imap.NewSearchCriteria()
	if unreadOnly {
		criteria.WithoutFlags = []string{imap.SeenFlag}
	}

	// Search for messages
	uids, err := c.Search(criteria)
	if err != nil {
		return "", fmt.Errorf("failed to search emails: %w", err)
	}

	if len(uids) == 0 {
		result := map[string]interface{}{
			"success": true,
			"emails":  []interface{}{},
			"count":   0,
		}
		resultJSON, _ := json.Marshal(result)
		return string(resultJSON), nil
	}

	// Get the most recent emails
	start := len(uids) - limit
	if start < 0 {
		start = 0
	}
	uids = uids[start:]

	seqset := new(imap.SeqSet)
	seqset.AddNum(uids...)

	messages := make(chan *imap.Message, limit)
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
		return "", fmt.Errorf("failed to fetch emails: %w", err)
	}

	result := map[string]interface{}{
		"success": true,
		"emails":  emails,
		"count":   len(emails),
	}

	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}

// readEmail reads a specific email
func (t *EmailTool) readEmail(ctx context.Context, args map[string]interface{}) (string, error) {
	mailbox, ok := args["mailbox"].(string)
	if !ok || mailbox == "" {
		mailbox = "INBOX"
	}

	uid, ok := args["uid"].(float64)
	if !ok {
		return "", fmt.Errorf("uid is required")
	}

	c, err := t.connectIMAP()
	if err != nil {
		return "", fmt.Errorf("failed to connect to IMAP: %w", err)
	}
	defer c.Logout()

	// Select mailbox
	_, err = c.Select(mailbox, false)
	if err != nil {
		return "", fmt.Errorf("failed to select mailbox: %w", err)
	}

	seqset := new(imap.SeqSet)
	seqset.AddNum(uint32(uid))

	messages := make(chan *imap.Message, 1)
	done := make(chan error, 1)
	go func() {
		done <- c.Fetch(seqset, []imap.FetchItem{imap.FetchEnvelope, imap.FetchFlags, imap.FetchUid, imap.FetchRFC822}, messages)
	}()

	msg := <-messages
	if msg == nil {
		return "", fmt.Errorf("email not found")
	}

	if err := <-done; err != nil {
		return "", fmt.Errorf("failed to fetch email: %w", err)
	}

	// Parse email body
	section := &imap.BodySectionName{}
	r := msg.GetBody(section)
	if r == nil {
		return "", fmt.Errorf("failed to get email body")
	}

	mr, err := message.Read(r)
	if err != nil {
		return "", fmt.Errorf("failed to read email: %w", err)
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

	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}

// searchEmails searches for emails
func (t *EmailTool) searchEmails(ctx context.Context, args map[string]interface{}) (string, error) {
	mailbox := "INBOX"
	if mb, ok := args["mailbox"].(string); ok && mb != "" {
		mailbox = mb
	}

	query, ok := args["query"].(string)
	if !ok || query == "" {
		return "", fmt.Errorf("query is required")
	}

	limit := 20
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}

	c, err := t.connectIMAP()
	if err != nil {
		return "", fmt.Errorf("failed to connect to IMAP: %w", err)
	}
	defer c.Logout()

	// Select mailbox
	_, err = c.Select(mailbox, false)
	if err != nil {
		return "", fmt.Errorf("failed to select mailbox: %w", err)
	}

	// Build search criteria
	criteria := imap.NewSearchCriteria()
	criteria.Text = []string{query}

	// Search for messages
	uids, err := c.Search(criteria)
	if err != nil {
		return "", fmt.Errorf("failed to search emails: %w", err)
	}

	if len(uids) == 0 {
		result := map[string]interface{}{
			"success": true,
			"emails":  []interface{}{},
			"count":   0,
		}
		resultJSON, _ := json.Marshal(result)
		return string(resultJSON), nil
	}

	// Limit results
	if len(uids) > limit {
		uids = uids[len(uids)-limit:]
	}

	seqset := new(imap.SeqSet)
	seqset.AddNum(uids...)

	messages := make(chan *imap.Message, limit)
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
		return "", fmt.Errorf("failed to fetch emails: %w", err)
	}

	result := map[string]interface{}{
		"success": true,
		"emails":  emails,
		"count":   len(emails),
		"query":   query,
	}

	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}

// listMailboxes lists all mailboxes
func (t *EmailTool) listMailboxes(ctx context.Context, args map[string]interface{}) (string, error) {
	c, err := t.connectIMAP()
	if err != nil {
		return "", fmt.Errorf("failed to connect to IMAP: %w", err)
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
		return "", fmt.Errorf("failed to list mailboxes: %w", err)
	}

	result := map[string]interface{}{
		"success":   true,
		"mailboxes": mailboxList,
		"count":     len(mailboxList),
	}

	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}

// createMailbox creates a new mailbox
func (t *EmailTool) createMailbox(ctx context.Context, args map[string]interface{}) (string, error) {
	name, ok := args["name"].(string)
	if !ok || name == "" {
		return "", fmt.Errorf("name is required")
	}

	c, err := t.connectIMAP()
	if err != nil {
		return "", fmt.Errorf("failed to connect to IMAP: %w", err)
	}
	defer c.Logout()

	err = c.Create(name)
	if err != nil {
		return "", fmt.Errorf("failed to create mailbox: %w", err)
	}

	result := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Mailbox '%s' created successfully", name),
	}

	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}

// deleteMailbox deletes a mailbox
func (t *EmailTool) deleteMailbox(ctx context.Context, args map[string]interface{}) (string, error) {
	name, ok := args["name"].(string)
	if !ok || name == "" {
		return "", fmt.Errorf("name is required")
	}

	c, err := t.connectIMAP()
	if err != nil {
		return "", fmt.Errorf("failed to connect to IMAP: %w", err)
	}
	defer c.Logout()

	err = c.Delete(name)
	if err != nil {
		return "", fmt.Errorf("failed to delete mailbox: %w", err)
	}

	result := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Mailbox '%s' deleted successfully", name),
	}

	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}

// deleteEmail deletes an email
func (t *EmailTool) deleteEmail(ctx context.Context, args map[string]interface{}) (string, error) {
	mailbox, ok := args["mailbox"].(string)
	if !ok || mailbox == "" {
		mailbox = "INBOX"
	}

	uid, ok := args["uid"].(float64)
	if !ok {
		return "", fmt.Errorf("uid is required")
	}

	c, err := t.connectIMAP()
	if err != nil {
		return "", fmt.Errorf("failed to connect to IMAP: %w", err)
	}
	defer c.Logout()

	// Select mailbox
	_, err = c.Select(mailbox, false)
	if err != nil {
		return "", fmt.Errorf("failed to select mailbox: %w", err)
	}

	seqset := new(imap.SeqSet)
	seqset.AddNum(uint32(uid))

	// Mark as deleted
	item := imap.FormatFlagsOp(imap.AddFlags, true)
	flags := []interface{}{imap.DeletedFlag}
	err = c.Store(seqset, item, flags, nil)
	if err != nil {
		return "", fmt.Errorf("failed to mark email as deleted: %w", err)
	}

	// Expunge
	err = c.Expunge(nil)
	if err != nil {
		return "", fmt.Errorf("failed to expunge: %w", err)
	}

	result := map[string]interface{}{
		"success": true,
		"message": "Email deleted successfully",
	}

	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}

// markAsRead marks an email as read
func (t *EmailTool) markAsRead(ctx context.Context, args map[string]interface{}) (string, error) {
	return t.setFlag(args, imap.SeenFlag, true)
}

// markAsUnread marks an email as unread
func (t *EmailTool) markAsUnread(ctx context.Context, args map[string]interface{}) (string, error) {
	return t.setFlag(args, imap.SeenFlag, false)
}

// moveEmail moves an email to another mailbox
func (t *EmailTool) moveEmail(ctx context.Context, args map[string]interface{}) (string, error) {
	sourceMailbox, ok := args["source_mailbox"].(string)
	if !ok || sourceMailbox == "" {
		return "", fmt.Errorf("source_mailbox is required")
	}

	targetMailbox, ok := args["target_mailbox"].(string)
	if !ok || targetMailbox == "" {
		return "", fmt.Errorf("target_mailbox is required")
	}

	uid, ok := args["uid"].(float64)
	if !ok {
		return "", fmt.Errorf("uid is required")
	}

	c, err := t.connectIMAP()
	if err != nil {
		return "", fmt.Errorf("failed to connect to IMAP: %w", err)
	}
	defer c.Logout()

	// Select source mailbox
	_, err = c.Select(sourceMailbox, false)
	if err != nil {
		return "", fmt.Errorf("failed to select source mailbox: %w", err)
	}

	seqset := new(imap.SeqSet)
	seqset.AddNum(uint32(uid))

	// Copy to target mailbox
	err = c.Copy(seqset, targetMailbox)
	if err != nil {
		return "", fmt.Errorf("failed to copy email: %w", err)
	}

	// Mark original as deleted
	item := imap.FormatFlagsOp(imap.AddFlags, true)
	flags := []interface{}{imap.DeletedFlag}
	err = c.Store(seqset, item, flags, nil)
	if err != nil {
		return "", fmt.Errorf("failed to mark email as deleted: %w", err)
	}

	// Expunge
	err = c.Expunge(nil)
	if err != nil {
		return "", fmt.Errorf("failed to expunge: %w", err)
	}

	result := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Email moved from '%s' to '%s'", sourceMailbox, targetMailbox),
	}

	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}

// Helper methods

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

	// Connect to SMTP server with TLS
	addr := fmt.Sprintf("%s:%d", t.smtpHost, t.smtpPort)

	// Try STARTTLS first
	conn, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %w", err)
	}
	defer conn.Close()

	// Start TLS
	tlsConfig := &tls.Config{
		ServerName: t.smtpHost,
	}
	if err = conn.StartTLS(tlsConfig); err != nil {
		return fmt.Errorf("failed to start TLS: %w", err)
	}

	// Authenticate
	if err = conn.Auth(auth); err != nil {
		return fmt.Errorf("failed to authenticate: %w", err)
	}

	// Set sender
	if err = conn.Mail(t.fromAddress); err != nil {
		return fmt.Errorf("failed to set sender: %w", err)
	}

	// Set recipients
	for _, addr := range to {
		if err = conn.Rcpt(addr); err != nil {
			return fmt.Errorf("failed to set recipient %s: %w", addr, err)
		}
	}

	// Send message
	w, err := conn.Data()
	if err != nil {
		return fmt.Errorf("failed to get data writer: %w", err)
	}
	defer w.Close()

	_, err = w.Write(msg)
	if err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	return nil
}

func (t *EmailTool) connectIMAP() (*client.Client, error) {
	addr := fmt.Sprintf("%s:%d", t.imapHost, t.imapPort)

	// Connect with TLS
	tlsConfig := &tls.Config{
		ServerName: t.imapHost,
	}

	c, err := client.DialTLS(addr, tlsConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to IMAP server: %w", err)
	}

	// Login
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
		// Simple text email
		body, err := io.ReadAll(mr.Body)
		if err != nil {
			return "", err
		}
		return string(body), nil
	}

	// Multipart email
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

func (t *EmailTool) setFlag(args map[string]interface{}, flag string, add bool) (string, error) {
	mailbox, ok := args["mailbox"].(string)
	if !ok || mailbox == "" {
		mailbox = "INBOX"
	}

	uid, ok := args["uid"].(float64)
	if !ok {
		return "", fmt.Errorf("uid is required")
	}

	c, err := t.connectIMAP()
	if err != nil {
		return "", fmt.Errorf("failed to connect to IMAP: %w", err)
	}
	defer c.Logout()

	// Select mailbox
	_, err = c.Select(mailbox, false)
	if err != nil {
		return "", fmt.Errorf("failed to select mailbox: %w", err)
	}

	seqset := new(imap.SeqSet)
	seqset.AddNum(uint32(uid))

	var item imap.StoreItem
	if add {
		item = imap.FormatFlagsOp(imap.AddFlags, true)
	} else {
		item = imap.FormatFlagsOp(imap.RemoveFlags, true)
	}

	flags := []interface{}{flag}
	err = c.Store(seqset, item, flags, nil)
	if err != nil {
		return "", fmt.Errorf("failed to set flag: %w", err)
	}

	action := "added"
	if !add {
		action = "removed"
	}

	result := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Flag %s %s", flag, action),
	}

	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}

// GetJSONSchema returns the JSON schema for the Email tool
func (t *EmailTool) GetJSONSchema() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type": "string",
				"enum": []string{
					"send_email",
					"send_html_email",
					"list_emails",
					"read_email",
					"search_emails",
					"list_mailboxes",
					"create_mailbox",
					"delete_mailbox",
					"delete_email",
					"mark_as_read",
					"mark_as_unread",
					"move_email",
				},
				"description": "The action to perform",
			},
			"to": map[string]interface{}{
				"type":        "array",
				"items":       map[string]interface{}{"type": "string"},
				"description": "Recipient email addresses",
			},
			"subject": map[string]interface{}{
				"type":        "string",
				"description": "Email subject",
			},
			"body": map[string]interface{}{
				"type":        "string",
				"description": "Email body (plain text)",
			},
			"html": map[string]interface{}{
				"type":        "string",
				"description": "Email body (HTML)",
			},
			"cc": map[string]interface{}{
				"type":        "array",
				"items":       map[string]interface{}{"type": "string"},
				"description": "CC recipients",
			},
			"bcc": map[string]interface{}{
				"type":        "array",
				"items":       map[string]interface{}{"type": "string"},
				"description": "BCC recipients",
			},
			"mailbox": map[string]interface{}{
				"type":        "string",
				"description": "Mailbox name",
			},
			"limit": map[string]interface{}{
				"type":        "number",
				"description": "Number of items to return",
			},
			"unread": map[string]interface{}{
				"type":        "boolean",
				"description": "Only return unread emails",
			},
			"uid": map[string]interface{}{
				"type":        "number",
				"description": "Email UID",
			},
			"query": map[string]interface{}{
				"type":        "string",
				"description": "Search query",
			},
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Mailbox name",
			},
			"source_mailbox": map[string]interface{}{
				"type":        "string",
				"description": "Source mailbox name",
			},
			"target_mailbox": map[string]interface{}{
				"type":        "string",
				"description": "Target mailbox name",
			},
		},
		"required": []string{"action"},
	}
}
