// Package salesforce ports Agno's SalesforceTools: CRUD operations, SOQL
// queries, SOSL search, reports and metadata discovery for any Salesforce
// object through the REST API (/services/data/vXX.X).
//
// Authentication (pick one), as in Python:
//   - Username / password (+ security token): a SOAP partner login, the same
//     flow simple-salesforce uses (SALESFORCE_USERNAME, SALESFORCE_PASSWORD,
//     SALESFORCE_SECURITY_TOKEN, SALESFORCE_DOMAIN).
//   - Instance URL + session id (access token): WithInstanceURL/WithSessionID.
package salesforce

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/internal/restkit"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// DefaultAPIVersion is the REST API version (simple-salesforce default).
const DefaultAPIVersion = "59.0"

// Instructions mirrors Python's SALESFORCE_INSTRUCTIONS.
const Instructions = `You have access to Salesforce CRM tools for querying, creating, and managing records.

## SOQL (query tool)
- ` + "`SELECT Id, Name FROM Account WHERE Industry = 'Technology' LIMIT 10`" + `
- ` + "`SELECT Id, Name, Account.Name FROM Contact WHERE Account.Industry = 'Technology'`" + ` — relationship query
- ` + "`SELECT COUNT() FROM Lead WHERE Status = 'Open - Not Contacted'`" + ` — aggregate

## SOSL (search tool)
SOSL requires FIND with curly braces and explicit RETURNING with object names and fields:
- ` + "`FIND {search term} IN ALL FIELDS RETURNING Account(Id, Name), Contact(Id, Name, Email)`" + `
- ` + "`FIND {John} IN NAME FIELDS RETURNING Lead(Id, Name, Company)`" + `
SOSL does NOT support ` + "`RETURNING ALL OBJECTS`" + ` or ` + "`RETURNING ALL FIELDS`" + `. You must specify object names and fields.

## Creating Records
Use describe_object first to discover field names. Skip fields where defaultedOnCreate is true (e.g. OwnerId) — they are auto-populated.`

// SalesforceTools manages Salesforce CRM records.
type SalesforceTools struct {
	toolkit.Toolkit
	username, password, securityToken, domain string
	instanceURL, sessionID                    string
	loginBaseURL                              string
	apiVersion                                string
	maxRecords, maxFields                     int
	enabled                                   map[string]bool
	all                                       bool
	instructions                              string
	addInstructions                           bool
	httpClient                                *http.Client

	mu     sync.Mutex
	client *restkit.Client
}

// Option configures SalesforceTools.
type Option func(*SalesforceTools)

// WithUsername sets the username (default: SALESFORCE_USERNAME).
func WithUsername(v string) Option { return func(s *SalesforceTools) { s.username = v } }

// WithPassword sets the password (default: SALESFORCE_PASSWORD).
func WithPassword(v string) Option { return func(s *SalesforceTools) { s.password = v } }

// WithSecurityToken sets the security token (default: SALESFORCE_SECURITY_TOKEN).
func WithSecurityToken(v string) Option { return func(s *SalesforceTools) { s.securityToken = v } }

// WithDomain sets the login domain, "login" or "test" (default: SALESFORCE_DOMAIN or "login").
func WithDomain(v string) Option { return func(s *SalesforceTools) { s.domain = v } }

// WithInstanceURL sets the org instance URL (used with WithSessionID).
func WithInstanceURL(v string) Option { return func(s *SalesforceTools) { s.instanceURL = v } }

// WithSessionID sets a session id / OAuth access token (used with WithInstanceURL).
func WithSessionID(v string) Option { return func(s *SalesforceTools) { s.sessionID = v } }

// WithBaseURL overrides the login base URL (default https://{domain}.salesforce.com).
func WithBaseURL(v string) Option { return func(s *SalesforceTools) { s.loginBaseURL = v } }

// WithAPIVersion sets the REST API version (default 59.0).
func WithAPIVersion(v string) Option { return func(s *SalesforceTools) { s.apiVersion = v } }

// WithMaxRecords caps returned records (default 200).
func WithMaxRecords(n int) Option { return func(s *SalesforceTools) { s.maxRecords = n } }

// WithMaxFields caps fields returned by DescribeObject (default 100).
func WithMaxFields(n int) Option { return func(s *SalesforceTools) { s.maxFields = n } }

// WithTimeout sets the HTTP timeout.
func WithTimeout(d time.Duration) Option { return func(s *SalesforceTools) { s.Toolkit.Timeout = d } }

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(s *SalesforceTools) { s.httpClient = c } }

