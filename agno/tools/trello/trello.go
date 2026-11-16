// Package trello ports Agno's TrelloTools: boards, lists and cards through the
// Trello REST API (https://api.trello.com/1) authenticated with the key and
// token query parameters (what py-trello sends).
package trello

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/internal/restkit"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// DefaultBaseURL is the Trello API base URL.
const DefaultBaseURL = "https://api.trello.com/1"

// TrelloTools manages Trello boards, lists and cards.
type TrelloTools struct {
	toolkit.Toolkit
	apiKey     string
	apiSecret  string
	token      string
	baseURL    string
	httpClient *http.Client
	client     *restkit.Client
}

// Option configures TrelloTools.
type Option func(*TrelloTools)

// WithAPIKey sets the API key (default: TRELLO_API_KEY).
func WithAPIKey(key string) Option { return func(t *TrelloTools) { t.apiKey = key } }

// WithAPISecret sets the API secret (default: TRELLO_API_SECRET). Only kept for
// parity with Python: key+token are enough for the REST API.
func WithAPISecret(s string) Option { return func(t *TrelloTools) { t.apiSecret = s } }

// WithToken sets the user token (default: TRELLO_TOKEN).
func WithToken(token string) Option { return func(t *TrelloTools) { t.token = token } }

// WithBaseURL overrides the API base URL.
func WithBaseURL(u string) Option { return func(t *TrelloTools) { t.baseURL = u } }

// WithTimeout sets the HTTP timeout.
func WithTimeout(d time.Duration) Option { return func(t *TrelloTools) { t.Toolkit.Timeout = d } }

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(t *TrelloTools) { t.httpClient = c } }

// CreateCardParams are the parameters of CreateCard.
type CreateCardParams struct {
	BoardID     string `json:"board_id" description:"ID of the board to create the card in." required:"true"`
	ListName    string `json:"list_name" description:"Name of the list to add the card to." required:"true"`
	CardTitle   string `json:"card_title" description:"Title of the card." required:"true"`
	Description string `json:"description,omitempty" description:"Description of the card."`
}

// BoardParams identify a board.
type BoardParams struct {
	BoardID string `json:"board_id" description:"ID of the board." required:"true"`
}

// MoveCardParams are the parameters of MoveCard.
type MoveCardParams struct {
	CardID string `json:"card_id" description:"ID of the card to move." required:"true"`
	ListID string `json:"list_id" description:"ID of the destination list." required:"true"`
}

// ListParams identify a list.
type ListParams struct {
	ListID string `json:"list_id" description:"ID of the list." required:"true"`
}

// CreateBoardParams are the parameters of CreateBoard.
type CreateBoardParams struct {
	Name         string `json:"name" description:"Name of the board." required:"true"`
	DefaultLists bool   `json:"default_lists,omitempty" description:"Whether the default lists should be created. Default false."`
}

// CreateListParams are the parameters of CreateList.
type CreateListParams struct {
	BoardID  string `json:"board_id" description:"ID of the board to create the list in." required:"true"`
	ListName string `json:"list_name" description:"Name of the new list." required:"true"`
	Pos      string `json:"pos,omitempty" description:"Position of the list - 'top', 'bottom', or a positive number. Default 'bottom'."`
}

// ListBoardsParams are the parameters of ListBoards.
type ListBoardsParams struct {
	BoardFilter string `json:"board_filter,omitempty" description:"Filter for boards. Options: 'all', 'open', 'closed', 'organization', 'public', 'starred'. Defaults to 'all'."`
}

