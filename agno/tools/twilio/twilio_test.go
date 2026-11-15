package twilio

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTwilio(t *testing.T) {
	var reqs []*http.Request
	var forms []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		reqs = append(reqs, r)
		forms = append(forms, r.PostForm.Get("To")+"|"+r.PostForm.Get("From")+"|"+r.PostForm.Get("Body"))
		switch r.URL.Path {
		case "/Accounts/AC1/Messages.json":
			if r.Method == "POST" {
				_, _ = w.Write([]byte(`{"sid":"SM1"}`))
				return
			}
			_, _ = w.Write([]byte(`{"messages":[{"sid":"SM1","to":"+1","from":"+2","body":"hi","status":"sent","date_sent":"Tue, 10 Aug 2010 08:02:17 +0000"}]}`))
		case "/Accounts/AC1/Calls/CA1.json":
			_, _ = w.Write([]byte(`{"to":"+1","from":"+2","status":"completed","duration":"10","start_time":"Tue, 10 Aug 2010 08:02:17 +0000","end_time":null}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	tk := New(WithAccountSID("AC1"), WithAuthToken("tok"), WithBaseURL(srv.URL))
	out, err := tk.SendSms(SendSmsParams{To: "+15551234567", From: "+15557654321", Body: "hello"})
	if err != nil || out != "Message sent successfully. SID: SM1" {
		t.Fatalf("out=%v err=%v", out, err)
	}
	if u, p, ok := reqs[0].BasicAuth(); !ok || u != "AC1" || p != "tok" || reqs[0].Method != "POST" {
		t.Fatalf("bad auth/method")
	}
	if forms[0] != "+15551234567|+15557654321|hello" {
		t.Fatalf("bad form %s", forms[0])
	}

	call, _ := tk.GetCallDetails(GetCallDetailsParams{CallSID: "CA1"})
	cm := call.(map[string]interface{})
	if cm["status"] != "completed" || cm["start_time"] != "2010-08-10 08:02:17+00:00" || cm["end_time"] != "None" {
		t.Fatalf("bad call %v", cm)
	}

	msgs, _ := tk.ListMessages(ListMessagesParams{})
	list := msgs.([]map[string]interface{})
	if len(list) != 1 || list[0]["sid"] != "SM1" || reqs[2].URL.Query().Get("PageSize") != "20" {
		t.Fatalf("bad list %v", list)
	}

	if out, _ := tk.SendSms(SendSmsParams{To: "123", From: "+15557654321", Body: "x"}); out != "Error: 'to' number must be in E.164 format (e.g., +1234567890)" {
		t.Fatalf("got %v", out)
	}
}

func TestAPIKeyAuthAndRegion(t *testing.T) {
	tk := New(WithAccountSID("AC1"), WithAPIKey("SK"), WithAPISecret("sec"), WithRegion("au1"), WithEdge("sydney"))
	if tk.client.BasicUser != "SK" || tk.client.BaseURL != "https://api.sydney.au1.twilio.com/2010-04-01" {
		t.Fatalf("got %s %s", tk.client.BasicUser, tk.client.BaseURL)
	}
}