// WithInstructions overrides the toolkit instructions.
func WithInstructions(v string) Option { return func(s *SalesforceTools) { s.instructions = v } }

// WithAddInstructions toggles appending instructions to the description (default true).
func WithAddInstructions(v bool) Option { return func(s *SalesforceTools) { s.addInstructions = v } }

func enable(name string, v bool) Option {
	return func(s *SalesforceTools) { s.enabled[name] = v }
}

// WithEnableListObjects toggles ListObjects (default true).
func WithEnableListObjects(v bool) Option { return enable("ListObjects", v) }

// WithEnableDescribeObject toggles DescribeObject (default true).
func WithEnableDescribeObject(v bool) Option { return enable("DescribeObject", v) }

// WithEnableGetRecord toggles GetRecord (default true).
func WithEnableGetRecord(v bool) Option { return enable("GetRecord", v) }

// WithEnableQuery toggles Query (default true).
func WithEnableQuery(v bool) Option { return enable("Query", v) }

// WithEnableSearch toggles Search (default true).
func WithEnableSearch(v bool) Option { return enable("Search", v) }

// WithEnableCreateRecord toggles CreateRecord (default false).
func WithEnableCreateRecord(v bool) Option { return enable("CreateRecord", v) }

// WithEnableUpdateRecord toggles UpdateRecord (default false).
func WithEnableUpdateRecord(v bool) Option { return enable("UpdateRecord", v) }

// WithEnableDeleteRecord toggles DeleteRecord (default false).
func WithEnableDeleteRecord(v bool) Option { return enable("DeleteRecord", v) }

// WithEnableGetReport toggles GetReport (default false).
func WithEnableGetReport(v bool) Option { return enable("GetReport", v) }

// WithAll enables every tool.
func WithAll() Option { return func(s *SalesforceTools) { s.all = true } }

// EmptyParams is used by methods without arguments.
type EmptyParams struct{}

// SObjectParams identifies an object type.
type SObjectParams struct {
	SObject string `json:"sobject" description:"API name of the Salesforce object (e.g. Account, Contact, Lead, Opportunity)." required:"true"`
}

// GetRecordParams are the parameters of GetRecord.
type GetRecordParams struct {
	SObject  string `json:"sobject" description:"API name of the Salesforce object." required:"true"`
	RecordID string `json:"record_id" description:"The 15 or 18 character Salesforce record ID." required:"true"`
	Fields   string `json:"fields,omitempty" description:"Comma-separated field names to return. If empty, returns all fields."`
}

// CreateRecordParams are the parameters of CreateRecord.
type CreateRecordParams struct {
	SObject    string `json:"sobject" description:"API name of the Salesforce object." required:"true"`
	RecordData string `json:"record_data" description:"JSON string of field name-value pairs." required:"true"`
}

// UpdateRecordParams are the parameters of UpdateRecord.
type UpdateRecordParams struct {
	SObject    string `json:"sobject" description:"API name of the Salesforce object." required:"true"`
	RecordID   string `json:"record_id" description:"The Salesforce record ID to update." required:"true"`
	RecordData string `json:"record_data" description:"JSON string of field name-value pairs to update." required:"true"`
}

// DeleteRecordParams are the parameters of DeleteRecord.
type DeleteRecordParams struct {
	SObject  string `json:"sobject" description:"API name of the Salesforce object." required:"true"`
	RecordID string `json:"record_id" description:"The Salesforce record ID to delete." required:"true"`
}

// QueryParams are the parameters of Query.
type QueryParams struct {
	SOQL string `json:"soql" description:"The SOQL query string." required:"true"`
}

// SearchParams are the parameters of Search.
type SearchParams struct {
	SOSL string `json:"sosl" description:"The SOSL search string." required:"true"`
}

// GetReportParams are the parameters of GetReport.
type GetReportParams struct {
	ReportID string `json:"report_id" description:"The Salesforce report ID (15 or 18 character)." required:"true"`
}

