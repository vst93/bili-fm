package markdown

import (
	"reflect"
	"testing"
)

func TestHTML(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"", ""},
		{"Hello\nworld", "<p>Hello world</p>"},
		{"One\n\nTwo", "<p>One</p><p>Two</p>"},
		{"### Added\n\n- One\n- Two", "<h3>Added</h3><ul><li>One</li><li>Two</li></ul>"},
		{"## 1.2.0 ##", "<h2>1.2.0</h2>"},
		{"#hashtag", "<p>#hashtag</p>"},
		{"Fixes:\n- one\n- two", "<p>Fixes:</p><ul><li>one</li><li>two</li></ul>"},
		{"1. one\n2. two", "<ol><li>one</li><li>two</li></ol>"},
		{"We shipped\n2. things", "<p>We shipped 2. things</p>"},
		{"- a\n  - b\n  - c\n- d", "<ul><li>a<ul><li>b</li><li>c</li></ul></li><li>d</li></ul>"},
		{"1. a\n  - b", "<ol><li>a<ul><li>b</li></ul></li></ol>"},
		{"- a long\nitem\n- b", "<ul><li>a long item</li><li>b</li></ul>"},
		{"- a\n\n  more\n- b", "<ul><li>a<br>more</li><li>b</li></ul>"},
		{"- a\n\nAfter", "<ul><li>a</li></ul><p>After</p>"},
		{"- a\n* b", "<ul><li>a</li><li>b</li></ul>"},
		{"- a\n1. b", "<ul><li>a</li></ul><ol><li>b</li></ol>"},
		{"> quoted\n> text", "<blockquote><p>quoted text</p></blockquote>"},
		{"---", "<hr>"},
		{"- - -", "<hr>"},
		{"```\n<b>code</b>\n\n  x\n```\nafter", "<pre><code>&lt;b&gt;code&lt;/b&gt;\n\n  x</code></pre><p>after</p>"},
		{"**bold** and *em* and _em_ and __bold__", "<p><strong>bold</strong> and <em>em</em> and <em>em</em> and <strong>bold</strong></p>"},
		{"snake_case_name and 2 * 3 * 4", "<p>snake_case_name and 2 * 3 * 4</p>"},
		{"**bold *em* bold**", "<p><strong>bold <em>em</em> bold</strong></p>"},
		{"`a *b* <c>` and ``x ` y``", "<p><code>a *b* &lt;c&gt;</code> and <code>x ` y</code></p>"},
		{`\*not em\*`, "<p>*not em*</p>"},
		{"[docs](https://example.com/a?b=1&c=2 \"Title\")", `<p><a href="https://example.com/a?b=1&amp;c=2">docs</a></p>`},
		{"[**bold** link](https://example.com)", `<p><a href="https://example.com"><strong>bold</strong> link</a></p>`},
		{"[bad](javascript:alert(1))", "<p>bad)</p>"},
		{"[rel](/path)", "<p>rel</p>"},
		{"![logo](https://example.com/logo.png)", `<p><a href="https://example.com/logo.png">logo</a></p>`},
		{"<https://example.com> <b>hi</b>", `<p><a href="https://example.com">https://example.com</a> &lt;b&gt;hi&lt;/b&gt;</p>`},
		{"See https://example.com/x_(y). Or (https://example.com).", `<p>See <a href="https://example.com/x_(y)">https://example.com/x_(y)</a>. Or (<a href="https://example.com">https://example.com</a>).</p>`},
		{"<script>alert(1)</script>", "<p>&lt;script&gt;alert(1)&lt;/script&gt;</p>"},
		{`<img src=x onerror="alert(1)">`, "<p>&lt;img src=x onerror=&#34;alert(1)&#34;&gt;</p>"},
		{"[x](https://a\"onmouseover=\"alert(1))", `<p><a href="https://a&#34;onmouseover=&#34;alert(1">x</a>)</p>`},
		{"Café — ünïcode", "<p>Café — ünïcode</p>"},
		{"a\r\nb", "<p>a b</p>"},
		{"**unclosed", "<p>**unclosed</p>"},
		{"[unclosed](https://x", "<p>[unclosed](<a href=\"https://x\">https://x</a></p>"},
	} {
		if got := HTML(Parse(tc.in)); got != tc.want {
			t.Errorf("HTML(Parse(%q))\n got %s\nwant %s", tc.in, got, tc.want)
		}
	}
}

func TestParse(t *testing.T) {
	got := Parse("## Fixed\n\n- A **bold [link](https://example.com)** fix\n  1. nested\n\n> `code`")
	want := []Block{
		{Kind: Heading, Level: 2, Inlines: []Inline{{Kind: Text, Text: "Fixed"}}},
		{Kind: List, Items: [][]Block{{
			{Kind: Paragraph, Inlines: []Inline{
				{Kind: Text, Text: "A "},
				{Kind: Strong, Children: []Inline{
					{Kind: Text, Text: "bold "},
					{Kind: Link, URL: "https://example.com", Children: []Inline{{Kind: Text, Text: "link"}}},
				}},
				{Kind: Text, Text: " fix"},
			}},
			{Kind: List, Ordered: true, Items: [][]Block{{{Kind: Paragraph, Inlines: []Inline{{Kind: Text, Text: "nested"}}}}}},
		}}},
		{Kind: Quote, Blocks: []Block{{Kind: Paragraph, Inlines: []Inline{{Kind: CodeSpan, Text: "code"}}}}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Parse\n got %+v\nwant %+v", got, want)
	}
	if blocks := Parse(" \n\n"); len(blocks) != 0 {
		t.Errorf("Parse of blank notes = %+v, want none", blocks)
	}
}
