// Mugo: compiler for a (micro) subset of Go

package main

var (
	// Lexer variables
	c    int // current lexer byte
	line int // current line and column
	col  int

	// Parser-compiler variables
	token          int      // current parser token
	tokenInt       int      // integer value of current token (if applicable)
	tokenStr       string   // string value of current token (if applicable)
	curFunc        string   // current function name, or "" if not in a func
	tokens         []string // token names
	types          []string // type names
	typeSizes      []int    // type sizes in bytes
	labelNum       int      // current label number
	consts         []string // constant names
	globals        []string // global names and types
	globalTypes    []int
	locals         []string // local names and types
	localTypes     []int
	funcs          []string // function names
	funcSigIndexes []int    // indexes into funcSigs
	funcSigs       []int    // for each func: retType N arg1Type ... argNType
	strs           []string // string constants
)

const (
	localSpace int = 64      // max space for locals declared with := (not arguments)
	heapSize   int = 1048576 // 1MB "heap"

	// Types
	typeVoid     int = 1 // only used as return "type"
	typeInt      int = 2
	typeString   int = 3
	typeSliceInt int = 4
	typeSliceStr int = 5

	// Keywords
	tIf      int = 1
	tElse    int = 2
	tFor     int = 3
	tVar     int = 4
	tConst   int = 5
	tFunc    int = 6
	tReturn  int = 7
	tPackage int = 8

	// Literals, identifiers, and EOF
	tIntLit int = 9
	tStrLit int = 10
	tIdent  int = 11
	tEOF    int = 12

	// Two-character tokens
	tOr         int = 13
	tAnd        int = 14
	tEq         int = 15
	tNotEq      int = 16
	tLessEq     int = 17
	tGreaterEq  int = 18
	tDeclAssign int = 19

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
)

// Lexer

func nextChar() {
	if c == '\n' {
		line = line + 1
		col = 0
	}
	c = getc()
	col = col + 1
}

func itoa(n int) string {
	if n < 0 {
		return "-" + itoa(-n)
	}
	if n < 10 {
		return char(n + '0')
	}
	return itoa(n/10) + itoa(n%10)
}

func error(msg string) {
	log("\n" + itoa(line) + ":" + itoa(col) + ": " + msg + "\n")
	exit(1)
}

func isDigit(ch int) bool {
	return ch >= '0' && ch <= '9'
}

func isAlpha(ch int) bool {
	return ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z'
}

func find(names []string, name string) int {
	i := 0
	for i < len(names) {
		if names[i] == name {
			return i
		}
		i = i + 1
	}
	return -1
}

func expectChar(ch int) {
	if c != ch {
		error("expected '" + char(ch) + "' not '" + char(c) + "'")
	}
	nextChar()
}

func tokenChoice(oneCharToken int, secondCh int, twoCharToken int) {
	nextChar()
	if c == secondCh {
		nextChar()
		token = twoCharToken
	} else {
		token = oneCharToken
	}
}

