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

	token      int
	intToken   int
	strToken   string
	inFunc     bool
	calledName string

	globals     []string
	globalTypes []int

	funcs          []string
	funcSigIndexes []int // indexes into funcSigs
	funcSigs       []int // for each func: retType N arg1Type ... argNType

	locals     []string
	localTypes []int

	strs     []string
	strAddrs []int
)

var (
	typeVoid        int = 0
	typeInt         int = 1
	typeString      int = 2
	typeSliceInt    int = 3
	typeSliceString int = 4
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
		return "string " + quoteStr(strToken, "\"")
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

func typeStr(typ int) string {
	if typ == typeVoid {
		return "void"
	} else if typ == typeInt {
		return "int"
	} else if typ == typeString {
		return "string"
	} else if typ == typeSliceInt {
		return "[]int"
	} else if typ == typeSliceString {
		return "[]string"
	} else {
		return "unknown type " + intStr(typ)
	}
}

func typeSize(typ int) int {
	if typ == typeInt {
		return 8
	} else if typ == typeString {
		return 16
	} else if typ == typeSliceInt {
		return 24
	} else if typ == typeSliceString {
		return 24
	} else {
		error("unknown type " + intStr(typ))
	}
	return 0
}

func expect(expected int, msg string) {
	if token != expected {
		error("expected " + msg + " not " + tokenStr(token))
	}
	nextToken()
}

func quoteStr(s string, delim string) string {
	i := 0
	quoted := delim
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
	return quoted + delim
}

func Literal() {
	if token == tIntLit {
		print("push qword " + intStr(intToken) + "\n")

		nextToken()
	} else if token == tStrLit {
		if len(strs) == 0 {
			strs = append(strs, strToken)
			strAddrs = append(strAddrs, 0)
		} else {
			lastLen := len(strs[len(strs)-1])
			lastAddr := strAddrs[len(strAddrs)-1]
			strs = append(strs, strToken)
			strAddrs = append(strAddrs, lastAddr+lastLen)
		}

		print("push qword " + intStr(len(strToken)) + "\n")
		print("push qword str" + intStr(len(strs)-1) + "\n")

		nextToken()
	} else {
		error("expected integer or string literal")
	}
}

func findLocal(name string) int {
	i := 0
	for i < len(locals) {
		if locals[i] == name {
			return i
		}
		i = i + 1
	}
	return -1
}

func findGlobal(name string) int {
	i := 0
	for i < len(globals) {
		if globals[i] == name {
			return i
		}
		i = i + 1
	}
	return -1
}

func findFunc(name string) int {
	i := 0
	for i < len(funcs) {
		if funcs[i] == name {
			return i
		}
		i = i + 1
	}
	return -1
}

func genIdentifier(name string) {
	localIndex := findLocal(name)
	if localIndex >= 0 {
		print("push qword [rbp+TODO]\n")
		return
	}
	globalIndex := findGlobal(name)
	if globalIndex >= 0 {
		print("push qword [" + name + "]\n") // TODO: handle strings and slices
		return
	}
	funcIndex := findFunc(name)
	if funcIndex >= 0 {
		calledName = name // save called function name for later
		return
	}
	error("identifier " + quoteStr(name, "\"") + " not defined")
}

func Operand() {
	if token == tIntLit || token == tStrLit {
		Literal()
	} else if token == tIdent {
		genIdentifier(strToken)
		identifier("identifier")
	} else if token == tLParen {
		nextToken()
		Expression()
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
		Expression()
	}
}

func argsSize(funcName string) int {
	i := findFunc(funcName)
	if i < 0 {
		error("function " + quoteStr(funcName, "\"") + " not defined")
	}
	sigIndex := funcSigIndexes[i]
	numArgs := funcSigs[sigIndex+1]
	size := 0
	i = 0
	for i < numArgs {
		size = size + typeSize(funcSigs[sigIndex+2+i])
		i = i + 1
	}
	return size
}

func Arguments() {
	expect(tLParen, "(")
	if token != tRParen {
		ExpressionList()
		if token == tComma {
			nextToken()
		}
	}
	expect(tRParen, ")")
	print("call " + calledName + "\n")
	print("add rsp, " + intStr(argsSize(calledName)) + "\n")
}

func Index() {
	expect(tLBracket, "[")
	Expression()
	expect(tRBracket, "]")
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
		nextToken()
		UnaryExpr()
		return
	}
	PrimaryExpr()
}

func mulExpr() {
	UnaryExpr()
	for token == tTimes || token == tDivide || token == tModulo {
		nextToken()
		UnaryExpr()
	}
}

func addExpr() {
	mulExpr()
	for token == tPlus || token == tMinus {
		nextToken()
		mulExpr()
	}
}

func comparisonExpr() {
	addExpr()
	for token == tEq || token == tNotEq || token == tLess || token == tLessEq ||
		token == tGreater || token == tGreaterEq {
		nextToken()
		addExpr()
	}
}

func andExpr() {
	comparisonExpr()
	for token == tAnd {
		nextToken()
		comparisonExpr()
	}
}

