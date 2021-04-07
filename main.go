// Mugo: compiler for a (micro) subset of Go

package main

// TODO:
// * ensure .bss is zeroed
// * consistent/better naming, e.g., readByte -> getc, intStr -> itoa, printError -> log, ec
// * consider "ret 8" style cleanup, better ABI, esp for locals than sub rsp, 160?

var (
	c    int
	line int
	col  int

	token       int
	intToken    int
	strToken    string
	currentFunc string
	labelNum    int

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
	tColon     int = ':'
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

	tEOF int = 256
)

func nextChar() {
	c = readByte()
	col = col + 1
	if c == '\n' {
		line = line + 1
		col = 0
	}
}

func intStr(n int) string {
	if n < 0 {
		return "-" + intStr(-n)
	}
	if n < 10 {
		return charStr(n + '0')
	}
	return intStr(n/10) + intStr(n%10)
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

func nextToken() {
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
	// TODO: can we do this all in skipWhitespace inline at the top?
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
	// TODO: consider factoring out, e.g., tokenChoice('=', tEq, tAssign)
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
	} else if c == ':' {
		nextChar()
		if c == '=' {
			nextChar()
			token = tDeclAssign
		} else {
			token = tColon
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
	}

	error("unexpected '" + charStr(c) + "'")
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
		} else if s[i] == '`' {
			quoted = quoted + "\\`"
		} else {
			quoted = quoted + charStr(int(s[i]))
		}
		i = i + 1
	}
	return quoted + delim
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
	} else if typ == typeSliceInt || typ == typeSliceString {
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

// TODO: consider factoring: find(names []string, name string) int
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

	print("printError:\n")
	print("push rbp\n") // rbp ret addr len
	print("mov rbp, rsp\n")
	print("mov rax, 1\n")        // system call for "write"
	print("mov rdi, 2\n")        // file handle 2 is stderr
	print("mov rsi, [rbp+16]\n") // address
	print("mov rdx, [rbp+24]\n") // length
	print("syscall\n")
	print("pop rbp\n")
	print("ret\n")
	print("\n")

	print("readByte:\n")
	print("push qword 0\n")
	print("mov rax, 0\n")   // system call for "read"
	print("mov rdi, 0\n")   // file handle 0 is stdin
	print("mov rsi, rsp\n") // address
	print("mov rdx, 1\n")   // length
	print("syscall\n")
	print("cmp rax, 1\n")
	print("je _readByte1\n")
	print("mov qword [rsp], -1\n")
	print("_readByte1:\n")
	print("pop rax\n")
	print("ret\n")
	print("\n")

	print("exit:\n")
	print("mov rdi, [rsp+8]\n") // code
	print("mov rax, 60\n")      // system call for "exit"
	print("syscall\n")
	print("\n")

	print("int:\n")
	print("mov rax, [rsp+8]\n") // value
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

	print("_strEq:\n")
	print("push rbp\n") // rbp ret addr1 len1 addr0 len0
	print("mov rbp, rsp\n")
	print("mov rcx, [rbp+40]\n")
	print("cmp rcx, [rbp+24]\n")
	print("jne _strEqNotEqual\n")
	print("mov rsi, [rbp+16]\n")
	print("mov rdi, [rbp+32]\n")
	print("rep cmpsb\n")
	print("jne _strEqNotEqual\n")
	print("mov rax, 1\n")
	print("pop rbp\n")
	print("ret\n")
	// Return addrNew len0+len1 (addrNew already in rax)
	print("_strEqNotEqual:\n")
	print("xor rax, rax\n")
	print("pop rbp\n")
	print("ret\n")
	print("\n")

	print("charStr:\n")
	print("push rbp\n") // rbp ret ch
	print("mov rbp, rsp\n")
	// Allocate 1 byte
	print("push 1\n")
	print("call _alloc\n")
	print("add rsp, 8\n")
	// Move byte to destination
	print("mov rbx, [rbp+16]\n")
	print("mov [rax], bl\n")
	// Return addrNew 1 (addrNew already in rax)
	print("mov rbx, 1\n")
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
	print("\n")

	print("_appendInt:\n")
	print("push rbp\n") // rbp ret value addr len cap
	print("mov rbp, rsp\n")
	// Ensure capacity is large enough
	print("mov rax, [rbp+32]\n") // len
	print("mov rbx, [rbp+40]\n") // cap
	print("cmp rax, rbx\n")      // if len >= cap, resize
	print("jl _appendInt1\n")
	print("add rbx, rbx\n")    // double in size
	print("jnz _appendInt2\n") // if it's zero, allocate minimum size
	print("inc rbx\n")
	print("_appendInt2:\n")
	print("mov [rbp+40], rbx\n") // update cap
	// Allocate newCap*8 bytes
	print("lea rbx, [rbx*8]\n")
	print("push rbx\n")
	print("call _alloc\n")
	print("add rsp, 8\n")
	// Move from old array to new
	print("mov rsi, [rbp+24]\n")
	print("mov rdi, rax\n")
	print("mov [rbp+24], rax\n") // update addr
	print("mov rcx, [rbp+32]\n")
	print("rep movsq\n")
	// Set addr[len] = value
	print("_appendInt1:\n")
	print("mov rax, [rbp+24]\n") // addr
	print("mov rbx, [rbp+32]\n") // len
	print("mov rdx, [rbp+16]\n") // value
	print("mov [rax+rbx*8], rdx\n")
	// Return addr len+1 cap (in rax rbx rcx)
	print("inc rbx\n")
	print("mov rcx, [rbp+40]\n")
	print("pop rbp\n")
	print("ret\n")
	print("\n")

	print("_appendString:\n")
	print("push rbp\n") // rbp ret 16strAddr 24strLen 32addr 40len 48cap
	print("mov rbp, rsp\n")
	// Ensure capacity is large enough
	print("mov rax, [rbp+40]\n") // len
	print("mov rbx, [rbp+48]\n") // cap
	print("cmp rax, rbx\n")      // if len >= cap, resize
	print("jl _appendInt3\n")
	print("add rbx, rbx\n")    // double in size
	print("jnz _appendInt4\n") // if it's zero, allocate minimum size
	print("inc rbx\n")
	print("_appendInt4:\n")
	print("mov [rbp+48], rbx\n") // update cap
	// Allocate newCap*16 bytes
	print("add rbx, rbx\n")
	print("lea rbx, [rbx*8]\n")
	print("push rbx\n")
	print("call _alloc\n")
	print("add rsp, 8\n")
	// Move from old array to new
	print("mov rsi, [rbp+32]\n")
	print("mov rdi, rax\n")
	print("mov [rbp+32], rax\n") // update addr
	print("mov rcx, [rbp+40]\n")
	print("add rcx, rcx\n")
	print("rep movsq\n")
	// Set addr[len] = strValue
	print("_appendInt3:\n")
	print("mov rax, [rbp+32]\n") // addr
	print("mov rbx, [rbp+40]\n") // len
	print("add rbx, rbx\n")
	print("mov rdx, [rbp+16]\n") // strAddr
	print("mov [rax+rbx*8], rdx\n")
	print("mov rdx, [rbp+24]\n") // strLen
	print("mov [rax+rbx*8+8], rdx\n")
	// Return addr len+1 cap (in rax rbx rcx)
	print("mov rbx, [rbp+40]\n")
	print("inc rbx\n")
	print("mov rcx, [rbp+48]\n")
	print("pop rbp\n")
	print("ret\n")

	print("_lenString:\n")
	print("push rbp\n") // rbp ret addr len
	print("mov rbp, rsp\n")
	print("mov rax, [rbp+24]\n")
	print("pop rbp\n")
	print("ret\n")
	print("\n")

	print("_lenSlice:\n") // TODO: can be same as above?
	print("push rbp\n")   // rbp ret addr len cap
	print("mov rbp, rsp\n")
	print("mov rax, [rbp+24]\n")
	print("pop rbp\n")
	print("ret\n")
	print("\n")
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

func genFetchInstrs(typ int, addr string) {
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
	genFetchInstrs(typ, "rbp+"+intStr(offset))
	return typ
}

func genGlobalFetch(index int) int {
	name := globals[index]
	typ := globalTypes[index]
	genFetchInstrs(typ, name)
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

func varType(name string) int {
	localIndex := findLocal(name)
	if localIndex >= 0 {
		return localTypes[localIndex]
	}
	globalIndex := findGlobal(name)
	if globalIndex >= 0 {
		return globalTypes[globalIndex]
	}
	error("identifier " + quoteStr(name, "\"") + " not defined")
	return 0
}

func genSliceAssign(name string) {
	typ := varType(name)
	print("pop rax\n") // value (addr if string type)
	if typ == typeSliceString {
		print("pop rbx\n") // value (len)
		print("pop rcx\n") // index * 2
		print("add rcx, rcx\n")
	} else {
		print("pop rcx\n")
	}
	localIndex := findLocal(name)
	if localIndex >= 0 {
		offset := localOffset(localIndex)
		print("mov rdx, [rbp+" + intStr(offset) + "]\n")
	} else {
		print("mov rdx, [" + name + "]\n")
	}
	// TODO: bounds checking!
	print("mov [rdx+rcx*8], rax\n")
	if typ == typeSliceString {
		print("mov [rdx+rcx*8+8], rbx\n")
	}
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
	} else if resultType == typeSliceInt || resultType == typeSliceString {
		print("push rcx\n")
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
	print("_space: resb 10485760\n")
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
		print("mov rax, 0\n")
		print("setz al\n")
	}
	print("push rax\n")
}

func genBinaryString(op int) int {
	if op == tPlus {
		print("call _strAdd\n")
		print("add rsp, 32\n")
		print("push rbx\n")
		print("push rax\n")
		return typeString
	} else if op == tEq {
		print("call _strEq\n")
		print("add rsp, 32\n")
		print("push rax\n")
		return typeInt
	} else if op == tNotEq {
		print("call _strEq\n")
		print("add rsp, 32\n")
		print("cmp rax, 0\n")
		print("mov rax, 0\n")
		print("setz al\n")
		print("push rax\n")
		return typeInt
	} else {
		error("operator " + tokenStr(op) + " not allowed on strings")
		return 0
	}
}

func genBinaryInt(op int) int {
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
		print("mov rax, 0\n")
		print("sete al\n")
	} else if op == tNotEq {
		print("cmp rax, rbx\n")
		print("mov rax, 0\n")
		print("setne al\n")
	} else if op == tLess {
		print("cmp rax, rbx\n")
		print("mov rax, 0\n")
		print("setl al\n")
	} else if op == tLessEq {
		print("cmp rax, rbx\n")
		print("mov rax, 0\n")
		print("setle al\n")
	} else if op == tGreater {
		print("cmp rax, rbx\n")
		print("mov rax, 0\n")
		print("setg al\n")
	} else if op == tGreaterEq {
		print("cmp rax, rbx\n")
		print("mov rax, 0\n")
		print("setge al\n")
	} else if op == tAnd {
		print("and rax, rbx\n")
	} else if op == tOr {
		print("or rax, rbx\n")
	}
	print("push rax\n")
	return typeInt
}