func next() {
	// Skip whitespace and comments, and look for / operator
	for c == '/' || c == ' ' || c == '\t' || c == '\r' || c == '\n' {
		if c == '/' {
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
		} else if c == '\n' {
			nextChar()
			// Semicolon insertion: golang.org/ref/spec#Semicolons
			if token == tIdent || token == tIntLit || token == tStrLit ||
				token == tReturn || token == tRParen ||
				token == tRBracket || token == tRBrace {
				token = tSemicolon
				return
			}
		} else {
			nextChar()
		}
	}
	if c < 0 {
		// End of file
		token = tEOF
		return
	}

	// Integer literal
	if isDigit(c) {
		tokenInt = c - '0'
		nextChar()
		for isDigit(c) {
			tokenInt = tokenInt*10 + c - '0'
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
				tokenInt = '\''
			} else if c == '\\' {
				tokenInt = '\\'
			} else if c == 't' {
				tokenInt = '\t'
			} else if c == 'r' {
				tokenInt = '\r'
			} else if c == 'n' {
				tokenInt = '\n'
			} else {
				error("unexpected escape '\\" + char(c) + "'")
			}
			nextChar()
		} else {
			tokenInt = c
			nextChar()
		}
		expectChar('\'')
		token = tIntLit
		return
	}

	// String literal
	if c == '"' {
		nextChar()
		tokenStr = ""
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
					error("unexpected escape \"\\" + char(c) + "\"")
				}
			}
			tokenStr = tokenStr + char(c)
			nextChar()
		}
		expectChar('"')
		token = tStrLit
		return
	}

	// Keyword or identifier
	if isAlpha(c) || c == '_' {
		tokenStr = char(c)
		nextChar()
		for isAlpha(c) || isDigit(c) || c == '_' {
			tokenStr = tokenStr + char(c)
			nextChar()
		}
		index := find(tokens, tokenStr)
		if index >= tIf && index <= tPackage {
			// Keyword
			token = index
		} else {
			// Otherwise it's an identifier
			token = tIdent
		}
		return
	}

	// Single-character tokens (token is ASCII value)
	if c == '+' || c == '-' || c == '*' || c == '%' || c == ';' ||
		c == ',' || c == '(' || c == ')' || c == '{' || c == '}' ||
		c == '[' || c == ']' {
		token = c
		nextChar()
		return
	}

	// One or two-character tokens
	if c == '=' {
		tokenChoice(tAssign, '=', tEq)
		return
	} else if c == '<' {
		tokenChoice(tLess, '=', tLessEq)
		return
	} else if c == '>' {
		tokenChoice(tGreater, '=', tGreaterEq)
		return
	} else if c == '!' {
		tokenChoice(tNot, '=', tNotEq)
		return
	} else if c == ':' {
		tokenChoice(tColon, '=', tDeclAssign)
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

	error("unexpected '" + char(c) + "'")
}

// Escape given string; use "delim" as quote character.
func escape(s string, delim string) string {
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
			quoted = quoted + char(int(s[i]))
		}
		i = i + 1
	}
	return quoted + delim
}

func tokenName(t int) string {
	if t > ' ' {
		return char(t)
	}
	return tokens[t]
}

// Code generator functions

