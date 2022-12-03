%{
package filter

import (
	"fmt"
)

// End Of Filter
const EOF = 0

func unexpectedTokenErr(s string) error {
	return fmt.Errorf("unexpected token '%s'", s)
}

func couldNotParseErr(err error) error {
	return fmt.Errorf("could not parse expression: %w", err)
}
%}

%union {
	s string
	labels map[string]string
	expression Expression
}

%token NOT AND OR

%right NOT
%right AND
%right OR

%token<s> RESOURCE PATTERN NAMESPACE LABEL

%type<expression> expr
%type<labels> labels

%%

start:         { yylex.(*Lexer).result = truthyExpression{} }
	| expr { yylex.(*Lexer).result = $1 }
	;

expr:   RESOURCE PATTERN {
		namespacePattern, namePattern := splitPattern($2)
		if err := validateNamespace(namespacePattern); err != nil {
			yylex.Error(couldNotParseErr(err).Error())
		}

		if err := validateResourceName($1, namePattern); err != nil {
			yylex.Error(couldNotParseErr(err).Error())
		}

		switch $1 {
		case "pod":
			$$ = podExpression { namePattern: namePattern, namespacePattern: namespacePattern }
		case "job":
			$$ = jobExpression { namePattern: namePattern, namespacePattern: namespacePattern }
		case "replicaset":
			$$ = replicasetExpression { namePattern: namePattern, namespacePattern: namespacePattern }
		case "deployment":
			$$ = deploymentExpression { namePattern: namePattern, namespacePattern: namespacePattern }
		case "service":
			$$ = serviceExpression { namePattern: namePattern, namespacePattern: namespacePattern }
		}
	}
	| NAMESPACE PATTERN {
		if err := validateNamespace($2); err != nil {
			yylex.Error(couldNotParseErr(err).Error())
		}

		$$ = namespaceExpression{ namespacePattern: $2 }
	}
	| LABEL labels { $$ = labelExpression{ labelPatterns: $2 } }
	| '(' expr ')' { $$ = $2	 }
	| expr AND expr { $$ = andExpression { left: $1, right: $3 } }
	| expr OR expr { $$ = orExpression { left: $1, right: $3 } }
	| NOT expr { $$ = notExpression { inner: $2 } }
	;

labels: PATTERN {
		key, val, err := splitLabelPattern($1)

		if err != nil {
			yylex.Error(fmt.Sprintf("could not parse label pattern '%s': %s", $1, err))
		}

		$$ = map[string]string { key: val }
	}
	| labels PATTERN {
		key, val, err := splitLabelPattern($2)

		if err != nil {
			yylex.Error(fmt.Sprintf("could not parse label pattern '%s': %s", $1, err))
		}

		$$[key] = val
	}
	;

%%