// New creates the Salesforce toolkit. Credentials are resolved now but the
// login happens lazily on the first call; missing credentials yield an error
// at call time.
func New(opts ...Option) *SalesforceTools {
	s := &SalesforceTools{
		apiVersion:      DefaultAPIVersion,
		maxRecords:      200,
		maxFields:       100,
		addInstructions: true,
		enabled: map[string]bool{
			"ListObjects": true, "DescribeObject": true, "GetRecord": true, "Query": true, "Search": true,
		},
	}
	s.Toolkit = toolkit.NewToolkit()
	s.Toolkit.Name = "salesforce_tools"
	s.Toolkit.Description = "Query, search, create and manage Salesforce CRM records."
	for _, o := range opts {
		o(s)
	}
	s.username = restkit.Env(s.username, "SALESFORCE_USERNAME")
	s.password = restkit.Env(s.password, "SALESFORCE_PASSWORD")
	s.securityToken = restkit.Env(s.securityToken, "SALESFORCE_SECURITY_TOKEN")
	s.domain = restkit.Env(s.domain, "SALESFORCE_DOMAIN")
	if s.domain == "" {
		s.domain = "login"
	}
	if s.loginBaseURL == "" {
		s.loginBaseURL = "https://" + s.domain + ".salesforce.com"
	}
	if s.httpClient == nil {
		s.httpClient = s.Toolkit.HTTPClient()
	}
	if s.addInstructions {
		ins := s.instructions
		if ins == "" {
			ins = Instructions
		}
		s.Toolkit.Description += "\n\n" + ins
	}

	reg := func(name, desc string, fn, params interface{}) {
		if s.all || s.enabled[name] {
			s.Toolkit.Register(name, desc, s, fn, params)
		}
	}
	reg("ListObjects", "List all available Salesforce objects in the org.", s.ListObjects, EmptyParams{})
	reg("DescribeObject", "Get the schema for a Salesforce object including field names, types, and picklist values. Use this before creating or updating records to discover required fields.", s.DescribeObject, SObjectParams{})
	reg("GetRecord", "Get a single Salesforce record by its ID.", s.GetRecord, GetRecordParams{})
	reg("Query", "Execute a SOQL query against Salesforce.", s.Query, QueryParams{})
	reg("Search", "Execute a SOSL full-text search across Salesforce objects.", s.Search, SearchParams{})
	reg("CreateRecord", "Create a new Salesforce record.", s.CreateRecord, CreateRecordParams{})
	reg("UpdateRecord", "Update an existing Salesforce record.", s.UpdateRecord, UpdateRecordParams{})
	reg("DeleteRecord", "Delete a Salesforce record.", s.DeleteRecord, DeleteRecordParams{})
	reg("GetReport", "Run a Salesforce report and return the results.", s.GetReport, GetReportParams{})
	return s
}

// api returns an authenticated REST client, logging in when needed.
func (s *SalesforceTools) api() (*restkit.Client, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.client != nil {
		return s.client, nil
	}
	instance, session := s.instanceURL, s.sessionID
	if instance == "" || session == "" {
		if s.username == "" || s.password == "" {
			return nil, ErrNoCredentials
		}
		var err error
		instance, session, err = s.soapLogin()
		if err != nil {
			return nil, err
		}
	}
	s.client = &restkit.Client{
		HTTP:    s.httpClient,
		BaseURL: strings.TrimRight(instance, "/") + "/services/data/v" + s.apiVersion,
		Name:    "Salesforce API",
		Headers: map[string]string{"Authorization": "Bearer " + session},
	}
	return s.client, nil
}

func xmlEscape(v string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(v))
	return b.String()
}

// soapLogin performs the partner SOAP login (as simple-salesforce does).
func (s *SalesforceTools) soapLogin() (string, string, error) {
	body := `<?xml version="1.0" encoding="utf-8" ?>
<env:Envelope xmlns:xsd="http://www.w3.org/2001/XMLSchema" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:env="http://schemas.xmlsoap.org/soap/envelope/" xmlns:urn="urn:partner.soap.sforce.com">
<env:Header><urn:CallOptions><urn:client>agno</urn:client><urn:defaultNamespace>sf</urn:defaultNamespace></urn:CallOptions></env:Header>
<env:Body><n1:login xmlns:n1="urn:partner.soap.sforce.com"><n1:username>` + xmlEscape(s.username) + `</n1:username><n1:password>` +
		xmlEscape(s.password+s.securityToken) + `</n1:password></n1:login></env:Body>
</env:Envelope>`
	loginURL := strings.TrimRight(s.loginBaseURL, "/") + "/services/Soap/u/" + s.apiVersion
	req, err := http.NewRequest(http.MethodPost, loginURL, strings.NewReader(body))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "text/xml; charset=UTF-8")
	req.Header.Set("SOAPAction", "login")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("Salesforce login failed: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, restkit.MaxBodyBytes))
	var env struct {
		SessionID   string `xml:"Body>loginResponse>result>sessionId"`
		ServerURL   string `xml:"Body>loginResponse>result>serverUrl"`
		FaultString string `xml:"Body>Fault>faultstring"`
	}
	_ = xml.Unmarshal(data, &env)
	if resp.StatusCode != http.StatusOK || env.SessionID == "" {
		msg := env.FaultString
		if msg == "" {
			msg = strings.TrimSpace(restkit.Truncate(string(data), 500))
		}
		return "", "", fmt.Errorf("Salesforce login failed (status %d): %s", resp.StatusCode, msg)
	}
	u, err := url.Parse(env.ServerURL)
	if err != nil || u.Host == "" {
		return "", "", fmt.Errorf("Salesforce login returned invalid serverUrl %q", env.ServerURL)
	}
	return u.Scheme + "://" + u.Host, env.SessionID, nil
}

