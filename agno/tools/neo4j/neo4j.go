// Package neo4j ports Agno's Neo4jTools using the Neo4j HTTP Query API v2
// (POST /db/{database}/query/v2) instead of the Bolt driver, so no extra
// dependency is required.
package neo4j

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/internal/restapi"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// Neo4jTools lists labels/relationship types, reads the schema and runs Cypher.
type Neo4jTools struct {
	toolkit.Toolkit
	uri, user, password, database string
	httpClient                    *http.Client

	enableLabels, enableRels, enableSchema, enableCypher bool
}

// Option configures Neo4jTools.
type Option func(*Neo4jTools)

// WithURI sets the server URI (default NEO4J_URI, else bolt://localhost:7687).
// bolt:// and neo4j:// URIs are mapped to http://<host>:7474; http(s) URIs are used as-is.
func WithURI(u string) Option { return func(n *Neo4jTools) { n.uri = u } }

// WithAuth sets user and password (default NEO4J_USERNAME / NEO4J_PASSWORD).
func WithAuth(user, password string) Option {
	return func(n *Neo4jTools) { n.user, n.password = user, password }
}

// WithDatabase sets the database (default "neo4j").
func WithDatabase(db string) Option { return func(n *Neo4jTools) { n.database = db } }

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(n *Neo4jTools) { n.httpClient = c } }

// WithTimeout sets the request timeout.
func WithTimeout(d time.Duration) Option { return func(n *Neo4jTools) { n.Toolkit.Timeout = d } }

// WithListLabels toggles ListLabels (default true).
func WithListLabels(b bool) Option { return func(n *Neo4jTools) { n.enableLabels = b } }

// WithListRelationships toggles ListRelationshipTypes (default true).
func WithListRelationships(b bool) Option { return func(n *Neo4jTools) { n.enableRels = b } }

// WithGetSchema toggles GetSchema (default true).
func WithGetSchema(b bool) Option { return func(n *Neo4jTools) { n.enableSchema = b } }

// WithRunCypher toggles RunCypherQuery (default true).
func WithRunCypher(b bool) Option { return func(n *Neo4jTools) { n.enableCypher = b } }

// WithAll enables every method.
func WithAll() Option {
	return func(n *Neo4jTools) {
		n.enableLabels, n.enableRels, n.enableSchema, n.enableCypher = true, true, true, true
	}
}

// NoParams is used by parameterless methods.
type NoParams struct{}

// CypherParams for RunCypherQuery.
type CypherParams struct {
	Query string `json:"query" description:"The Cypher query string to execute." required:"true"`
}

// NewNeo4jTools creates the toolkit.
func NewNeo4jTools(opts ...Option) *Neo4jTools {
	n := &Neo4jTools{
		uri: os.Getenv("NEO4J_URI"), user: os.Getenv("NEO4J_USERNAME"), password: os.Getenv("NEO4J_PASSWORD"),
		database: "neo4j", enableLabels: true, enableRels: true, enableSchema: true, enableCypher: true,
	}
	if n.uri == "" {
		n.uri = "bolt://localhost:7687"
	}
	n.Toolkit = toolkit.NewToolkit()
	n.Toolkit.Name = "neo4j_tools"
	n.Toolkit.Description = "Query a Neo4j graph database: labels, relationship types, schema and Cypher."
	for _, o := range opts {
		o(n)
	}
	if n.httpClient == nil {
		n.httpClient = n.Toolkit.HTTPClient()
	}
	if n.enableLabels {
		n.Toolkit.Register("ListLabels", "Retrieve all node labels present in the connected Neo4j database.", n, n.ListLabels, NoParams{})
	}
	if n.enableRels {
		n.Toolkit.Register("ListRelationshipTypes", "Retrieve all relationship types present in the connected Neo4j database.", n, n.ListRelationshipTypes, NoParams{})
	}
	if n.enableSchema {
		n.Toolkit.Register("GetSchema", "Retrieve a visualization of the database schema, including nodes and relationships.", n, n.GetSchema, NoParams{})
	}
	if n.enableCypher {
		n.Toolkit.Register("RunCypherQuery", "Execute an arbitrary Cypher query against the connected Neo4j database.", n, n.RunCypherQuery, CypherParams{})
	}
	return n
}

// HTTPBaseURL returns the HTTP endpoint derived from the configured URI.
func (n *Neo4jTools) HTTPBaseURL() string {
	u, err := url.Parse(n.uri)
	if err != nil {
		return n.uri
	}
	if u.Scheme == "http" || u.Scheme == "https" {
		return strings.TrimRight(n.uri, "/")
	}
	scheme := "http"
	if strings.HasSuffix(u.Scheme, "+s") || strings.HasSuffix(u.Scheme, "+ssc") {
		scheme = "https"
	}
	return scheme + "://" + u.Hostname() + ":7474"
}

// Query runs a Cypher statement and returns rows as field->value maps.
func (n *Neo4jTools) Query(statement string) ([]map[string]interface{}, error) {
	c := restapi.New(n.HTTPBaseURL(), n.httpClient, nil)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		c.URL("/db/"+url.PathEscape(n.database)+"/query/v2", nil),
		strings.NewReader(restapi.ToJSON(map[string]string{"statement": statement})))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if n.user != "" || n.password != "" {
		req.SetBasicAuth(n.user, n.password)
	}
	data, err := c.Send(req)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data struct {
			Fields []string        `json:"fields"`
			Values [][]interface{} `json:"values"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	rows := make([]map[string]interface{}, 0, len(resp.Data.Values))
	for _, vals := range resp.Data.Values {
		row := make(map[string]interface{}, len(vals))
		for i, f := range resp.Data.Fields {
			if i < len(vals) {
				row[f] = vals[i]
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func (n *Neo4jTools) column(statement, field string) []interface{} {
	rows, err := n.Query(statement)
	out := []interface{}{}
	if err != nil {
		return out
	}
	for _, r := range rows {
		out = append(out, r[field])
	}
	return out
}

// ListLabels returns all node labels (empty list on error, as in Python).
func (n *Neo4jTools) ListLabels(NoParams) (interface{}, error) {
	return n.column("CALL db.labels()", "label"), nil
}

// ListRelationshipTypes returns all relationship types.
func (n *Neo4jTools) ListRelationshipTypes(NoParams) (interface{}, error) {
	return n.column("CALL db.relationshipTypes()", "relationshipType"), nil
}

// GetSchema returns db.schema.visualization() records.
func (n *Neo4jTools) GetSchema(NoParams) (interface{}, error) {
	rows, err := n.Query("CALL db.schema.visualization()")
	if err != nil {
		return []interface{}{}, nil
	}
	return rows, nil
}

// RunCypherQuery runs an arbitrary Cypher query.
func (n *Neo4jTools) RunCypherQuery(p CypherParams) (interface{}, error) {
	rows, err := n.Query(p.Query)
	if err != nil {
		return []interface{}{}, nil
	}
	return rows, nil
}
