// Mugo: compiler for a (micro) subset of Go

package main

// Run with this command:
//
// go run . <examples/test.go

// types: T = byte int string []byte
// - maybe? []T [n]T map[string]T struct *ptr bool
// builtins: append cap copy len print println

// parse and compile at once - can we pull this off?
// - write to []byte and patch jumps?
// - what about things used before they're defined?

var (
	c    int
	line int = 1
	col  int = 0

	token    int
	intToken int
	strToken string
)

var (
	tEOF int = -1

	// Keywords
	tIf      int = 1
	tElse    int = 2
	tFor     int = 3
	tVar     int = 4
	tFunc    int = 5
	tReturn  int = 6
	tPackage int = 7

	// Literals and identifiers
	tIntLit int = 8
	tStrLit int = 9
	tIdent  int = 10

	// Two-character tokens
	tOr         int = 11
	tAnd        int = 12
	tEq         int = 13
	tNotEq      int = 14
	tLessEq     int = 15
	tGreaterEq  int = 16
	tDeclAssign int = 17

	// Single-character tokens (these use the ASCII value)
	tPlus      int = '+'
	tMinus     int = '-'
	tTimes     int = '*'
	tDivide    int = '/'
	tModulo    int = '%'
	tComma     int = ','
	tSemicolon int = ';'
	tAssign    int = '='
	tNot       int = '!'
	tLess      int = '<'
	tGreater   int = '>'
	tLParen    int = '('
	tRParen    int = ')'
	tLBrace    int = '{'
	tRBrace    int = '}'
	tLBracket  int = '['
	tRBracket  int = ']'
)

func nextChar() {
	c = readByte()
	col = col + 1
	if c == '\n' {
		line = line + 1
		col = 0
	}
}

func error(msg string) {
	printError("\n" + intStr(line) + ":" + intStr(col) + ": " + msg + "\n")
	exit(1)
}

func isDigit(ch int) bool {
	return ch >= '0' && ch <= '9'
}

func isAlpha(ch int) bool {
	return ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z'
}

func expectChar(ch int) {
	if c != ch {
		error("expected '" + charStr(ch) + "' not '" + charStr(c) + "'")
	}
	nextChar()
}

func nextToken() {
	nextTokenInner()
	//	print("TOKEN: " + tokenStr(token) + "\n")
}

func skipWhitespace() int {
	for c == ' ' || c == '\t' || c == '\r' || c == '\n' {
		if c == '\n' {
			// Semicolon insertion: https://golang.org/ref/spec#Semicolons
			if token == tIdent || token == tIntLit || token == tStrLit ||
				token == tReturn || token == tRParen || token == tRBracket ||
				token == tRBrace {
				nextChar()
				return tSemicolon
			}
		}
		nextChar()
	}
	return 0
}

func nextTokenInner() {
	t := skipWhitespace()
	if t != 0 {
		token = t
		return
	}

	if c < 0 {
		// End of file
		token = tEOF
		return
	}

	// Skip comments (and detect the '/' operator)
	for c == '/' {
		nextChar()
		if c != '/' {
			token = tDivide
			return
		}
		nextChar()
		// Comment, skip till end of line
		for c >= 0 && c != '\n' {
			nextChar()
		}
		t := skipWhitespace()
		if t != 0 {
			token = t
			return
		}
	}

	// Integer literal
	if isDigit(c) {
		intToken = c - '0'
		nextChar()
		for isDigit(c) {
			intToken = intToken*10 + c - '0'
			nextChar()
		}
		token = tIntLit
		return
	}

	// Character literal
	if c == '\'' {
		nextChar()
		if c == '\n' {
			error("newline not allowed in character literal")
		}
		if c == '\\' {
			// Escape character
			nextChar()
			if c == '\'' {
				intToken = '\''
			} else if c == '\\' {
				intToken = '\\'
			} else if c == 't' {
				intToken = '\t'
			} else if c == 'r' {
				intToken = '\r'
			} else if c == 'n' {
				intToken = '\n'
			} else {
				error("unexpected escape '\\" + charStr(c) + "'")
			}
			nextChar()
		} else {
			intToken = c
			nextChar()
		}
		expectChar('\'')
		token = tIntLit
		return
	}

	// String literal
	if c == '"' {
		nextChar()
		strToken = ""
		for c >= 0 && c != '"' {
			if c == '\n' {
				error("newline not allowed in string")
			}
			if c == '\\' {
				// Escape character
				nextChar()
				if c == '"' {
					c = '"'
				} else if c == '\\' {
					c = '\\'
				} else if c == 't' {
					c = '\t'
				} else if c == 'r' {
					c = '\r'
				} else if c == 'n' {
					c = '\n'
				} else {
					error("unexpected escape \"\\" + charStr(c) + "\"")
				}
			}
			// TODO: not great to build string via concatenation
			strToken = strToken + charStr(c)
			nextChar()
		}
		expectChar('"')
		token = tStrLit
		return
	}

	// Keyword or identifier
	if isAlpha(c) || c == '_' {
		strToken = charStr(c)
		nextChar()
		for isAlpha(c) || isDigit(c) || c == '_' {
			// TODO: not great to build string via concatenation
			strToken = strToken + charStr(c)
			nextChar()
		}
		// Check for keywords
		if strToken == "if" {
			token = tIf
		} else if strToken == "else" {
			token = tElse
		} else if strToken == "for" {
			token = tFor
		} else if strToken == "var" {
			token = tVar
		} else if strToken == "func" {
			token = tFunc
		} else if strToken == "return" {
			token = tReturn
		} else if strToken == "package" {
			token = tPackage
		} else {
			// Otherwise it's an identifier
			token = tIdent
		}
		return
	}

	// Single-character tokens (token is ASCII value)
	if c == '+' || c == '-' || c == '*' || c == '%' || c == ';' || c == ',' ||
		c == '(' || c == ')' || c == '{' || c == '}' || c == '[' || c == ']' {
		token = c
		nextChar()
		return
	}

	// One or two-character tokens
	if c == '=' {
		nextChar()
		if c == '=' {
			nextChar()
			token = tEq
		} else {
			token = tAssign
		}
		return
	} else if c == '<' {
		nextChar()
		if c == '=' {
			nextChar()
			token = tLessEq
		} else {
			token = tLess
		}
		return
	} else if c == '>' {
		nextChar()
		if c == '=' {
			nextChar()
			token = tGreaterEq
		} else {
			token = tGreater
		}
		return
	} else if c == '!' {
		nextChar()
		if c == '=' {
			nextChar()
			token = tNotEq
		} else {
			token = tNot
		}
		return
	}

	// Two-character tokens
	if c == '|' {
		nextChar()
		expectChar('|')
		token = tOr
		return
	} else if c == '&' {
		nextChar()
		expectChar('&')
		token = tAnd
		return
	} else if c == ':' {
		nextChar()
		expectChar('=')
		token = tDeclAssign
		return
	}

	error("unexpected '" + charStr(c) + "'")
}