// ErrNoCredentials is returned when neither username/password nor
// instance URL + session id are configured.
var ErrNoCredentials = errors.New("Salesforce credentials not configured. Set SALESFORCE_USERNAME, SALESFORCE_PASSWORD, and SALESFORCE_SECURITY_TOKEN or pass instance_url and session_id")

func errOut(err error) (interface{}, error) {
	if errors.Is(err, ErrNoCredentials) {
		return nil, err
	}
	return restkit.ErrorJSON(err.Error()), nil
}

func (s *SalesforceTools) call(method, path string, query url.Values, body interface{}) (interface{}, error) {
	c, err := s.api()
	if err != nil {
		return nil, err
	}
	return c.JSON(method, path, query, body)
}

func (s *SalesforceTools) callMap(method, path string, query url.Values, body interface{}) (map[string]interface{}, error) {
	v, err := s.call(method, path, query, body)
	if err != nil {
		return nil, err
	}
	m, _ := v.(map[string]interface{})
	return m, nil
}

// ListObjects lists all available Salesforce objects in the org.
func (s *SalesforceTools) ListObjects(_ EmptyParams) (interface{}, error) {
	describe, err := s.callMap(http.MethodGet, "/sobjects/", nil, nil)
	if err != nil {
		return errOut(err)
	}
	objects := []map[string]interface{}{}
	for _, obj := range restkit.Slice(describe, "sobjects") {
		objects = append(objects, map[string]interface{}{
			"name":       restkit.Str(obj, "name"),
			"label":      obj["label"],
			"queryable":  obj["queryable"],
			"createable": obj["createable"],
			"updateable": obj["updateable"],
			"deletable":  obj["deletable"],
		})
	}
	total := len(objects)
	if total > s.maxRecords {
		objects = objects[:s.maxRecords]
	}
	return restkit.CompactJSON(map[string]interface{}{"total": total, "returned": len(objects), "objects": objects}), nil
}

// DescribeObject returns the schema for a Salesforce object.
func (s *SalesforceTools) DescribeObject(params SObjectParams) (interface{}, error) {
	describe, err := s.callMap(http.MethodGet, "/sobjects/"+url.PathEscape(params.SObject)+"/describe", nil, nil)
	if err != nil {
		return errOut(err)
	}
	all := restkit.Slice(describe, "fields")
	limited := all
	if len(limited) > s.maxFields {
		limited = limited[:s.maxFields]
	}
	fields := []map[string]interface{}{}
	for _, f := range limited {
		info := map[string]interface{}{
			"name":              f["name"],
			"label":             f["label"],
			"type":              f["type"],
			"nillable":          f["nillable"],
			"createable":        f["createable"],
			"updateable":        f["updateable"],
			"defaultedOnCreate": f["defaultedOnCreate"],
		}
		if pl := restkit.Slice(f, "picklistValues"); len(pl) > 0 {
			vals := []map[string]interface{}{}
			for _, p := range pl {
				if active, _ := p["active"].(bool); active {
					vals = append(vals, map[string]interface{}{"value": p["value"], "label": p["label"]})
				}
			}
			info["picklistValues"] = vals
		}
		if restkit.Str(f, "type") == "reference" {
			info["referenceTo"] = f["referenceTo"]
		}
		fields = append(fields, info)
	}
	return restkit.CompactJSON(map[string]interface{}{
		"name":           describe["name"],
		"label":          describe["label"],
		"createable":     describe["createable"],
		"updateable":     describe["updateable"],
		"deletable":      describe["deletable"],
		"totalFields":    len(all),
		"returnedFields": len(fields),
		"fields":         fields,
	}), nil
}