// New creates the Trello toolkit.
func New(opts ...Option) *TrelloTools {
	t := &TrelloTools{baseURL: DefaultBaseURL}
	t.Toolkit = toolkit.NewToolkit()
	t.Toolkit.Name = "trello"
	t.Toolkit.Description = "Manage Trello boards, lists and cards."
	for _, o := range opts {
		o(t)
	}
	t.apiKey = restkit.Env(t.apiKey, "TRELLO_API_KEY")
	t.apiSecret = restkit.Env(t.apiSecret, "TRELLO_API_SECRET")
	t.token = restkit.Env(t.token, "TRELLO_TOKEN")
	if t.httpClient == nil {
		t.httpClient = t.Toolkit.HTTPClient()
	}
	t.client = &restkit.Client{HTTP: t.httpClient, BaseURL: t.baseURL, Name: "Trello API"}
	t.Toolkit.Register("CreateCard", "Create a new card in the specified board and list.", t, t.CreateCard, CreateCardParams{})
	t.Toolkit.Register("GetBoardLists", "Get all lists on a board.", t, t.GetBoardLists, BoardParams{})
	t.Toolkit.Register("MoveCard", "Move a card to a different list.", t, t.MoveCard, MoveCardParams{})
	t.Toolkit.Register("GetCards", "Get all cards in a list.", t, t.GetCards, ListParams{})
	t.Toolkit.Register("CreateBoard", "Create a new Trello board.", t, t.CreateBoard, CreateBoardParams{})
	t.Toolkit.Register("CreateList", "Create a new list on a specified board.", t, t.CreateList, CreateListParams{})
	t.Toolkit.Register("ListBoards", "Get a list of all boards for the authenticated user.", t, t.ListBoards, ListBoardsParams{})
	return t
}

func (t *TrelloTools) call(method, path string, q url.Values, out interface{}) error {
	if t.apiKey == "" || t.token == "" {
		return fmt.Errorf("missing Trello credentials: set TRELLO_API_KEY and TRELLO_TOKEN")
	}
	if q == nil {
		q = url.Values{}
	}
	q.Set("key", t.apiKey)
	q.Set("token", t.token)
	return t.client.Into(method, path, q, nil, out)
}

