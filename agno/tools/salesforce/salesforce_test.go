package salesforce

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSessionAuthQueryAndCRUD(t *testing.T) {
	var lastMethod, lastPath, lastQ, lastAuth string
	var lastBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastMethod, lastPath, lastQ, lastAuth = r.Method, r.URL.Path, r.URL.Query().Get("q"), r.Header.Get("Authorization")
		lastBody = nil
		_ = json.NewDecoder(r.Body).Decode(&lastBody)
		switch {
		case strings.HasSuffix(r.URL.Path, "/query/"):
			_, _ = w.Write([]byte(`{"totalSize":3,"done":true,"records":[{"Id":"1"},{"Id":"2"},{"Id":"3"}]}`))
		case r.Method == http.MethodPost:
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"001X","success":true,"errors":[]}`))
		case r.Method == http.MethodPatch:
			w.WriteHeader(http.StatusNoContent)
		case strings.HasSuffix(r.URL.Path, "/describe"):
			_, _ = w.Write([]byte(`{"name":"Lead","label":"Lead","fields":[{"name":"Status","type":"picklist","picklistValues":[{"value":"Open","label":"Open","active":true},{"value":"Old","label":"Old","active":false}]},{"name":"OwnerId","type":"reference","referenceTo":["User"]}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`[{"errorCode":"NOT_FOUND"}]`))
		}
	}))
	defer srv.Close()

	s := New(WithInstanceURL(srv.URL), WithSessionID("tok"), WithMaxRecords(2), WithAll())
	if len(s.GetMethods()) != 9 {
		t.Fatalf("expected 9 methods, got %d", len(s.GetMethods()))
	}

	out, _ := s.Query(QueryParams{SOQL: "SELECT Id FROM Account"})
	if lastPath != "/services/data/v59.0/query/" || lastQ != "SELECT Id FROM Account" || lastAuth != "Bearer tok" {
		t.Fatalf("bad query request %s %s %s", lastPath, lastQ, lastAuth)
	}
	var q map[string]interface{}
	_ = json.Unmarshal([]byte(out.(string)), &q)
	if q["totalSize"] != float64(3) || q["returned"] != float64(2) {
		t.Fatalf("unexpected query output %s", out)
	}

	out, _ = s.CreateRecord(CreateRecordParams{SObject: "Lead", RecordData: `{"LastName":"Doe"}`})
	if lastMethod != http.MethodPost || lastPath != "/services/data/v59.0/sobjects/Lead/" || lastBody["LastName"] != "Doe" {
		t.Fatalf("bad create request %s %s %v", lastMethod, lastPath, lastBody)
	}
	if out != `{"id":"001X","sobject":"Lead","success":true}` {
		t.Fatalf("unexpected create output %s", out)
	}

	out, _ = s.UpdateRecord(UpdateRecordParams{SObject: "Lead", RecordID: "001X", RecordData: `{"Company":"Acme"}`})
	if lastMethod != http.MethodPatch || lastPath != "/services/data/v59.0/sobjects/Lead/001X" || !strings.Contains(out.(string), `"status":"success"`) {
		t.Fatalf("bad update %s %s %s", lastMethod, lastPath, out)
	}

	out, _ = s.DescribeObject(SObjectParams{SObject: "Lead"})
	if !strings.Contains(out.(string), `"picklistValues":[{"label":"Open","value":"Open"}]`) || !strings.Contains(out.(string), `"referenceTo":["User"]`) {
		t.Fatalf("unexpected describe output %s", out)
	}

	out, _ = s.GetRecord(GetRecordParams{SObject: "Lead", RecordID: "nope"})
	if !strings.Contains(out.(string), `"error"`) || !strings.Contains(out.(string), "404") {
		t.Fatalf("expected error output, got %s", out)
	}

	if out, _ := s.CreateRecord(CreateRecordParams{SObject: "Lead", RecordData: "{bad"}); !strings.Contains(out.(string), "Invalid JSON in record_data") {
		t.Fatalf("expected invalid JSON error, got %s", out)
	}
}

func TestSOAPLogin(t *testing.T) {
	var soapBody string
	var srvURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/services/Soap/u/59.0" {
			b, _ := io.ReadAll(r.Body)
			soapBody = string(b)
			_, _ = w.Write([]byte(`<?xml version="1.0"?><soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/"><soapenv:Body><loginResponse><result><serverUrl>` + srvURL + `/services/Soap/u/59.0/00D</serverUrl><sessionId>SESS</sessionId></result></loginResponse></soapenv:Body></soapenv:Envelope>`))
			return
		}
		if r.Header.Get("Authorization") != "Bearer SESS" || r.URL.Path != "/services/data/v59.0/sobjects/" {
			t.Errorf("bad request %s %s", r.URL.Path, r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte(`{"sobjects":[{"name":"Account","label":"Account","queryable":true}]}`))
	}))
	defer srv.Close()
	srvURL = srv.URL

	s := New(WithUsername("u@x.com"), WithPassword("p&w"), WithSecurityToken("TT"), WithBaseURL(srv.URL))
	out, err := s.ListObjects(EmptyParams{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(soapBody, "<n1:password>p&amp;wTT</n1:password>") {
		t.Fatalf("bad soap body %s", soapBody)
	}
	if !strings.Contains(out.(string), `"total":1`) {
		t.Fatalf("unexpected output %s", out)
	}
}

func TestMissingCredentials(t *testing.T) {
	for _, k := range []string{"SALESFORCE_USERNAME", "SALESFORCE_PASSWORD"} {
		t.Setenv(k, "")
	}
	if _, err := New().Query(QueryParams{SOQL: "x"}); !errors.Is(err, ErrNoCredentials) {
		t.Fatalf("expected ErrNoCredentials, got %v", err)
	}
}