func genProgramStart() {
	print("global _start\n")
	print("section .text\n")
	print("\n")

	// Initialize and call main.
	print("_start:\n")
	print("xor rax, rax\n") // ensure heap is zeroed
	print("mov rdi, _heap\n")
	print("mov rcx, " + itoa(heapSize/8) + "\n")
	print("rep stosq\n")
	print("mov rax, _heap\n")
	print("mov [_heapPtr], rax\n")
	print("call main\n")
	print("mov rax, 60\n") // system call for "exit"
	print("mov rdi, 0\n")  // exit code 0
	print("syscall\n")
	print("\n")

	// Write a string to stdout.
	print("print:\n")
	print("push rbp\n") // rbp ret addr len
	print("mov rbp, rsp\n")
	print("mov rax, 1\n")        // system call for "write"
	print("mov rdi, 1\n")        // file handle 1 is stdout
	print("mov rsi, [rbp+16]\n") // address
	print("mov rdx, [rbp+24]\n") // length
	print("syscall\n")
	print("pop rbp\n")
	print("ret 16\n")
	print("\n")

	// Write a string to stderr.
	print("log:\n")
	print("push rbp\n") // rbp ret addr len
	print("mov rbp, rsp\n")
	print("mov rax, 1\n")        // system call for "write"
	print("mov rdi, 2\n")        // file handle 2 is stderr
	print("mov rsi, [rbp+16]\n") // address
	print("mov rdx, [rbp+24]\n") // length
	print("syscall\n")
	print("pop rbp\n")
	print("ret 16\n")
	print("\n")

	// Read a single byte from stdin, or return -1 on EOF.
	print("getc:\n")
	print("push qword 0\n")
	print("mov rax, 0\n")   // system call for "read"
	print("mov rdi, 0\n")   // file handle 0 is stdin
	print("mov rsi, rsp\n") // address
	print("mov rdx, 1\n")   // length
	print("syscall\n")
	print("cmp rax, 1\n")
	print("je _getc1\n")
	print("mov qword [rsp], -1\n")
	print("_getc1:\n")
	print("pop rax\n")
	print("ret\n")
	print("\n")

	// Like os.Exit().
	print("exit:\n")
	print("mov rdi, [rsp+8]\n") // code
	print("mov rax, 60\n")      // system call for "exit"
	print("syscall\n")
	print("\n")

	// No-op int() for use in escape(), to satisfy Go's type checker.
	print("int:\n")
	print("mov rax, [rsp+8]\n") // value
	print("ret 8\n")
	print("\n")

	// Return concatenation of two strings.
	print("_strAdd:\n")
	print("push rbp\n") // rbp ret addr1 len1 addr0 len0
	print("mov rbp, rsp\n")
	// Allocate len0+len1 bytes
	print("mov rax, [rbp+24]\n") // len1
	print("add rax, [rbp+40]\n") // len1 + len0
	print("push rax\n")
	print("call _alloc\n")
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
	print("ret 32\n")
	print("\n")

	// Return true if strings are equal.
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
	print("ret 32\n")
	// Return addrNew len0+len1 (addrNew already in rax)
	print("_strEqNotEqual:\n")
	print("xor rax, rax\n")
	print("pop rbp\n")
	print("ret 32\n")
	print("\n")

	// Return new 1-byte string from integer character.
	print("char:\n")
	print("push rbp\n") // rbp ret ch
	print("mov rbp, rsp\n")
	// Allocate 1 byte
	print("push 1\n")
	print("call _alloc\n")
	// Move byte to destination
	print("mov rbx, [rbp+16]\n")
	print("mov [rax], bl\n")
	// Return addrNew 1 (addrNew already in rax)
	print("mov rbx, 1\n")
	print("pop rbp\n")
	print("ret 8\n")
	print("\n")

	// Simple bump allocator (with no GC!). Takes allocation size in bytes,
	// returns pointer to allocated memory.
	print("_alloc:\n")
	print("push rbp\n") // rbp ret size
	print("mov rbp, rsp\n")
	print("mov rax, [_heapPtr]\n")
	print("mov rbx, [rbp+16]\n")
	print("add rbx, [_heapPtr]\n")
	print("cmp rbx, _heapEnd\n")
	print("jg _outOfMem\n")
	print("mov [_heapPtr], rbx\n")
	print("pop rbp\n")
	print("ret 8\n")
	print("_outOfMem:\n")
	print("push qword 14\n") // len("out of memory\n")
	print("push _strOutOfMem\n")
	print("call log\n")
	print("push qword 1\n")
	print("call exit\n")
	print("\n")

	// Append single integer to []int, allocating and copying as necessary.
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
	print("ret 32\n")
	print("\n")

	// Append single string to []string, allocating and copying as necessary.
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
	print("ret 40\n")

	// Return string length
	print("len:\n")
	print("push rbp\n") // rbp ret addr len
	print("mov rbp, rsp\n")
	print("mov rax, [rbp+24]\n")
	print("pop rbp\n")
	print("ret 16\n")
	print("\n")

	// Return slice length
	print("_lenSlice:\n")
	print("push rbp\n") // rbp ret addr len cap
	print("mov rbp, rsp\n")
	print("mov rax, [rbp+24]\n")
	print("pop rbp\n")
	print("ret 24\n")
	print("\n")
}

func genConst(name string, value int) {
	print(name + " equ " + itoa(value) + "\n")
}

func genIntLit(n int) {
	print("push qword " + itoa(n) + "\n")
}