func genBinary(op int, typ1 int, typ2 int) int {
	if typ1 != typ2 {
		error("binary operands must be the same type")
	}
	if typ1 == typeString {
		return genBinaryString(op)
	} else {
		return genBinaryInt(op)
	}
}

func genReturn(typ int) {
	if typ == typeInt {
		print("pop rax\n")
	} else if typ == typeString {
		print("pop rax\n")
		print("pop rbx\n")
	} else if typ == typeSliceInt || typ == typeSliceString {
		print("pop rax\n")
		print("pop rbx\n")
		print("pop rcx\n")
	}
	genFuncEnd()
}

func genJumpIfZero(label string) {
	print("pop rax\n")
	print("cmp rax, 0\n")
	print("jz " + label + "\n")
}

func genJump(label string) {
	print("jmp " + label + "\n")
}

func genLabel(label string) {
	print("\n")
	print(label + ":\n")
}

func genDiscard(typ int) {
	size := typeSize(typ)
	if size > 0 {
		print("add rsp, " + intStr(typeSize(typ)) + "\n")
	}
}

func genSliceExpr() {
	// Slice expression of form slice[:max]
	// TODO: bounds checking!
	print("pop rax\n")  // max
	print("pop rbx\n")  // addr
	print("pop rcx\n")  // old length (capacity remains same)
	print("push rax\n") // new length
	print("push rbx\n") // addr remains same
}