func tokenStr(t int) string {
	if t < 0 {
		return "EOF"
	} else if t > ' ' {
		return charStr(t)
	} else if t == tIf {
		return "\"if\""
	} else if t == tElse {
		return "\"else\""
	} else if t == tFor {
		return "\"for\""
	} else if t == tVar {
		return "\"var\""
	} else if t == tFunc {
		return "\"func\""
	} else if t == tReturn {
		return "\"return\""
	} else if t == tPackage {
		return "\"package\""
	} else if t == tIntLit {
		return "integer " + intStr(intToken)
	} else if t == tStrLit {
		return "string " + quoteStr(strToken)
	} else if t == tIdent {
		return "identifier \"" + strToken + "\""
	} else if t == tOr {
		return "||"
	} else if t == tAnd {
		return "&&"
	} else if t == tEq {
		return "=="
	} else if t == tNotEq {
		return "!="
	} else if t == tLessEq {
		return "<="
	} else if t == tGreaterEq {
		return ">="
	} else if t == tDeclAssign {
		return ":="
	} else {
		return "unknown token " + intStr(t)
	}
}

func expect(expected int, msg string) {
	if token != expected {
		error("expected " + msg + " not " + tokenStr(token))
	}
	nextToken()
}

func quoteStr(s string) string {
	i := 0
	quoted := "\""
	for i < len(s) {
		if s[i] == '"' {
			quoted = quoted + "\\\""
		} else if s[i] == '\\' {
			quoted = quoted + "\\\\"
		} else if s[i] == '\t' {
			quoted = quoted + "\\t"
		} else if s[i] == '\r' {
			quoted = quoted + "\\r"
		} else if s[i] == '\n' {
			quoted = quoted + "\\n"
		} else {
			quoted = quoted + charStr(int(s[i]))
		}
		i = i + 1
	}
	return quoted + "\""
}

func Literal() {
	if token == tIntLit {
		print(intStr(intToken))
		nextToken()
	} else if token == tStrLit {
		print(quoteStr(strToken))
		nextToken()
	} else {
		error("expected integer or string literal")
	}
}

func Operand() {
	if token == tIntLit || token == tStrLit {
		Literal()
	} else if token == tIdent {
		print(strToken)
		identifier("identifier")
	} else if token == tLParen {
		nextToken()
		print("(")
		Expression()
		print(")")
		expect(tRParen, ")")
	} else {
		error("expected literal, identifier, or (expression)")
	}
}

func ExpressionList() {
	// TODO: this doesn't parse trailing commas correctly -- probably same for ParameterList
	Expression()
	for token == tComma {
		nextToken()
		print(", ")
		Expression()
	}
}

func Arguments() {
	expect(tLParen, "(")
	print("(")
	if token != tRParen {
		ExpressionList()
		if token == tComma {
			nextToken()
		}
	}
	expect(tRParen, ")")
	print(")")
}

func Index() {
	expect(tLBracket, "[")
	print("[")
	Expression()
	expect(tRBracket, "]")
	print("]")
}