func genStrLit(s string) {
	// Add string to strs and strAddrs tables
	index := find(strs, s)
	if index < 0 {
		// Haven't seen this string constant before, add a new one
		index = len(strs)
		strs = append(strs, s)
	}
	// Push string struct: length and then address (by label)
	print("push qword " + itoa(len(s)) + "\n")
	print("push qword str" + itoa(index) + "\n")
}

func typeName(typ int) string {
	return types[typ]
}

func typeSize(typ int) int {
	return typeSizes[typ]
}

// Return offset of local variable from rbp (including arguments).
func localOffset(index int) int {
	funcIndex := find(funcs, curFunc)
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
	genFetchInstrs(typ, "rbp+"+itoa(offset))
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
	localIndex := find(locals, name)
	if localIndex >= 0 {
		return genLocalFetch(localIndex)
	}
	globalIndex := find(globals, name)
	if globalIndex >= 0 {
		return genGlobalFetch(globalIndex)
	}
	constIndex := find(consts, name)
	if constIndex >= 0 {
		return genConstFetch(constIndex)
	}
	funcIndex := find(funcs, name)
	if funcIndex >= 0 {
		sigIndex := funcSigIndexes[funcIndex]
		return funcSigs[sigIndex] // result type
	}
	error("identifier " + escape(name, "\"") + " not defined")
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
	genAssignInstrs(localTypes[index], "rbp+"+itoa(offset))
}

func genGlobalAssign(index int) {
	name := globals[index]
	genAssignInstrs(globalTypes[index], name)
}

func genAssign(name string) {
	localIndex := find(locals, name)
	if localIndex >= 0 {
		genLocalAssign(localIndex)
		return
	}
	globalIndex := find(globals, name)
	if globalIndex >= 0 {
		genGlobalAssign(globalIndex)
		return
	}
	error("identifier " + escape(name, "\"") + " not defined (or not assignable)")
}

func varType(name string) int {
	localIndex := find(locals, name)
	if localIndex >= 0 {
		return localTypes[localIndex]
	}
	globalIndex := find(globals, name)
	if globalIndex >= 0 {
		return globalTypes[globalIndex]
	}
	error("identifier " + escape(name, "\"") + " not defined")
	return 0
}

func genSliceAssign(name string) {
	typ := varType(name)
	print("pop rax\n") // value (addr if string type)
	if typ == typeSliceStr {
		print("pop rbx\n") // value (len)
		print("pop rcx\n") // index * 2
		print("add rcx, rcx\n")
	} else {
		print("pop rcx\n")
	}
	localIndex := find(locals, name)
	if localIndex >= 0 {
		offset := localOffset(localIndex)
		print("mov rdx, [rbp+" + itoa(offset) + "]\n")
	} else {
		print("mov rdx, [" + name + "]\n")
	}
	print("mov [rdx+rcx*8], rax\n")
	if typ == typeSliceStr {
		print("mov [rdx+rcx*8+8], rbx\n")
	}
}

func genCall(name string) int {
	print("call " + name + "\n")
	index := find(funcs, name)
	sigIndex := funcSigIndexes[index]
	resultType := funcSigs[sigIndex]
	if resultType == typeInt {
		print("push rax\n")
	} else if resultType == typeString {
		print("push rbx\n")
		print("push rax\n")
	} else if resultType == typeSliceInt || resultType == typeSliceStr {
		print("push rcx\n")
		print("push rbx\n")
		print("push rax\n")
	}
	return resultType
}

func genFuncStart(name string) {
	print("\n")
	print(name + ":\n")
	print("push rbp\n")
	print("mov rbp, rsp\n")
	print("sub rsp, " + itoa(localSpace) + "\n") // space for locals
}

