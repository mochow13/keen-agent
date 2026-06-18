package theme

import (
	"strings"

	"github.com/charmbracelet/glamour/ansi"
)

const markdownMargin = 2

const (
	keenPrimaryLightColor = "#7986CB"

	atomOneDarkRed     = "#e06c75"
	atomOneDarkGreen   = "#98c379"
	atomOneDarkYellow  = "#e5c07b"
	atomOneDarkOrange  = "#d19a66"
	atomOneDarkBlue    = "#61afef"
	atomOneDarkPurple  = "#c678dd"
	atomOneDarkCyan    = "#56b6c2"
	atomOneDarkWhite   = "#abb2bf"
	atomOneDarkComment = "#5c6370"
)

// MarkdownStyleConfig keeps assistant markdown on the terminal's default
// foreground color while preserving markdown structure.
func MarkdownStyleConfig(wordWrap int) ansi.StyleConfig {
	return ansi.StyleConfig{
		Document: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				BlockPrefix: "\n",
				BlockSuffix: "\n",
			},
			Margin: uintPtr(markdownMargin),
		},
		BlockQuote: ansi.StyleBlock{
			Indent:      uintPtr(1),
			IndentToken: stringPtr("| "),
		},
		Paragraph: ansi.StyleBlock{},
		List: ansi.StyleList{
			LevelIndent: 4,
		},
		Heading: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				BlockSuffix: "\n",
				Bold:        boolPtr(true),
			},
		},
		H1: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "# ",
			},
		},
		H2: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "## ",
			},
		},
		H3: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "### ",
			},
		},
		H4: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "#### ",
			},
		},
		H5: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "##### ",
			},
		},
		H6: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "###### ",
			},
		},
		Strikethrough: ansi.StylePrimitive{
			CrossedOut: boolPtr(true),
		},
		Emph: ansi.StylePrimitive{
			Italic: boolPtr(true),
		},
		Strong: ansi.StylePrimitive{
			Bold: boolPtr(true),
		},
		HorizontalRule: ansi.StylePrimitive{
			Format: "\n" + strings.Repeat("─", markdownContentWidth(wordWrap)) + "\n",
		},
		Item: ansi.StylePrimitive{
			BlockPrefix: "• ",
		},
		Enumeration: ansi.StylePrimitive{
			BlockPrefix: ". ",
		},
		Task: ansi.StyleTask{
			Ticked:   "[x] ",
			Unticked: "[ ] ",
		},
		Link: ansi.StylePrimitive{
			Underline: boolPtr(true),
		},
		LinkText: ansi.StylePrimitive{
			Underline: boolPtr(true),
		},
		ImageText: ansi.StylePrimitive{
			Format: "Image: {{.text}} ->",
		},
		Code: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "`",
				Suffix: "`",
				Color:  stringPtr(keenPrimaryLightColor),
			},
		},
		CodeBlock: ansi.StyleCodeBlock{
			StyleBlock: ansi.StyleBlock{
				Margin: uintPtr(markdownMargin),
			},
			Chroma: markdownChromaStyle(),
		},
		Table: ansi.StyleTable{
			CenterSeparator: stringPtr("┼"),
			ColumnSeparator: stringPtr("│"),
			RowSeparator:    stringPtr("─"),
		},
		DefinitionDescription: ansi.StylePrimitive{
			BlockPrefix: "\n* ",
		},
	}
}

func markdownContentWidth(wordWrap int) int {
	width := wordWrap - markdownMargin*2
	if width < 1 {
		return 1
	}
	return width
}

func markdownChromaStyle() *ansi.Chroma {
	return &ansi.Chroma{
		Comment: ansi.StylePrimitive{
			Color:  stringPtr(atomOneDarkComment),
			Italic: boolPtr(true),
		},
		CommentPreproc: ansi.StylePrimitive{
			Color: stringPtr(atomOneDarkPurple),
		},
		Keyword: ansi.StylePrimitive{
			Color: stringPtr(atomOneDarkPurple),
			Bold:  boolPtr(true),
		},
		KeywordReserved: ansi.StylePrimitive{
			Color: stringPtr(atomOneDarkPurple),
			Bold:  boolPtr(true),
		},
		KeywordNamespace: ansi.StylePrimitive{
			Color: stringPtr(atomOneDarkPurple),
		},
		KeywordType: ansi.StylePrimitive{
			Color: stringPtr(atomOneDarkYellow),
		},
		Operator: ansi.StylePrimitive{
			Color: stringPtr(atomOneDarkCyan),
		},
		Punctuation: ansi.StylePrimitive{
			Color: stringPtr(atomOneDarkWhite),
		},
		NameBuiltin: ansi.StylePrimitive{
			Color: stringPtr(atomOneDarkCyan),
		},
		NameTag: ansi.StylePrimitive{
			Color: stringPtr(atomOneDarkRed),
		},
		NameAttribute: ansi.StylePrimitive{
			Color: stringPtr(atomOneDarkOrange),
		},
		NameClass: ansi.StylePrimitive{
			Color: stringPtr(atomOneDarkYellow),
			Bold:  boolPtr(true),
		},
		NameFunction: ansi.StylePrimitive{
			Color: stringPtr(atomOneDarkBlue),
			Bold:  boolPtr(true),
		},
		LiteralNumber: ansi.StylePrimitive{
			Color: stringPtr(atomOneDarkOrange),
		},
		LiteralString: ansi.StylePrimitive{
			Color: stringPtr(atomOneDarkGreen),
		},
		LiteralStringEscape: ansi.StylePrimitive{
			Color: stringPtr(atomOneDarkOrange),
		},
		GenericDeleted: ansi.StylePrimitive{
			Color: stringPtr(atomOneDarkRed),
		},
		GenericInserted: ansi.StylePrimitive{
			Color: stringPtr(atomOneDarkGreen),
		},
		GenericEmph: ansi.StylePrimitive{
			Italic: boolPtr(true),
		},
		GenericStrong: ansi.StylePrimitive{
			Bold: boolPtr(true),
		},
	}
}

func stringPtr(value string) *string {
	return &value
}

func boolPtr(value bool) *bool {
	return &value
}

func uintPtr(value uint) *uint {
	return &value
}