type card struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Desc   string `json:"desc"`
	URL    string `json:"url"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
}

type list struct {
	ID    string      `json:"id"`
	Name  string      `json:"name"`
	Pos   interface{} `json:"pos"`
	Cards []card      `json:"cards"`
}

// CreateCard creates a card in the list named list_name (case-insensitive).
func (t *TrelloTools) CreateCard(p CreateCardParams) (interface{}, error) {
	var lists []list
	if err := t.call(http.MethodGet, "/boards/"+url.PathEscape(p.BoardID)+"/lists", url.Values{"filter": {"all"}}, &lists); err != nil {
		return fmt.Sprintf("Error creating card: %v", err), nil
	}
	var target *list
	for i := range lists {
		if strings.EqualFold(lists[i].Name, p.ListName) {
			target = &lists[i]
			break
		}
	}
	if target == nil {
		return fmt.Sprintf("List '%s' not found on board", p.ListName), nil
	}
	var c card
	q := url.Values{"idList": {target.ID}, "name": {p.CardTitle}, "desc": {p.Description}}
	if err := t.call(http.MethodPost, "/cards", q, &c); err != nil {
		return fmt.Sprintf("Error creating card: %v", err), nil
	}
	return restkit.CompactJSON(map[string]interface{}{"id": c.ID, "name": c.Name, "url": c.URL, "list": p.ListName}), nil
}

// GetBoardLists returns {"lists": [{id, name, cards_count}]}.
func (t *TrelloTools) GetBoardLists(p BoardParams) (interface{}, error) {
	var lists []list
	q := url.Values{"filter": {"all"}, "cards": {"open"}}
	if err := t.call(http.MethodGet, "/boards/"+url.PathEscape(p.BoardID)+"/lists", q, &lists); err != nil {
		return fmt.Sprintf("Error getting board lists: %v", err), nil
	}
	info := []map[string]interface{}{}
	for _, l := range lists {
		info = append(info, map[string]interface{}{"id": l.ID, "name": l.Name, "cards_count": len(l.Cards)})
	}
	return restkit.CompactJSON(map[string]interface{}{"lists": info}), nil
}

// MoveCard moves a card to another list.
func (t *TrelloTools) MoveCard(p MoveCardParams) (interface{}, error) {
	var c card
	if err := t.call(http.MethodPut, "/cards/"+url.PathEscape(p.CardID), url.Values{"idList": {p.ListID}}, &c); err != nil {
		return fmt.Sprintf("Error moving card: %v", err), nil
	}
	return restkit.CompactJSON(map[string]interface{}{"success": true, "card_id": p.CardID, "new_list_id": p.ListID}), nil
}

// GetCards returns {"cards": [...]} for the open cards of a list.
func (t *TrelloTools) GetCards(p ListParams) (interface{}, error) {
	var cards []card
	if err := t.call(http.MethodGet, "/lists/"+url.PathEscape(p.ListID)+"/cards", url.Values{"filter": {"open"}}, &cards); err != nil {
		return fmt.Sprintf("Error getting cards: %v", err), nil
	}
	info := []map[string]interface{}{}
	for _, c := range cards {
		labels := []string{}
		for _, l := range c.Labels {
			labels = append(labels, l.Name)
		}
		info = append(info, map[string]interface{}{"id": c.ID, "name": c.Name, "description": c.Desc, "url": c.URL, "labels": labels})
	}
	return restkit.CompactJSON(map[string]interface{}{"cards": info}), nil
}

// CreateBoard creates a board.
func (t *TrelloTools) CreateBoard(p CreateBoardParams) (interface{}, error) {
	var b struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		URL  string `json:"url"`
	}
	q := url.Values{"name": {p.Name}, "defaultLists": {strconv.FormatBool(p.DefaultLists)}}
	if err := t.call(http.MethodPost, "/boards", q, &b); err != nil {
		return fmt.Sprintf("Error creating board: %v", err), nil
	}
	return restkit.CompactJSON(map[string]interface{}{"id": b.ID, "name": b.Name, "url": b.URL}), nil
}

// CreateList creates a list on a board.
func (t *TrelloTools) CreateList(p CreateListParams) (interface{}, error) {
	pos := p.Pos
	if pos == "" {
		pos = "bottom"
	}
	var l list
	q := url.Values{"name": {p.ListName}, "idBoard": {p.BoardID}, "pos": {pos}}
	if err := t.call(http.MethodPost, "/lists", q, &l); err != nil {
		return fmt.Sprintf("Error creating list: %v", err), nil
	}
	return restkit.CompactJSON(map[string]interface{}{"id": l.ID, "name": l.Name, "pos": l.Pos, "board_id": p.BoardID}), nil
}

// ListBoards lists the boards of the authenticated member.
func (t *TrelloTools) ListBoards(p ListBoardsParams) (interface{}, error) {
	filter := p.BoardFilter
	if filter == "" {
		filter = "all"
	}
	var boards []struct {
		ID             string      `json:"id"`
		Name           string      `json:"name"`
		Desc           string      `json:"desc"`
		URL            string      `json:"url"`
		Closed         bool        `json:"closed"`
		Starred        bool        `json:"starred"`
		IDOrganization interface{} `json:"idOrganization"`
	}
	if err := t.call(http.MethodGet, "/members/me/boards", url.Values{"filter": {filter}}, &boards); err != nil {
		return fmt.Sprintf("Error listing boards: %v", err), nil
	}
	list := []map[string]interface{}{}
	for _, b := range boards {
		list = append(list, map[string]interface{}{
			"id": b.ID, "name": b.Name, "description": b.Desc, "url": b.URL,
			"closed": b.Closed, "starred": b.Starred, "organization": b.IDOrganization,
		})
	}
	return restkit.CompactJSON(map[string]interface{}{"filter_used": filter, "total_boards": len(list), "boards": list}), nil
}

// Execute implements toolkit.Tool.
func (t *TrelloTools) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return t.Toolkit.Execute(methodName, input)
}