// Return size (in bytes) of current function's arguments.
func argsSize() int {
	i := find(funcs, curFunc)
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

// Return size (in bytes) of current function's locals (excluding arguments).
func localsSize() int {
	i := find(funcs, curFunc)
	sigIndex := funcSigIndexes[i]
	numArgs := funcSigs[sigIndex+1]
	size := 0
	i = numArgs
	for i < len(locals) {
		size = size + typeSize(localTypes[i])
		i = i + 1
	}
	return size
}

func genFuncEnd() {
	size := localsSize()
	if size > localSpace {
		error(curFunc + "'s locals too big (" + itoa(size) + " > " + itoa(localSpace) + ")\n")
	}
	print("mov rsp, rbp\n")
	print("pop rbp\n")
	size = argsSize()
	if size > 0 {
		print("ret " + itoa(size) + "\n")
	} else {
		print("ret\n")
	}
}

func genDataSections() {
	print("\n")
	print("section .data\n")
	print("_strOutOfMem: db `out of memory\\n`\n")

	// String constants
	i := 0
	for i < len(strs) {
		print("str" + itoa(i) + ": db " + escape(strs[i], "`") + "\n")
		i = i + 1
	}

	// Global variables
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

	// "Heap" (used for strings and slice appends)
	print("\n")
	print("section .bss\n")
	print("_heapPtr: resq 1\n")
	print("_heap: resb " + itoa(heapSize) + "\n")
	print("_heapEnd:\n")
}

func genUnary(op int, typ int) {
	if typ != typeInt {
		error("unary operator not allowed on type " + typeName(typ))
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
		print("push rbx\n")
		print("push rax\n")
		return typeString
	} else if op == tEq {
		print("call _strEq\n")
		print("push rax\n")
		return typeInt
	} else if op == tNotEq {
		print("call _strEq\n")
		print("cmp rax, 0\n")
		print("mov rax, 0\n")
		print("setz al\n")
		print("push rax\n")
		return typeInt
	} else {
		error("operator " + tokenName(op) + " not allowed on strings")
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
		print("cqo\n")
		print("idiv rbx\n")
	} else if op == tModulo {
		print("cqo\n")
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
	} else if typ == typeSliceInt || typ == typeSliceStr {
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
		print("add rsp, " + itoa(typeSize(typ)) + "\n")
	}
}

func genSliceExpr() {
	// Slice expression of form slice[:max]
	print("pop rax\n")  // max
	print("pop rbx\n")  // addr
	print("pop rcx\n")  // old length (capacity remains same)
	print("push rax\n") // new length
	print("push rbx\n") // addr remains same
}

func genSliceFetch(typ int) int {
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
	} else if typ == typeSliceStr {
		print("pop rax\n") // index
		print("pop rbx\n") // addr
		print("pop rcx\n") // len
		print("pop rdx\n") // cap
		print("add rax, rax\n")
		print("push qword [rbx+rax*8+8]\n")
		print("push qword [rbx+rax*8]\n")
		return typeString
	} else {
		error("invalid slice type " + typeName(typ))
		return 0
	}
}

// Recursive-descent parser

func expect(expected int, msg string) {
	if token != expected {
		error("expected " + msg + " not " + tokenName(token))
	}
	next()
}

func Literal() int {
	if token == tIntLit {
		genIntLit(tokenInt)
		next()
		return typeInt
	} else if token == tStrLit {
		genStrLit(tokenStr)
		next()
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
		name := tokenStr
		identifier("identifier")
		return genIdentifier(name)
	} else {
		error("expected literal or identifier")
		return 0
	}
}

func ExpressionList() int {
	firstType := Expression()
	for token == tComma {
		next()
		Expression()
	}
	return firstType
}

func Arguments() int {
	funcName := tokenStr // function name will still be in tokenStr
	expect(tLParen, "(")
	arg1Type := typeVoid
	if token != tRParen {
		arg1Type = ExpressionList()
	}
	expect(tRParen, ")")

	// Replace "generic" built-in functions with type-specific versions
	if funcName == "append" {
		if arg1Type == typeSliceInt {
			funcName = "_appendInt"
		} else if arg1Type == typeSliceStr {
			funcName = "_appendString"
		} else {
			error("can't append to " + typeName(arg1Type))
		}
	} else if funcName == "len" {
		if arg1Type == typeString {
			funcName = "len"
		} else if arg1Type == typeSliceInt || arg1Type == typeSliceStr {
			funcName = "_lenSlice"
		} else {
			error("can't get length of " + typeName(arg1Type))
		}
	}
	return genCall(funcName)
}