func orExpr() {
	andExpr()
	for token == tOr {
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
	identifier("package identifier")
}

func Type() int {
	// Only type names are supported right now
	if token == tLBracket {
		nextToken()
		expect(tRBracket, "]")
		typeName := strToken
		identifier("\"int\" or \"string\"")
		if typeName == "int" {
			return typeSliceInt
		} else if typeName == "string" {
			return typeSliceString
		} else {
			error("only []int and []string are supported")
		}
	}
	typeName := strToken
	identifier("\"int\" or \"string\"")
	if typeName == "int" {
		return typeInt
	} else if typeName == "string" {
		return typeString
	} else {
		error("only int and string are supported")
	}
	return typeVoid
}

func VarSpec() {
	// We only support a single identifier, not a list
	varName := strToken
	globals = append(globals, varName)
	identifier("variable identifier")
	typ := Type()
	globalTypes = append(globalTypes, typ)
	if token == tAssign {
		nextToken()
		Expression()
	} else {
		// TODO: assign type's zero value
	}
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
	identifier("parameter name")
	typ := Type()
	funcSigs = append(funcSigs, typ)
	resultIndex := funcSigIndexes[len(funcSigIndexes)-1]
	funcSigs[resultIndex+1] = typ // increment numArgs
}

func ParameterList() {
	ParameterDecl()
	for token == tComma {
		nextToken()
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
	funcSigs = append(funcSigs, typeVoid) // space for result type
	funcSigs = append(funcSigs, 0)        // space for numArgs
	Parameters()
	if token != tLBrace {
		typ := Type()
		resultIndex := funcSigIndexes[len(funcSigIndexes)-1]
		funcSigs[resultIndex] = typ // set result type
	}
}

func SimpleStmt() {
	Expression()
	if token == tAssign || token == tDeclAssign {
		nextToken()
		Expression()
	}
}

func ReturnStmt() {
	expect(tReturn, "\"return\"")
	if token != tSemicolon {
		Expression()
	}
}

func IfStmt() {
	expect(tIf, "\"if\"")
	Expression()
	Block()
	if token == tElse {
		nextToken()
		if token == tIf {
			IfStmt()
		} else {
			Block()
		}
	}
}

func ForStmt() {
	expect(tFor, "\"for\"")
	Expression()
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
	StatementList()
	expect(tRBrace, "}")
}

func FunctionBody() {
	inFunc = true
	Block()
	inFunc = false
}

func FunctionDecl() {
	expect(tFunc, "\"func\"")
	print("\n")
	print(strToken + ":\n")
	print("push rbp\n")
	print("mov rbp, rsp\n")
	funcs = append(funcs, strToken)
	funcSigIndexes = append(funcSigIndexes, len(funcSigs))
	identifier("function name")
	Signature()
	FunctionBody()
	print("pop rbp\n")
	print("ret\n")
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

func dumpGlobals() {
	i := 0
	for i < len(globals) {
		print("GLOBAL: " + globals[i] + " " + typeStr(globalTypes[i]) + "\n")
		i = i + 1
	}
}

func dumpFuncs() {
	i := 0
	for i < len(funcs) {
		print("FUNC: " + funcs[i] + "(")
		sigIndex := funcSigIndexes[i]
		resultType := funcSigs[sigIndex]
		numArgs := funcSigs[sigIndex+1]
		j := 0
		for j < numArgs {
			argType := funcSigs[sigIndex+2+j]
			print(typeStr(argType))
			if j < numArgs-1 {
				print(", ")
			}
			j = j + 1
		}
		print(") " + typeStr(resultType) + "\n")
		i = i + 1
	}
}

func dumpStrs() {
	print("\n")
	print("section .data\n")
	i := 0
	for i < len(strs) {
		print("str" + intStr(i) + ": db " + quoteStr(strs[i], "`") + "\n")
		i = i + 1
	}
}

func main() {
	print("global _start\n")
	print("section .text\n")
	print("\n")
	print("_start:\n")
	print("call main\n")
	print("mov rax, 60\n") // system call for "exit"
	print("mov rdi, 0\n")  // exit code 0
	print("syscall\n")
	print("\n")
	print("print:\n")
	print("push rbp\n")
	print("mov rbp, rsp\n")
	print("mov rax, 1\n")        // system call for "write"
	print("mov rdi, 1\n")        // file handle 1 is stdout
	print("mov rsi, [rbp+16]\n") // address
	print("mov rdx, [rbp+24]\n") // length
	print("syscall\n")
	print("pop rbp\n")
	print("ret\n")

	// Define builtin: func print(s string)
	funcs = append(funcs, "print")
	funcSigIndexes = append(funcSigIndexes, len(funcSigs))
	funcSigs = append(funcSigs, typeVoid)
	funcSigs = append(funcSigs, 1)
	funcSigs = append(funcSigs, typeString)

	nextChar()
	nextToken()

	SourceFile()

	dumpGlobals()
	//	dumpFuncs()
	dumpStrs()
}
