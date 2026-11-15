package awslambda

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLambda(t *testing.T) {
	var auths []string
	var invokeBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auths = append(auths, r.Header.Get("Authorization"))
		switch r.Method + " " + r.URL.Path {
		case "GET /2015-03-31/functions/":
			_, _ = w.Write([]byte(`{"Functions":[{"FunctionName":"alpha"},{"FunctionName":"beta"}]}`))
		case "POST /2015-03-31/functions/alpha/invocations":
			data, _ := io.ReadAll(r.Body)
			invokeBody = string(data)
			_, _ = w.Write([]byte(`{"ok":true}`))
		default:
			http.Error(w, `{"message":"nope"}`, 404)
		}
	}))
	defer srv.Close()

	a := New(WithStaticCredentials("AKID", "SECRET", ""), WithRegion("eu-west-1"), WithBaseURL(srv.URL))
	out, _ := a.ListFunctions(ListFunctionsParams{})
	if out != "Available Lambda functions: alpha, beta" {
		t.Fatalf("got %v", out)
	}
	out, _ = a.InvokeFunction(InvokeFunctionParams{FunctionName: "alpha"})
	if out != `Function invoked successfully. Status code: 200, Payload: {"ok":true}` || invokeBody != "{}" {
		t.Fatalf("got %v body=%s", out, invokeBody)
	}
	for _, h := range auths {
		if !strings.HasPrefix(h, "AWS4-HMAC-SHA256 Credential=AKID/") || !strings.Contains(h, "/eu-west-1/lambda/aws4_request") {
			t.Fatalf("bad auth %q", h)
		}
	}
	out, _ = a.InvokeFunction(InvokeFunctionParams{FunctionName: "missing"})
	if !strings.HasPrefix(out.(string), "Error invoking function:") {
		t.Fatalf("got %v", out)
	}
}

func TestDefaults(t *testing.T) {
	a := New(WithEnableInvokeFunction(false))
	if a.baseURL != "https://lambda.us-east-1.amazonaws.com" || len(a.GetMethods()) != 1 {
		t.Fatalf("base=%s methods=%d", a.baseURL, len(a.GetMethods()))
	}
}