func indexExpr() {
	typ := Expression()
	if typ != typeInt {
		error("slice index must be int")
	}
}

func PrimaryExpr() int {
	typ := Operand()
	if token == tLParen {
		return Arguments()
	} else if token == tLBracket {
		next()
		if token == tColon {
			if typ != typeSliceInt && typ != typeSliceStr {
				error("slice expression requires slice type")
			}
			next()
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
		next()
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
		next()
		typRight := UnaryExpr()
		typ = genBinary(op, typ, typRight)
	}
	return typ
}

func addExpr() int {
	typ := mulExpr()
	for token == tPlus || token == tMinus {
		op := token
		next()
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
		next()
		typRight := addExpr()
		typ = genBinary(op, typ, typRight)
	}
	return typ
}

func andExpr() int {
	typ := comparisonExpr()
	for token == tAnd {
		op := token
		next()
		typRight := comparisonExpr()
		typ = genBinary(op, typ, typRight)
	}
	return typ
}

func orExpr() int {
	typ := andExpr()
	for token == tOr {
		op := token
		next()
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
	if token == tLBracket {
		next()
		expect(tRBracket, "]")
		name := tokenStr
		identifier("\"int\" or \"string\"")
		if name == "int" || name == "bool" {
			return typeSliceInt
		} else if name == "string" {
			return typeSliceStr
		} else {
			error("only []int and []string are supported")
		}
	}
	name := tokenStr
	identifier("\"int\" or \"string\"")
	if name == "int" || name == "bool" {
		return typeInt
	} else if name == "string" {
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
	varName := tokenStr
	identifier("variable identifier")
	typ := Type()
	if curFunc != "" {
		error("\"var\" not supported inside functions")
	}
	globals = append(globals, varName)
	globalTypes = append(globalTypes, typ)
	if token == tAssign {
		error("assignment not supported for top-level var")
	}
}

func VarDecl() {
	expect(tVar, "\"var\"")
	expect(tLParen, "(")
	for token != tRParen {
		VarSpec()
		expect(tSemicolon, ";")
	}
	expect(tRParen, ")")
}

func ConstSpec() {
	// We only support typed integer constants
	name := tokenStr
	consts = append(consts, name)
	identifier("variable identifier")
	typ := Type()
	if typ != typeInt {
		error("constants must be typed int")
	}
	expect(tAssign, "=")
	value := tokenInt
	expect(tIntLit, "integer literal")
	genConst(name, value)
}

func ConstDecl() {
	expect(tConst, "\"const\"")
	expect(tLParen, "(")
	for token != tRParen {
		ConstSpec()
		expect(tSemicolon, ";")
	}
	expect(tRParen, ")")
}

func ParameterDecl() {
	paramName := tokenStr
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
		next()
		ParameterDecl()
	}
}

func Parameters() {
	expect(tLParen, "(")
	if token != tRParen {
		ParameterList()
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
	identName := tokenStr
	expect(tIdent, "assignment or call statement")
	if token == tAssign {
		next()
		lhsType := varType(identName)
		rhsType := Expression()
		if lhsType != rhsType {
			error("can't assign " + typeName(rhsType) + " to " +
				typeName(lhsType))
		}
		genAssign(identName)
	} else if token == tDeclAssign {
		next()
		typ := Expression()
		defineLocal(typ, identName)
		genAssign(identName)
	} else if token == tLParen {
		genIdentifier(identName)
		typ := Arguments()
		genDiscard(typ) // discard return value
	} else if token == tLBracket {
		next()
		indexExpr()
		expect(tRBracket, "]")
		expect(tAssign, "=")
		Expression()
		genSliceAssign(identName)
	} else {
		error("expected assignment or call not " + tokenName(token))
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
	return "label" + itoa(labelNum)
}

func IfStmt() {
	expect(tIf, "\"if\"")
	Expression()
	ifLabel := newLabel()
	genJumpIfZero(ifLabel) // jump to else or end of if block
	Block()
	if token == tElse {
		next()
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

func Statement() {
	if token == tIf {
		IfStmt()
	} else if token == tFor {
		ForStmt()
	} else if token == tReturn {
		ReturnStmt()
	} else {
		SimpleStmt()
	}
}

func StatementList() {
	for token != tRBrace {
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
	curFunc = tokenStr
	genFuncStart(tokenStr)
	funcs = append(funcs, tokenStr)
	funcSigIndexes = append(funcSigIndexes, len(funcSigs))
	identifier("function name")
	Signature()
	FunctionBody()
	genFuncEnd()
	locals = locals[:0]
	localTypes = localTypes[:0]
	curFunc = ""
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

func addFunc(name string, resultType int, numArgs int, arg1Type int, arg2Type int) {
	funcs = append(funcs, name)
	funcSigIndexes = append(funcSigIndexes, len(funcSigs))
	funcSigs = append(funcSigs, resultType)
	funcSigs = append(funcSigs, numArgs)
	if numArgs > 0 {
		funcSigs = append(funcSigs, arg1Type)
	}
	if numArgs > 1 {
		funcSigs = append(funcSigs, arg2Type)
	}
}

func addToken(name string) {
	tokens = append(tokens, name)
}

func addType(name string, size int) {
	types = append(types, name)
	typeSizes = append(typeSizes, size)
}

// Test constructs not used in compiler itself.
var (
	testSlice []string
)

func testAppend(sl []string, s string) []string {
	return append(testSlice, s)
}

func testUnused() {
	sl := testSlice
	sl = testAppend(sl, "one") // test returning a slice
	sl[0] = "two"
	if sl[0] != "two" {
		error("fail: string slice assignment")
	}
	t := 0 == 0
	f := 0 == 1
	if !t || !!f {
		error("fail: not operator")
	}
}

func main() {
	// Builtin functions (defined in genProgramStart; Go versions in gofuncs.go)
	addFunc("print", typeVoid, 1, typeString, 0)
	addFunc("log", typeVoid, 1, typeString, 0)
	addFunc("getc", typeInt, 0, 0, 0)
	addFunc("exit", typeVoid, 1, typeInt, 0)
	addFunc("char", typeString, 1, typeInt, 0)
	addFunc("len", typeInt, 1, typeString, 0)
	addFunc("_lenSlice", typeInt, 1, typeSliceInt, 0) // works with typeSliceStr too
	addFunc("int", typeInt, 1, typeInt, 0)
	addFunc("append", typeSliceInt, 2, typeSliceInt, typeInt)
	addFunc("_appendInt", typeSliceInt, 2, typeSliceInt, typeInt)
	addFunc("_appendString", typeSliceStr, 2, typeSliceStr, typeString)

	// Forward references
	addFunc("Expression", typeInt, 0, 0, 0)
	addFunc("Block", typeVoid, 0, 0, 0)

	// Token names
	addToken("") // token 0 is not valid
	addToken("if")
	addToken("else")
	addToken("for")
	addToken("var")
	addToken("const")
	addToken("func")
	addToken("return")
	addToken("package")
	addToken("integer")
	addToken("string")
	addToken("identifier")
	addToken("EOF")
	addToken("||")
	addToken("&&")
	addToken("==")
	addToken("!=")
	addToken("<=")
	addToken(">=")
	addToken(":=")

	// Type names and sizes
	addType("", 0) // type 0 is not valid
	addType("void", 0)
	addType("int", 8)
	addType("string", 16)
	addType("[]int", 24)
	addType("[]string", 24)

	testUnused()

	genProgramStart()

	line = 1
	col = 0
	nextChar()
	next()
	SourceFile()

	genDataSections()
}
