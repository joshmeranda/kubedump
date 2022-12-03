package filter

//go:generate goyacc -o yyparser.go parser.y

func Parse(s string) (Expression, error) {
	lexer := NewLexer(s)

	yyParse(&lexer)

	if lexer.err != nil {
		return nil, lexer.err
	}

	return lexer.result, nil
}
