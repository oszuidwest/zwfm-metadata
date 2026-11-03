package outputs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"text/template"
	"text/template/parse"

	"zwfm-metadata/config"
	"zwfm-metadata/core"
	"zwfm-metadata/utils"
)

// urlEscapeFunc is the template function that parseURLTemplate adds to each action.
const urlEscapeFunc = "urlEscape"

// URLOutput sends metadata via configurable HTTP GET or POST requests.
type URLOutput struct {
	*core.OutputBase
	core.PassiveComponent
	settings      config.URLOutputConfig
	payloadMapper *PayloadMapper
	pathTemplate  *template.Template
	queryTemplate *template.Template
}

// NewURLOutput creates a URLOutput with the given name and settings.
func NewURLOutput(name string, settings config.URLOutputConfig) (*URLOutput, error) {
	mapper, err := NewPayloadMapper(settings.PayloadMapping)
	if err != nil {
		return nil, err
	}

	settings.Method = strings.ToUpper(settings.Method)
	if settings.Method != http.MethodGet && settings.Method != http.MethodPost {
		return nil, fmt.Errorf("method must be GET or POST, got %q", settings.Method)
	}

	if err := utils.ValidateHTTPURL(settings.URL); err != nil {
		return nil, err
	}

	var pathTmpl, queryTmpl *template.Template
	if isTemplate(settings.URL) {
		path, query, hasQuery := strings.Cut(settings.URL, "?")
		pathTmpl, err = parseURLTemplate("url path", path, url.PathEscape)
		if err != nil {
			return nil, err
		}
		if hasQuery {
			queryTmpl, err = parseURLTemplate("url query", query, url.QueryEscape)
			if err != nil {
				return nil, err
			}
		}
	}

	return &URLOutput{
		OutputBase:    core.NewOutputBase(name),
		settings:      settings,
		payloadMapper: mapper,
		pathTemplate:  pathTmpl,
		queryTemplate: queryTmpl,
	}, nil
}

// Send sends metadata via the configured HTTP method.
func (u *URLOutput) Send(st *core.StructuredText) error {
	payload := ConvertStructuredText(st)
	if u.settings.Method == http.MethodGet {
		return u.sendGETRequest(payload)
	}
	return u.sendPOSTRequest(payload)
}

// parseURLTemplate parses text so that every printed value is escaped last, after
// the template functions have run on the raw metadata.
func parseURLTemplate(name, text string, escape func(string) string) (*template.Template, error) {
	tmpl, err := template.New(name).Funcs(templateFuncs).Funcs(template.FuncMap{
		urlEscapeFunc: func(value any) string { return escape(fmt.Sprint(value)) },
	}).Parse(text)
	if err != nil {
		return nil, fmt.Errorf("invalid URL template: %w", err)
	}
	for _, t := range tmpl.Templates() {
		appendURLEscape(t.Root)
	}
	return tmpl, nil
}

// appendURLEscape adds the escape function to the pipeline of every action that
// prints a value, like html/template does. Variable declarations print nothing.
func appendURLEscape(node parse.Node) {
	switch n := node.(type) {
	case *parse.ListNode:
		if n != nil {
			for _, child := range n.Nodes {
				appendURLEscape(child)
			}
		}
	case *parse.ActionNode:
		if len(n.Pipe.Decl) == 0 {
			n.Pipe.Cmds = append(n.Pipe.Cmds, &parse.CommandNode{
				NodeType: parse.NodeCommand,
				Args:     []parse.Node{parse.NewIdentifier(urlEscapeFunc)},
			})
		}
	case *parse.IfNode:
		appendURLEscape(n.List)
		appendURLEscape(n.ElseList)
	case *parse.RangeNode:
		appendURLEscape(n.List)
		appendURLEscape(n.ElseList)
	case *parse.WithNode:
		appendURLEscape(n.List)
		appendURLEscape(n.ElseList)
	}
}

func (u *URLOutput) sendGETRequest(payload *UniversalMetadata) error {
	requestURL := u.settings.URL

	if u.pathTemplate != nil {
		data := payload.ToTemplateData()
		var b strings.Builder
		if err := u.pathTemplate.Execute(&b, data); err != nil {
			return fmt.Errorf("execute URL template: %w", err)
		}
		if u.queryTemplate != nil {
			b.WriteByte('?')
			if err := u.queryTemplate.Execute(&b, data); err != nil {
				return fmt.Errorf("execute URL template: %w", err)
			}
		}
		requestURL = b.String()
	}

	slog.Debug("Sending GET request",
		"output", u.GetName(),
		"url", requestURL,
	)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, requestURL, http.NoBody)
	if err != nil {
		return fmt.Errorf("create GET request: %w", err)
	}

	return u.doRequest(req)
}

func (u *URLOutput) sendPOSTRequest(payload *UniversalMetadata) error {
	if u.payloadMapper != nil {
		payload.Type = "url"
	}

	jsonData, err := json.Marshal(u.payloadMapper.Apply(payload))
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	slog.Debug("Sending POST request",
		"output", u.GetName(),
		"url", u.settings.URL,
		"payload", string(jsonData),
	)

	req, err := http.NewRequestWithContext(
		context.Background(), http.MethodPost, u.settings.URL, bytes.NewReader(jsonData),
	)
	if err != nil {
		return fmt.Errorf("create POST request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	return u.doRequest(req)
}

// doRequest sets the configured auth header, executes the request, and logs the outcome.
func (u *URLOutput) doRequest(req *http.Request) error {
	if u.settings.BearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+u.settings.BearerToken)
	}

	if err := utils.DoOK(req); err != nil {
		return fmt.Errorf("send %s request: %w", req.Method, err)
	}

	slog.Debug("Successfully sent request",
		"output", u.GetName(),
		"method", req.Method,
		"url", req.URL.String(),
	)
	return nil
}
