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

%type<expression> expr single_expr
%type<labels> labels

%%

start:         { yylex.(*Lexer).result = truthyExpression{} }
	| expr { yylex.(*Lexer).result = $1 }
	;

expr: single_expr
	| '(' expr ')'     { $$ = $2 }
	| expr AND expr    { $$ = andExpression { left: $1, right: $3 } }
	| expr OR expr     { $$ = orExpression { left: $1, right: $3 } }
	| NOT single_expr  { $$ = notExpression { inner: $2 } }
	| NOT '(' expr ')' { $$ = notExpression { inner: $3 } }
	;

single_expr: RESOURCE PATTERN {
		namespacePattern, namePattern := splitPattern($2)
		if err := validateNamespace(namespacePattern); err != nil {
			yylex.Error(couldNotParseErr(err).Error())
		}

		if err := validateResourceName($1, namePattern); err != nil {
			yylex.Error(couldNotParseErr(err).Error())
		}

		switch $1 {
		case "pod":
			$$ = resourceExpression { kind: "Pod", namePattern: namePattern, namespacePattern: namespacePattern }
		case "job":
			$$ = resourceExpression { kind: "Job", namePattern: namePattern, namespacePattern: namespacePattern }
		case "replicaset":
			$$ = resourceExpression { kind: "ReplicaSet", namePattern: namePattern, namespacePattern: namespacePattern }
		case "deployment":
			$$ = resourceExpression { kind: "Deployment", namePattern: namePattern, namespacePattern: namespacePattern }
		case "service":
			$$ = resourceExpression { kind: "Service", namePattern: namePattern, namespacePattern: namespacePattern }
		case "configmap":
			$$ = resourceExpression { kind: "ConfigMap", namePattern: namePattern, namespacePattern: namespacePattern }
		}
	}
	| NAMESPACE PATTERN   {
		if err := validateNamespace($2); err != nil {
			yylex.Error(couldNotParseErr(err).Error())
		}

		$$ = namespaceExpression{ namespacePattern: $2 }
	}
	| LABEL labels        { $$ = labelExpression{ labels: $2 } }

labels: PATTERN          {
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