func PrimaryExpr() {
	Operand()
	if token == tLParen {
		Arguments()
	} else if token == tLBracket {
		Index()
	}
}

func UnaryExpr() {
	if token == tPlus || token == tMinus || token == tNot {
		print(tokenStr(token))
		nextToken()
		UnaryExpr()
		return
	}
	PrimaryExpr()
}

func mulExpr() {
	UnaryExpr()
	for token == tTimes || token == tDivide || token == tModulo {
		print(" " + tokenStr(token) + " ")
		nextToken()
		UnaryExpr()
	}
}

func addExpr() {
	mulExpr()
	for token == tPlus || token == tMinus {
		print(" " + tokenStr(token) + " ")
		nextToken()
		mulExpr()
	}
}

func comparisonExpr() {
	addExpr()
	for token == tEq || token == tNotEq || token == tLess || token == tLessEq ||
		token == tGreater || token == tGreaterEq {
		print(" " + tokenStr(token) + " ")
		nextToken()
		addExpr()
	}
}

func andExpr() {
	comparisonExpr()
	for token == tAnd {
		print(" " + tokenStr(token) + " ")
		nextToken()
		comparisonExpr()
	}
}

func orExpr() {
	andExpr()
	for token == tOr {
		print(" " + tokenStr(token) + " ")
		nextToken()
		andExpr()
	}
}

func Expression() {
	orExpr()
}

func identifier(msg string) {
	expect(tIdent, msg)
}

func PackageClause() {
	expect(tPackage, "\"package\"")
	print("// package " + strToken + "\n\n")
	identifier("package identifier")
}

func Type() {
	// Only type names are supported right now
	identifier("type name")
}

func VarSpec() {
	// We only support a single identifier, not a list
	varName := strToken
	identifier("variable identifier")
	typeName := strToken
	Type()
	if token == tAssign {
		nextToken()
		print(typeName + " " + varName + " = ")
		Expression()
	} else {
		print(typeName + " " + varName) // TODO: assign type's zero value
	}
	print(";\n")
}

func VarDecl() {
	expect(tVar, "\"var\"")
	if token == tLParen {
		nextToken()
		for token != tEOF && token != tRParen {
			VarSpec()
			expect(tSemicolon, ";")
		}
		expect(tRParen, ")")
	} else {
		VarSpec()
	}
}

func ParameterDecl() {
	paramName := strToken
	identifier("parameter name")
	typeName := strToken
	Type()
	print(typeName + " " + paramName)
}

func ParameterList() {
	ParameterDecl()
	for token == tComma {
		nextToken()
		print(", ")
		ParameterDecl()
	}
}

func Parameters() {
	expect(tLParen, "(")
	if token != tRParen {
		ParameterList()
		if token == tComma {
			nextToken()
		}
	}
	expect(tRParen, ")")
}

func Signature() {
	Parameters()
	if token != tLBrace {
		Type()
	}
}

func SimpleStmt() {
	Expression()
	if token == tAssign || token == tDeclAssign {
		nextToken()
		print(" = ")
		Expression()
	}
	print(";\n")
}

func ReturnStmt() {
	expect(tReturn, "\"return\"")
	print("return")
	if token != tSemicolon {
		print(" ")
		Expression()
	}
	print(";\n")
}

func IfStmt() {
	expect(tIf, "\"if\"")
	print("if (")
	Expression()
	print(") ")
	Block()
	if token == tElse {
		nextToken()
		print(" else ")
		if token == tIf {
			IfStmt()
		} else {
			Block()
		}
	}
}

func ForStmt() {
	expect(tFor, "\"for\"")
	print("while (")
	Expression()
	print(") ")
	Block()
}

func Statement() {
	if token == tVar {
		Declaration()
	} else if token == tReturn {
		ReturnStmt()
	} else if token == tLBrace {
		Block()
	} else if token == tIf {
		IfStmt()
	} else if token == tFor {
		ForStmt()
	} else {
		SimpleStmt()
	}
}

func StatementList() {
	for token != tEOF && token != tRBrace {
		Statement()
		expect(tSemicolon, ";")
	}
}

func Block() {
	expect(tLBrace, "{")
	print("{\n")
	StatementList()
	expect(tRBrace, "}")
	print("}\n")
}

func FunctionBody() {
	Block()
}

func FunctionDecl() {
	expect(tFunc, "\"func\"")
	print("void " + strToken + "(")
	identifier("function name")
	Signature()
	print(") ")
	FunctionBody()
}

func Declaration() {
	// We don't support ConstDecl or TypeDecl
	VarDecl()
}

func TopLevelDecl() {
	if token == tVar {
		VarDecl()
	} else if token == tFunc {
		FunctionDecl()
	} else {
		error("expected \"var\" or \"func\"")
	}
	print("\n")
}

func SourceFile() {
	PackageClause()
	expect(tSemicolon, ";")

	for token == tVar || token == tFunc {
		TopLevelDecl()
		expect(tSemicolon, ";")
	}

	expect(tEOF, "end of file")
}

func main() {
	nextChar()
	nextToken()

	SourceFile()
}
