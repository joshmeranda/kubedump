package filter

//go:generate go run ../codegen parser

func init() {
	yyErrorVerbose = true
}

func Parse(s string) (Expression, error) {
	lexer := NewLexer(s)

	yyParse(&lexer)

	if lexer.err != nil {
		return nil, lexer.err
	}

	return lexer.result, nil
}
