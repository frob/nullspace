package response

import "context"

// TextFormatter serializes response data as structured plain text.
// Fields appear as "Key: Value" lines, followed by a blank line and
// body content when present.
type TextFormatter struct{}

func (f *TextFormatter) Name() string        { return "text" }
func (f *TextFormatter) ContentType() string { return "text/plain; charset=utf-8" }

func (f *TextFormatter) Format(_ context.Context, resp *Response) ([]byte, error) {
	r := renderText(resp.Data)
	return renderPlain(r), nil
}
