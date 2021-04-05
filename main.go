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
	line int
	col  int

	token       int
	intToken    int
	strToken    string
	currentFunc string

	consts []string

	globals     []string
	globalTypes []int

	locals     []string
	localTypes []int

	funcs          []string
	funcSigIndexes []int // indexes into funcSigs
	funcSigs       []int // for each func: retType N arg1Type ... argNType

	strs     []string
	strAddrs []int
)

const (
	typeVoid        int = 1
	typeInt         int = 2
	typeString      int = 3
	typeSliceInt    int = 4
	typeSliceString int = 5
)

const (
	tEOF int = -1

	// Keywords
	tIf      int = 1
	tElse    int = 2
	tFor     int = 3
	tVar     int = 4
	tConst   int = 5
	tFunc    int = 6
	tReturn  int = 7
	tPackage int = 8

	// Literals and identifiers
	tIntLit int = 9
	tStrLit int = 10
	tIdent  int = 11

	// Two-character tokens
	tOr         int = 12
	tAnd        int = 13
	tEq         int = 14
	tNotEq      int = 15
	tLessEq     int = 16
	tGreaterEq  int = 17
	tDeclAssign int = 18

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
		} else if strToken == "const" {
			token = tConst
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
	} else if t == tConst {
		return "\"const\""
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
	if typ == typeVoid {
		return 0
	} else if typ == typeInt {
		return 8
	} else if typ == typeString {
		return 16
	} else if typ == typeSliceInt {
		return 24
	} else if typ == typeSliceString {
		return 24
	} else {
		error("unknown type " + intStr(typ))
		return 0
	}
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

// Code generator functions

func genProgramStart() {
	print("global _start\n")
	print("section .text\n")
	print("\n")

	print("_start:\n")
	print("mov rax, _space\n")
	print("mov [_spacePtr], rax\n")
	print("call main\n")
	print("mov rax, 60\n") // system call for "exit"
	print("mov rdi, 0\n")  // exit code 0
	print("syscall\n")
	print("\n")

	print("print:\n")
	print("push rbp\n") // rbp ret addr len
	print("mov rbp, rsp\n")
	print("mov rax, 1\n")        // system call for "write"
	print("mov rdi, 1\n")        // file handle 1 is stdout
	print("mov rsi, [rbp+16]\n") // address
	print("mov rdx, [rbp+24]\n") // length
	print("syscall\n")
	print("pop rbp\n")
	print("ret\n")
	print("\n")

	print("_strAdd:\n")
	print("push rbp\n") // rbp ret addr1 len1 addr0 len0
	print("mov rbp, rsp\n")
	// Allocate len0+len1 bytes
	print("mov rax, [rbp+24]\n") // len1
	print("add rax, [rbp+40]\n") // len1 + len0
	print("push rax\n")
	print("call _alloc\n")
	print("add rsp, 8\n")
	// Move len0 bytes from addr0 to addrNew
	print("mov rsi, [rbp+32]\n")
	print("mov rdi, rax\n")
	print("mov rcx, [rbp+40]\n")
	print("rep movsb\n")
	// Move len1 bytes from addr1 to addrNew+len0
	print("mov rsi, [rbp+16]\n")
	print("mov rdi, rax\n")
	print("add rdi, [rbp+40]\n")
	print("mov rcx, [rbp+24]\n")
	print("rep movsb\n")
	// Return addrNew len0+len1 (addrNew already in rax)
	print("mov rbx, [rbp+24]\n")
	print("add rbx, [rbp+40]\n")
	print("pop rbp\n")
	print("ret\n")
	print("\n")

	// TODO: check for out of memory
	print("_alloc:\n")
	print("push rbp\n") // rbp ret size
	print("mov rbp, rsp\n")
	print("mov rax, [_spacePtr]\n")
	print("mov rbx, [rbp+16]\n")
	print("add qword [_spacePtr], rbx\n")
	print("pop rbp\n")
	print("ret\n")
}

func genConst(name string, value int) {
	print(name + " equ " + intStr(value) + "\n")
}

func genIntLit(n int) {
	print("push qword " + intStr(n) + "\n")
}

func genStrLit(s string) {
	// Add string to strs and strAddrs tables
	if len(strs) == 0 {
		strs = append(strs, s)
		strAddrs = append(strAddrs, 0)
	} else {
		lastLen := len(strs[len(strs)-1])
		lastAddr := strAddrs[len(strAddrs)-1]
		strs = append(strs, s)
		strAddrs = append(strAddrs, lastAddr+lastLen)
	}
	// Push string struct: length and then address (by label)
	print("push qword " + intStr(len(s)) + "\n")
	print("push qword str" + intStr(len(strs)-1) + "\n")
}

func localOffset(index int) int {
	funcIndex := findFunc(currentFunc)
	sigIndex := funcSigIndexes[funcIndex]
	numArgs := funcSigs[sigIndex+1]
	if index < numArgs {
		// Function argument local (add to rbp; args are on stack in reverse)
		offset := 16
		i := numArgs - 1
		for i > index {
			offset = offset + typeSize(localTypes[i])
			i = i - 1
		}
		return offset
	} else {
		// Declared local (subtract from rbp)
		offset := 0
		i := numArgs
		for i <= index {
			offset = offset - typeSize(localTypes[i])
			i = i + 1
		}
		return offset
	}
}

func genFetch(typ int, addr string) {
	if typ == typeInt {
		print("push qword [" + addr + "]\n")
	} else if typ == typeString {
		print("push qword [" + addr + "+8]\n")
		print("push qword [" + addr + "]\n")
	} else { // slice
		print("push qword [" + addr + "+16]\n")
		print("push qword [" + addr + "+8]\n")
		print("push qword [" + addr + "]\n")
	}
}

func genLocalFetch(index int) int {
	offset := localOffset(index)
	typ := localTypes[index]
	genFetch(typ, "rbp+"+intStr(offset))
	return typ
}

func genGlobalFetch(index int) int {
	name := globals[index]
	typ := globalTypes[index]
	genFetch(typ, name)
	return typ
}

func genConstFetch(index int) int {
	name := consts[index]
	print("push qword " + name + "\n")
	return typeInt
}

func genIdentifier(name string) int {
	localIndex := findLocal(name)
	if localIndex >= 0 {
		return genLocalFetch(localIndex)
	}
	globalIndex := findGlobal(name)
	if globalIndex >= 0 {
		return genGlobalFetch(globalIndex)
	}
	constIndex := findConst(name)
	if constIndex >= 0 {
		return genConstFetch(constIndex)
	}
	funcIndex := findFunc(name)
	if funcIndex >= 0 {
		sigIndex := funcSigIndexes[funcIndex]
		return funcSigs[sigIndex] // result type
	}
	error("identifier " + quoteStr(name, "\"") + " not defined")
	return 0
}

func genAssignInstrs(typ int, addr string) {
	if typ == typeInt {
		print("pop qword [" + addr + "]\n")
	} else if typ == typeString {
		print("pop qword [" + addr + "]\n")
		print("pop qword [" + addr + "+8]\n")
	} else { // slice
		print("pop qword [" + addr + "]\n")
		print("pop qword [" + addr + "+8]\n")
		print("pop qword [" + addr + "+16]\n")
	}
}

func genLocalAssign(index int) {
	offset := localOffset(index)
	genAssignInstrs(localTypes[index], "rbp+"+intStr(offset))
}

func genGlobalAssign(index int) {
	name := globals[index]
	genAssignInstrs(globalTypes[index], name)
}

func genAssign(name string) {
	localIndex := findLocal(name)
	if localIndex >= 0 {
		genLocalAssign(localIndex)
		return
	}
	globalIndex := findGlobal(name)
	if globalIndex >= 0 {
		genGlobalAssign(globalIndex)
		return
	}
	error("identifier " + quoteStr(name, "\"") + " not defined (or not assignable)")
}

func genCall(name string) {
	print("call " + name + "\n")
	size := argsSize(name)
	if size > 0 {
		print("add rsp, " + intStr(size) + "\n")
	}
	index := findFunc(name)
	sigIndex := funcSigIndexes[index]
	resultType := funcSigs[sigIndex]
	if resultType == typeInt {
		print("push rax\n")
	} else if resultType == typeString {
		print("push rbx\n")
		print("push rax\n")
	}
}

func genFuncStart(name string) {
	print("\n")
	print(name + ":\n")
	print("push rbp\n")
	print("mov rbp, rsp\n")
	print("sub rsp, 160\n") // TODO: enough space for 10 strings -- huge hack!
}

func genFuncEnd() {
	print("add rsp, 160\n")
	print("pop rbp\n")
	print("ret\n")
}

func genDataSections() {
	print("\n")
	print("section .data\n")
	i := 0
	for i < len(strs) {
		print("str" + intStr(i) + ": db " + quoteStr(strs[i], "`") + "\n")
		i = i + 1
	}
	print("align 8\n")
	i = 0
	for i < len(globals) {
		typ := globalTypes[i]
		if typ == typeInt {
			print(globals[i] + ": dq 0\n")
		} else if typ == typeString {
			print(globals[i] + ": dq 0, 0\n") // string: address, length
		} else {
			print(globals[i] + ": dq 0, 0, 0\n") // slice: address, length, capacity
		}
		i = i + 1
	}

	print("\n")
	print("section .bss\n")
	print("_spacePtr: resq 1\n")
	print("_space: resb 1048576\n")
}

func genUnary(op int, typ int) {
	if typ != typeInt {
		error("unary operator not allowed on type " + typeStr(typ))
	}
	print("pop rax\n")
	if op == tMinus {
		print("neg rax\n")
	} else if op == tNot {
		print("cmp rax, 0\n")
		print("setz rax\n")
	}
	print("push rax\n")
}

func genBinary(op int, typ1 int, typ2 int) {
	if typ1 != typ2 {
		error("binary operands must be the same type")
	}
	if typ1 == typeString {
		genBinaryString(op)
	} else {
		genBinaryInt(op)
	}
}

func genBinaryString(op int) {
	if op == tPlus {
		print("call _strAdd\n")
		print("add rsp, 32\n")
		print("push rbx\n")
		print("push rax\n")
	} else if op == tEq {
		print("call _strEq\n")
		print("add rsp, 32\n")
		print("push rax\n")
	} else if op == tNotEq {
		print("call _strEq\n")
		print("add rsp, 32\n")
		print("cmp rax, 0\n")
		print("setz rax\n")
		print("push rax\n")
	} else {
		error("operator " + tokenStr(op) + " not allowed on strings")
	}
}

func genBinaryInt(op int) {
	print("pop rbx\n")
	print("pop rax\n")
	if op == tPlus {
		print("add rax, rbx\n")
	} else if op == tMinus {
		print("sub rax, rbx\n")
	} else if op == tTimes {
		print("imul rbx\n")
	} else if op == tDivide {
		print("cdq\n")
		print("idiv rbx\n")
	} else if op == tModulo {
		print("cdq\n")
		print("idiv rbx\n")
		print("mov rax, rdx\n")
	} else if op == tEq {
		print("cmp rax, rbx\n")
		print("sete rax\n")
	} else if op == tNotEq {
		print("cmp rax, rbx\n")
		print("setne rax\n")
	} else if op == tLess {
		print("cmp rax, rbx\n")
		print("setl rax\n")
	} else if op == tLessEq {
		print("cmp rax, rbx\n")
		print("setle rax\n")
	} else if op == tGreater {
		print("cmp rax, rbx\n")
		print("setg rax\n")
	} else if op == tGreaterEq {
		print("cmp rax, rbx\n")
		print("setge rax\n")
	} else if op == tAnd {
		print("and rax, rbx\n")
	} else if op == tOr {
		print("or rax, rbx\n")
	}
	print("push rax\n")
}

func genReturn(typ int) {
	if typ == typeInt {
		print("pop rax\n")
	} else if typ == typeString {
		print("pop rax\n")
		print("pop rbx\n")
	} else if typ != typeVoid {
		error("can only return int or string")
	}
	genFuncEnd()
}

// Parser functions

func Literal() int {
	if token == tIntLit {
		genIntLit(intToken)
		nextToken()
		return typeInt
	} else if token == tStrLit {
		genStrLit(strToken)
		nextToken()
		return typeString
	} else {
		error("expected integer or string literal")
		return 0
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

func findConst(name string) int {
	i := 0
	for i < len(consts) {
		if consts[i] == name {
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

func Operand() int {
	if token == tIntLit || token == tStrLit {
		return Literal()
	} else if token == tIdent {
		name := strToken
		identifier("identifier")
		return genIdentifier(name)
	} else if token == tLParen {
		nextToken()
		typ := Expression()
		expect(tRParen, ")")
		return typ
	} else {
		error("expected literal, identifier, or (expression)")
		return 0
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
	calledName := strToken
	expect(tLParen, "(")
	if token != tRParen {
		ExpressionList()
		if token == tComma {
			nextToken()
		}
	}
	expect(tRParen, ")")
	genCall(calledName)
}

func Index() {
	expect(tLBracket, "[")
	Expression()
	expect(tRBracket, "]")
}

func PrimaryExpr() int {
	typ := Operand()
	if token == tLParen {
		Arguments()
		return typ
	} else if token == tLBracket {
		Index()
		if typ == typeSliceInt {
			return typeInt
		} else if typ == typeSliceString {
			return typeString
		} else {
			error("invalid slice type " + typeStr(typ))
		}
	}
	return typ
}

func UnaryExpr() int {
	if token == tPlus || token == tMinus || token == tNot {
		op := token
		nextToken()
		typ := UnaryExpr()
		genUnary(op, typ)
		return typ
	}
	return PrimaryExpr()
}

func mulExpr() int {
	typ1 := UnaryExpr()
	for token == tTimes || token == tDivide || token == tModulo {
		op := token
		nextToken()
		typ2 := UnaryExpr()
		genBinary(op, typ1, typ2)
	}
	return typ1
}

func addExpr() int {
	typ1 := mulExpr()
	for token == tPlus || token == tMinus {
		op := token
		nextToken()
		typ2 := mulExpr()
		genBinary(op, typ1, typ2)
	}
	return typ1
}

func comparisonExpr() int {
	typ1 := addExpr()
	for token == tEq || token == tNotEq || token == tLess || token == tLessEq ||
		token == tGreater || token == tGreaterEq {
		op := token
		nextToken()
		typ2 := addExpr()
		genBinary(op, typ1, typ2)
	}
	return typ1
}

func andExpr() int {
	typ1 := comparisonExpr()
	for token == tAnd {
		op := token
		nextToken()
		typ2 := comparisonExpr()
		genBinary(op, typ1, typ2)
	}
	return typ1
}

func orExpr() int {
	typ1 := andExpr()
	for token == tOr {
		op := token
		nextToken()
		typ2 := andExpr()
		genBinary(op, typ1, typ2)
	}
	return typ1
}

func Expression() int {
	return orExpr()
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
	identifier("variable identifier")
	typ := Type()
	if currentFunc == "" {
		globals = append(globals, varName)
		globalTypes = append(globalTypes, typ)
		if token == tAssign {
			error("assignment not supported for top-level var")
		}
	} else {
		defineLocal(typ, varName)
		if token == tAssign {
			nextToken()
			Expression()
			genAssign(varName)
		}
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

func ConstSpec() {
	// We only support typed integer constants
	name := strToken
	consts = append(consts, name)
	identifier("variable identifier")
	typ := Type()
	if typ != typeInt {
		error("constants must be typed int")
	}
	expect(tAssign, "=")
	value := intToken
	expect(tIntLit, "integer literal")
	genConst(name, value)
}

func ConstDecl() {
	expect(tConst, "\"const\"")
	if token == tLParen {
		nextToken()
		for token != tEOF && token != tRParen {
			ConstSpec()
			expect(tSemicolon, ";")
		}
		expect(tRParen, ")")
	} else {
		ConstSpec()
	}
}

func ParameterDecl() {
	paramName := strToken
	identifier("parameter name")
	typ := Type()
	defineLocal(typ, paramName)
	funcSigs = append(funcSigs, typ)
	resultIndex := funcSigIndexes[len(funcSigIndexes)-1]
	funcSigs[resultIndex+1] = funcSigs[resultIndex+1] + 1 // increment numArgs
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

func defineLocal(typ int, name string) {
	locals = append(locals, name)
	localTypes = append(localTypes, typ)
}

func SimpleStmt() {
	// Funky parsing here to handle assignments
	if token == tIdent {
		identName := strToken
		nextToken()
		if token == tDeclAssign {
			nextToken()
			typ := Expression()
			defineLocal(typ, identName)
			genAssign(identName)
		} else if token == tLParen {
			genIdentifier(identName)
			Arguments()
		} else if token == tRBracket {
			genIdentifier(identName)
			Index()
		} else {
			genIdentifier(identName)
		}
	} else {
		Expression()
	}
}

func ReturnStmt() {
	expect(tReturn, "\"return\"")
	if token != tSemicolon {
		typ := Expression()
		genReturn(typ)
	} else {
		genReturn(typeVoid)
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
	Block()
}

func FunctionDecl() {
	expect(tFunc, "\"func\"")
	currentFunc = strToken
	genFuncStart(strToken)
	funcs = append(funcs, strToken)
	funcSigIndexes = append(funcSigIndexes, len(funcSigs))
	identifier("function name")
	Signature()
	FunctionBody()
	genFuncEnd()
	locals = locals[:0]
	localTypes = localTypes[:0]
	currentFunc = ""
}

func Declaration() {
	// We don't support ConstDecl or TypeDecl
	VarDecl()
}

func TopLevelDecl() {
	if token == tVar {
		VarDecl()
	} else if token == tConst {
		// ConstDecl only supported at top level
		ConstDecl()
	} else if token == tFunc {
		FunctionDecl()
	} else {
		error("expected \"var\", \"const\", or \"func\"")
	}
}

func SourceFile() {
	PackageClause()
	expect(tSemicolon, ";")

	for token == tVar || token == tFunc || token == tConst {
		TopLevelDecl()
		expect(tSemicolon, ";")
	}

	expect(tEOF, "end of file")
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

func dumpLocals() {
	i := 0
	for i < len(locals) {
		typ := localTypes[i]
		if typ == typeInt {
			print(locals[i] + ": dq 0; " + intStr(localOffset(i)) + "\n")
		} else if typ == typeString {
			print(locals[i] + ": dq 0, 0\n") // string: address, length
		} else {
			print(locals[i] + ": dq 0, 0, 0\n") // slice: address, length, capacity
		}
		i = i + 1
	}
}

func main() {
	// Define builtin: func print(s string)
	funcs = append(funcs, "print")
	funcSigIndexes = append(funcSigIndexes, len(funcSigs))
	funcSigs = append(funcSigs, typeVoid)
	funcSigs = append(funcSigs, 1)
	funcSigs = append(funcSigs, typeString)

	genProgramStart()

	line = 1
	col = 0
	nextChar()
	nextToken()
	SourceFile()

	genDataSections()
}