// GetRecord returns a single record by id.
func (s *SalesforceTools) GetRecord(params GetRecordParams) (interface{}, error) {
	if params.Fields != "" {
		fieldList := strings.ReplaceAll(params.Fields, " ", "")
		soql := fmt.Sprintf("SELECT %s FROM %s WHERE Id = '%s'", fieldList, params.SObject, strings.ReplaceAll(params.RecordID, "'", `\'`))
		result, err := s.callMap(http.MethodGet, "/query/", url.Values{"q": {soql}}, nil)
		if err != nil {
			return errOut(err)
		}
		records, _ := result["records"].([]interface{})
		if len(records) == 0 {
			return restkit.ErrorJSON(fmt.Sprintf("%s record %s not found.", params.SObject, params.RecordID)), nil
		}
		return restkit.CompactJSON(records[0]), nil
	}
	record, err := s.call(http.MethodGet, "/sobjects/"+url.PathEscape(params.SObject)+"/"+url.PathEscape(params.RecordID), nil, nil)
	if err != nil {
		return errOut(err)
	}
	return restkit.CompactJSON(record), nil
}

func parseRecordData(raw string) (map[string]interface{}, error) {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return nil, err
	}
	return data, nil
}

// CreateRecord creates a new record.
func (s *SalesforceTools) CreateRecord(params CreateRecordParams) (interface{}, error) {
	data, err := parseRecordData(params.RecordData)
	if err != nil {
		return restkit.ErrorJSON("Invalid JSON in record_data: " + err.Error()), nil
	}
	result, err := s.callMap(http.MethodPost, "/sobjects/"+url.PathEscape(params.SObject)+"/", nil, data)
	if err != nil {
		return errOut(err)
	}
	return restkit.CompactJSON(map[string]interface{}{"id": result["id"], "success": result["success"], "sobject": params.SObject}), nil
}

// UpdateRecord updates an existing record.
func (s *SalesforceTools) UpdateRecord(params UpdateRecordParams) (interface{}, error) {
	data, err := parseRecordData(params.RecordData)
	if err != nil {
		return restkit.ErrorJSON("Invalid JSON in record_data: " + err.Error()), nil
	}
	if _, err := s.call(http.MethodPatch, "/sobjects/"+url.PathEscape(params.SObject)+"/"+url.PathEscape(params.RecordID), nil, data); err != nil {
		return errOut(err)
	}
	return restkit.CompactJSON(map[string]interface{}{"status": "success", "id": params.RecordID, "sobject": params.SObject}), nil
}

// DeleteRecord deletes a record.
func (s *SalesforceTools) DeleteRecord(params DeleteRecordParams) (interface{}, error) {
	if _, err := s.call(http.MethodDelete, "/sobjects/"+url.PathEscape(params.SObject)+"/"+url.PathEscape(params.RecordID), nil, nil); err != nil {
		return errOut(err)
	}
	return restkit.CompactJSON(map[string]interface{}{"status": "success", "id": params.RecordID, "sobject": params.SObject}), nil
}

// Query executes a SOQL query.
func (s *SalesforceTools) Query(params QueryParams) (interface{}, error) {
	result, err := s.callMap(http.MethodGet, "/query/", url.Values{"q": {params.SOQL}}, nil)
	if err != nil {
		return errOut(err)
	}
	records, _ := result["records"].([]interface{})
	if records == nil {
		records = []interface{}{}
	}
	totalSize, ok := result["totalSize"]
	if !ok {
		totalSize = len(records)
	}
	if len(records) > s.maxRecords {
		records = records[:s.maxRecords]
	}
	return restkit.CompactJSON(map[string]interface{}{
		"totalSize": totalSize, "returned": len(records), "done": result["done"], "records": records,
	}), nil
}

// Search executes a SOSL search.
func (s *SalesforceTools) Search(params SearchParams) (interface{}, error) {
	result, err := s.call(http.MethodGet, "/search/", url.Values{"q": {params.SOSL}}, nil)
	if err != nil {
		return errOut(err)
	}
	if m, ok := result.(map[string]interface{}); ok {
		if records, ok := m["searchRecords"].([]interface{}); ok && len(records) > s.maxRecords {
			m["searchRecords"] = records[:s.maxRecords]
		}
	}
	return restkit.CompactJSON(result), nil
}

// GetReport runs a report and returns its metadata and fact map.
func (s *SalesforceTools) GetReport(params GetReportParams) (interface{}, error) {
	result, err := s.call(http.MethodGet, "/analytics/reports/"+url.PathEscape(params.ReportID), nil, nil)
	if err != nil {
		return errOut(err)
	}
	m, ok := result.(map[string]interface{})
	if !ok {
		return restkit.ErrorJSON("Unexpected report response format."), nil
	}
	return restkit.CompactJSON(map[string]interface{}{"reportMetadata": m["reportMetadata"], "factMap": m["factMap"]}), nil
}

// Execute implements toolkit.Tool.
func (s *SalesforceTools) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return s.Toolkit.Execute(methodName, input)
}
