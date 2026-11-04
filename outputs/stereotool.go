package outputs

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"zwfm-metadata/config"
	"zwfm-metadata/core"
	"zwfm-metadata/utils"
)

// StereoToolOutput sends RDS and streaming metadata to Stereo Tool.
type StereoToolOutput struct {
	*core.OutputBase
	core.PassiveComponent
	settings config.StereoToolOutputConfig
}

// NewStereoToolOutput creates a Stereo Tool metadata output.
func NewStereoToolOutput(name string, settings config.StereoToolOutputConfig) *StereoToolOutput {
	return &StereoToolOutput{OutputBase: core.NewOutputBase(name), settings: settings}
}

// Send updates Stereo Tool's RDS and streaming metadata fields.
func (i *StereoToolOutput) Send(st *core.StructuredText) error {
	return i.sendToStereoTool(st.String())
}

// Stereo Tool 10.75 and 11.05 use these metadata field IDs.
var stereoToolFields = []struct {
	id     int
	name   string
	format func(string) string
	// optional fields may not exist: the generic plugin build has no Song field.
	optional bool
}{
	{id: 6751, name: "Streaming Output Song", format: func(s string) string { return s }, optional: true},
	{id: 9985, name: "FM RDS Radio Text", format: rdsTextEscaper.Replace},
}

// rdsTextEscaper escapes what Stereo Tool reads as RDS text formatting: "/" separates
// texts, ":" ends a display time, "<", ">" and "|" scroll and wrap, "\\" starts a command.
var rdsTextEscaper = strings.NewReplacer(`\`, `\\`, `/`, `\/`, `:`, `\:`, `<`, `\<`, `>`, `\>`, `|`, `\|`)

var errStereoToolUnknownField = errors.New("stereo tool does not know this field")

// sendToStereoTool updates the fields concurrently because they are independent;
// a failing field does not stop the others.
func (i *StereoToolOutput) sendToStereoTool(metadata string) error {
	errs := make([]error, len(stereoToolFields))
	var wg sync.WaitGroup
	for n, field := range stereoToolFields {
		wg.Go(func() {
			err := i.updateField(field.id, field.name, field.format(metadata))
			if field.optional && errors.Is(err, errStereoToolUnknownField) {
				slog.Debug("Skipped Stereo Tool field", "output", i.GetName(), "field", field.name)
				return
			}
			errs[n] = err
		})
	}
	wg.Wait()
	return errors.Join(errs...)
}

// stereoToolQuote quotes a value for Stereo Tool's JSON reader, which decodes no escapes
// and ends a value at its quote character. It accepts single quotes too, so a double
// quote survives unless the value also holds an apostrophe; then it becomes one.
func stereoToolQuote(value string) string {
	if !strings.Contains(value, `"`) {
		return `"` + value + `"`
	}
	if !strings.Contains(value, `'`) {
		return `'` + value + `'`
	}
	return `"` + strings.ReplaceAll(value, `"`, `'`) + `"`
}

func (i *StereoToolOutput) updateField(id int, fieldName, value string) error {
	payload := `{"` + strconv.Itoa(id) + `":{"forced":"1","new_value":` + stereoToolQuote(value) + `}}`

	escapedPayload := strings.ReplaceAll(url.QueryEscape(payload), "+", "%20")
	host := joinHostPort(i.settings.Hostname, i.settings.Port)
	requestURL := "http://" + host + "/json-1/lis" + escapedPayload

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, requestURL, http.NoBody)
	if err != nil {
		return fmt.Errorf("failed to create request for %s: %w", fieldName, err)
	}

	body, err := utils.DoOKBody(req)
	if err != nil {
		return fmt.Errorf("failed to update %s: %w", fieldName, err)
	}
	// Stereo Tool answers a refused update with 200 too, and only a stored value has value_text.
	if !bytes.Contains(body, []byte(`"value_text"`)) {
		if bytes.Contains(body, []byte("ERROR_UNKNOWN_ID")) {
			return fmt.Errorf("failed to update %s: %w", fieldName, errStereoToolUnknownField)
		}
		return fmt.Errorf("failed to update %s: %s", fieldName, body)
	}

	slog.Debug("Updated Stereo Tool field", "output", i.GetName(), "field", fieldName, "value", value)
	return nil
}