func genSliceFetch(typ int) int {
	// TODO: bounds checking!
	if typ == typeString {
		print("pop rax\n") // index
		print("pop rbx\n") // addr
		print("pop rcx\n") // len
		print("xor rdx, rdx\n")
		print("mov dl, [rbx+rax]\n")
		print("push rdx\n")
		return typeInt
	} else if typ == typeSliceInt {
		print("pop rax\n") // index
		print("pop rbx\n") // addr
		print("pop rcx\n") // len
		print("pop rdx\n") // cap
		print("push qword [rbx+rax*8]\n")
		return typeInt
	} else if typ == typeSliceString {
		print("pop rax\n") // index
		print("pop rbx\n") // addr
		print("pop rcx\n") // len
		print("pop rdx\n") // cap
		print("add rax, rax\n")
		print("push qword [rbx+rax*8+8]\n")
		print("push qword [rbx+rax*8]\n")
		return typeString
	} else {
		error("invalid slice type " + typeStr(typ))
		return 0
	}
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

func identifier(msg string) {
	expect(tIdent, msg)
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

func ExpressionList() int {
	// TODO: this doesn't parse trailing commas correctly -- probably same for ParameterList
	firstType := Expression()
	for token == tComma {
		nextToken()
		Expression()
	}
	return firstType
}

func Arguments() {
	calledName := strToken // function name will still be in strToken
	expect(tLParen, "(")
	firstArgType := typeVoid
	if token != tRParen {
		firstArgType = ExpressionList()
		if token == tComma {
			nextToken()
		}
	}
	expect(tRParen, ")")

	// "Generic" built-in functions
	if calledName == "append" {
		if firstArgType == typeSliceInt {
			genCall("_appendInt")
		} else if firstArgType == typeSliceString {
			genCall("_appendString")
		} else {
			error("can't append to " + typeStr(firstArgType))
		}
		return
	}
	if calledName == "len" {
		if firstArgType == typeString {
			genCall("_lenString")
		} else if firstArgType == typeSliceInt || firstArgType == typeSliceString {
			genCall("_lenSlice")
		} else {
			error("can't get length of " + typeStr(firstArgType))
		}
		return
	}

	// Normal function call
	genCall(calledName)
}

func indexExpr() {
	typ := Expression()
	if typ != typeInt {
		error("slice index must be int")
	}
}

func PrimaryExpr() int {
	typ := Operand()
	if token == tLParen { // function call
		Arguments()
		return typ
	} else if token == tLBracket {
		nextToken()
		if token == tColon {
			if typ != typeSliceInt && typ != typeSliceString {
				error("slice expression requires slice type")
			}
			nextToken()
			indexExpr()
			expect(tRBracket, "]")
			genSliceExpr()
			return typ
		}
		indexExpr()
		expect(tRBracket, "]")
		return genSliceFetch(typ)
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
	typ := UnaryExpr()
	for token == tTimes || token == tDivide || token == tModulo {
		op := token
		nextToken()
		typRight := UnaryExpr()
		typ = genBinary(op, typ, typRight)
	}
	return typ
}

func addExpr() int {
	typ := mulExpr()
	for token == tPlus || token == tMinus {
		op := token
		nextToken()
		typRight := mulExpr()
		typ = genBinary(op, typ, typRight)
	}
	return typ
}

func comparisonExpr() int {
	typ := addExpr()
	for token == tEq || token == tNotEq || token == tLess || token == tLessEq ||
		token == tGreater || token == tGreaterEq {
		op := token
		nextToken()
		typRight := addExpr()
		typ = genBinary(op, typ, typRight)
	}
	return typ
}

func andExpr() int {
	typ := comparisonExpr()
	for token == tAnd {
		op := token
		nextToken()
		typRight := comparisonExpr()
		typ = genBinary(op, typ, typRight)
	}
	return typ
}

func orExpr() int {
	typ := andExpr()
	for token == tOr {
		op := token
		nextToken()
		typRight := andExpr()
		typ = genBinary(op, typ, typRight)
	}
	return typ
}

func Expression() int {
	return orExpr()
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
		if typeName == "int" || typeName == "bool" {
			return typeSliceInt
		} else if typeName == "string" {
			return typeSliceString
		} else {
			error("only []int and []string are supported")
		}
	}
	typeName := strToken
	identifier("\"int\" or \"string\"")
	if typeName == "int" || typeName == "bool" {
		return typeInt
	} else if typeName == "string" {
		return typeString
	} else {
		error("only int and string are supported")
	}
	return typeVoid
}

func defineLocal(typ int, name string) {
	locals = append(locals, name)
	localTypes = append(localTypes, typ)
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

func SimpleStmt() {
	// Funky parsing here to handle assignments
	if token == tIdent {
		identName := strToken
		nextToken()
		if token == tAssign {
			nextToken()
			Expression() // TODO: check that LHS type = RHS type
			genAssign(identName)
		} else if token == tDeclAssign {
			nextToken()
			typ := Expression()
			defineLocal(typ, identName)
			genAssign(identName)
		} else if token == tLParen {
			genIdentifier(identName)
			Arguments()
		} else if token == tLBracket {
			nextToken()
			indexExpr()
			expect(tRBracket, "]")
			expect(tAssign, "=")
			Expression()
			genSliceAssign(identName)
		} else {
			genIdentifier(identName)
		}
	} else {
		typ := Expression()
		genDiscard(typ)
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

func newLabel() string {
	labelNum = labelNum + 1
	return "label" + intStr(labelNum)
}

func IfStmt() {
	expect(tIf, "\"if\"")
	Expression()
	ifLabel := newLabel()
	genJumpIfZero(ifLabel) // jump to else or end of if block
	Block()
	if token == tElse {
		nextToken()
		elseLabel := newLabel()
		genJump(elseLabel) // jump past else block
		genLabel(ifLabel)
		if token == tIf {
			IfStmt()
		} else {
			Block()
		}
		genLabel(elseLabel)
	} else {
		genLabel(ifLabel)
	}
}

func ForStmt() {
	expect(tFor, "\"for\"")
	loopLabel := newLabel()
	genLabel(loopLabel) // top of loop
	Expression()
	doneLabel := newLabel()
	genJumpIfZero(doneLabel) // jump to after loop if done
	Block()
	genJump(loopLabel) // go back to top of loop
	genLabel(doneLabel)
}

func Declaration() {
	// We don't support ConstDecl or TypeDecl
	VarDecl()
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

// TODO: remove
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

// TODO: remove
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
	// Builtin: func print(s string)
	funcs = append(funcs, "print")
	funcSigIndexes = append(funcSigIndexes, len(funcSigs))
	funcSigs = append(funcSigs, typeVoid)
	funcSigs = append(funcSigs, 1)
	funcSigs = append(funcSigs, typeString)

	// TODO: define these in genProgramStart

	// Builtin: func printError(s string)
	funcs = append(funcs, "printError")
	funcSigIndexes = append(funcSigIndexes, len(funcSigs))
	funcSigs = append(funcSigs, typeVoid)
	funcSigs = append(funcSigs, 1)
	funcSigs = append(funcSigs, typeString)

	// Builtin: func readByte() int
	funcs = append(funcs, "readByte")
	funcSigIndexes = append(funcSigIndexes, len(funcSigs))
	funcSigs = append(funcSigs, typeInt)
	funcSigs = append(funcSigs, 0)

	// Builtin: func exit(code int)
	funcs = append(funcs, "exit")
	funcSigIndexes = append(funcSigIndexes, len(funcSigs))
	funcSigs = append(funcSigs, typeVoid)
	funcSigs = append(funcSigs, 1)
	funcSigs = append(funcSigs, typeInt)

	// Builtin: func charStr(ch int) string
	funcs = append(funcs, "charStr")
	funcSigIndexes = append(funcSigIndexes, len(funcSigs))
	funcSigs = append(funcSigs, typeString)
	funcSigs = append(funcSigs, 1)
	funcSigs = append(funcSigs, typeInt)

	// Builtin: func len(s stringOrSlice) int
	funcs = append(funcs, "len")
	funcSigIndexes = append(funcSigIndexes, len(funcSigs))
	funcSigs = append(funcSigs, typeInt)
	funcSigs = append(funcSigs, 1)
	funcSigs = append(funcSigs, typeString)

	funcs = append(funcs, "_lenString")
	funcSigIndexes = append(funcSigIndexes, len(funcSigs))
	funcSigs = append(funcSigs, typeInt)
	funcSigs = append(funcSigs, 1)
	funcSigs = append(funcSigs, typeString)

	funcs = append(funcs, "_lenSlice")
	funcSigIndexes = append(funcSigIndexes, len(funcSigs))
	funcSigs = append(funcSigs, typeInt)
	funcSigs = append(funcSigs, 1)
	funcSigs = append(funcSigs, typeSliceInt) // works with typeSliceString too

	// Builtin: func int(x int) int
	funcs = append(funcs, "int")
	funcSigIndexes = append(funcSigIndexes, len(funcSigs))
	funcSigs = append(funcSigs, typeInt)
	funcSigs = append(funcSigs, 1)
	funcSigs = append(funcSigs, typeInt)

	// Builtin: func append(s slice) slice
	funcs = append(funcs, "append")
	funcSigIndexes = append(funcSigIndexes, len(funcSigs))
	funcSigs = append(funcSigs, typeSliceInt) // not the real type
	funcSigs = append(funcSigs, 2)
	funcSigs = append(funcSigs, typeSliceInt)
	funcSigs = append(funcSigs, typeInt)

	funcs = append(funcs, "_appendInt")
	funcSigIndexes = append(funcSigIndexes, len(funcSigs))
	funcSigs = append(funcSigs, typeSliceInt)
	funcSigs = append(funcSigs, 2)
	funcSigs = append(funcSigs, typeSliceInt)
	funcSigs = append(funcSigs, typeInt)

	funcs = append(funcs, "_appendString")
	funcSigIndexes = append(funcSigIndexes, len(funcSigs))
	funcSigs = append(funcSigs, typeSliceString)
	funcSigs = append(funcSigs, 2)
	funcSigs = append(funcSigs, typeSliceString)
	funcSigs = append(funcSigs, typeString)

	// Builtin: func Expression() int -- TODO hack for forward reference
	funcs = append(funcs, "Expression")
	funcSigIndexes = append(funcSigIndexes, len(funcSigs))
	funcSigs = append(funcSigs, typeInt)
	funcSigs = append(funcSigs, 0)

	// Builtin: func Block() -- TODO hack for forward reference
	funcs = append(funcs, "Block")
	funcSigIndexes = append(funcSigIndexes, len(funcSigs))
	funcSigs = append(funcSigs, typeVoid)
	funcSigs = append(funcSigs, 0)

	genProgramStart()

	line = 1
	nextChar()
	nextToken()
	SourceFile()

	genDataSections()
}
