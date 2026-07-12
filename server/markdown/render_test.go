package markdown

import (
	"strings"
	"testing"
)

func TestRenderMarkdown(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string // substrings that must appear
	}{
		{
			name:  "unordered list",
			input: "- item1\n- item2\n- item3",
			want:  []string{"<ul>", "<li>item1</li>", "<li>item2</li>", "<li>item3</li>", "</ul>"},
		},
		{
			name:  "ordered list",
			input: "1. first\n2. second\n3. third",
			want:  []string{"<ol>", "<li>first</li>", "<li>second</li>", "<li>third</li>", "</ol>"},
		},
		{
			name:  "bold and italic",
			input: "**bold** and *italic* text",
			want:  []string{"<strong>bold</strong>", "<em>italic</em>"},
		},
		{
			name:  "strikethrough",
			input: "~~deleted~~ text",
			want:  []string{"<del>deleted</del>"},
		},
		{
			name:  "link",
			input: "[click here](https://example.com)",
			// bluemonday adds rel=nofollow and target=_blank
			want: []string{`<a href="https://example.com"`, `click here</a>`},
		},
		{
			name:  "image",
			input: "![alt](https://example.com/img.png)",
			want:  []string{`<img`, `src="https://example.com/img.png"`, `alt="alt"`},
		},
		{
			name:  "code block",
			input: "```go\nfunc main() {}\n```",
			want:  []string{"<pre", "language-go", "func main()", "</code>", "</pre>"},
		},
		{
			name:  "heading",
			input: "# Title\n## Section",
			want:  []string{"<h1>Title</h1>", "<h2>Section</h2>"},
		},
		{
			name:  "blockquote",
			input: "> quoted text",
			want:  []string{"<blockquote>", "quoted text", "</blockquote>"},
		},
		{
			name:  "horizontal rule",
			input: "before\n\n---\n\nafter",
			want:  []string{"<hr"},
		},
		{
			name:  "kaTeX preserved",
			input: "text $$x^2$$ more",
			want:  []string{"$$x^2$$"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RenderMarkdown(tt.input)
			for _, w := range tt.want {
				if !strings.Contains(got, w) {
					t.Errorf("RenderMarkdown(%q)\n  missing %q\n  got: %q", tt.input, w, got)
				}
			}
		})
	}
}
