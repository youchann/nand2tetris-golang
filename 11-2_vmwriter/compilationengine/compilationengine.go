package compilationengine

import (
	"slices"
	"strconv"

	"github.com/youchann/nand2tetris/11-2_vmwriter/symboltable"
	"github.com/youchann/nand2tetris/11-2_vmwriter/token"
	"github.com/youchann/nand2tetris/11-2_vmwriter/tokenizer"
	"github.com/youchann/nand2tetris/11-2_vmwriter/vmwriter"
)

var kindSegmentMap = map[symboltable.Kind]vmwriter.Segment{
	symboltable.STATIC:    vmwriter.STATIC,
	symboltable.FIELD:     vmwriter.THIS,
	symboltable.ARGUMENT:  vmwriter.ARGUMENT,
	symboltable.VAR_LOCAL: vmwriter.LOCAL,
}

type CompilationEngine struct {
	className    string
	labelCount   int
	tokenizer    *tokenizer.JackTokenizer
	vmwriter     *vmwriter.VMWriter
	classST      *symboltable.SymbolTable
	subroutineST *symboltable.SymbolTable
}

func New(n string, t *tokenizer.JackTokenizer, w *vmwriter.VMWriter) *CompilationEngine {
	return &CompilationEngine{
		className:    n,
		labelCount:   0,
		tokenizer:    t,
		vmwriter:     w,
		classST:      symboltable.New(),
		subroutineST: symboltable.New(),
	}
}

func (ce *CompilationEngine) CompileClass() {
	ce.process("class")

	// className
	name := ce.tokenizer.CurrentToken().Literal
	if ce.tokenizer.CurrentToken().Type != token.IDENTIFIER {
		panic("expected identifier but got " + name)
	} else if name != ce.className {
		panic("class name does not match file name")
	}
	ce.tokenizer.Advance()

	ce.process("{")
	ce.compileClassVarDec()
	ce.compileSubroutine()
	ce.process("}")
}

func (ce *CompilationEngine) compileClassVarDec() {
	for ce.tokenizer.CurrentToken().Literal == "static" || ce.tokenizer.CurrentToken().Literal == "field" {
		// static or field
		kind := ce.process(ce.tokenizer.CurrentToken().Literal)
		// type
		typ := ce.processType()

		// varName
		for ce.tokenizer.CurrentToken().Literal != ";" {
			name := ce.tokenizer.CurrentToken().Literal
			if ce.tokenizer.CurrentToken().Type != token.IDENTIFIER {
				panic("expected identifier but got " + name)
			}
			ce.classST.Define(name, typ, symboltable.KindMap[kind])
			ce.tokenizer.Advance()
			if ce.tokenizer.CurrentToken().Literal == "," {
				ce.process(",")
			}
		}

		ce.process(";")
	}
}

func (ce *CompilationEngine) compileSubroutine() {
	subroutineType := []token.Keyword{token.CONSTRUCTOR, token.FUNCTION, token.METHOD}
	for slices.Contains(subroutineType, token.Keyword(ce.tokenizer.CurrentToken().Literal)) {
		ce.subroutineST.Reset()

		// constructor, function, or method
		methodType := ce.tokenizer.CurrentToken().Literal
		ce.process(methodType)

		// void or type
		voidOrType := []token.Keyword{token.VOID, token.INT, token.CHAR, token.BOOLEAN}
		if !slices.Contains(voidOrType, token.Keyword(ce.tokenizer.CurrentToken().Literal)) && ce.tokenizer.CurrentToken().Type != token.IDENTIFIER {
			panic("expected type or void but got " + ce.tokenizer.CurrentToken().Literal)
		}
		ce.tokenizer.Advance()

		// subroutineName
		name := ce.tokenizer.CurrentToken().Literal
		if ce.tokenizer.CurrentToken().Type != token.IDENTIFIER {
			panic("expected identifier but got " + name)
		}
		ce.tokenizer.Advance()

		if token.Keyword(methodType) == token.METHOD {
			ce.subroutineST.Define("this", ce.className, symboltable.ARGUMENT)
		}

		ce.process("(")
		ce.compileParameterList()
		ce.process(")")

		ce.process("{")
		n := ce.compileVarDec()
		ce.vmwriter.WriteFunction(ce.className+"."+name, n)
		if token.Keyword(methodType) == token.CONSTRUCTOR {
			ce.vmwriter.WritePush(vmwriter.CONSTANT, ce.classST.VarCount(symboltable.FIELD))
			ce.vmwriter.WriteCall("Memory.alloc", 1)
			ce.vmwriter.WritePop(vmwriter.POINTER, 0)
		} else if token.Keyword(methodType) == token.METHOD {
			ce.vmwriter.WritePush(vmwriter.ARGUMENT, 0)
			ce.vmwriter.WritePop(vmwriter.POINTER, 0)
		}
		ce.compileStatements()
		ce.process("}")
	}
}

func (ce *CompilationEngine) compileParameterList() {
	for ce.tokenizer.CurrentToken().Literal != ")" {
		// type
		typ := ce.processType()

		// varName
		name := ce.tokenizer.CurrentToken().Literal
		if ce.tokenizer.CurrentToken().Type != token.IDENTIFIER {
			panic("expected identifier but got " + name)
		}
		ce.subroutineST.Define(name, typ, symboltable.ARGUMENT)
		ce.tokenizer.Advance()

		if ce.tokenizer.CurrentToken().Literal == "," {
			ce.process(",")
		}
	}
}

func (ce *CompilationEngine) compileVarDec() int {
	count := 0
	for ce.tokenizer.CurrentToken().Literal == "var" {
		ce.process("var")
		typ := ce.processType()

		// varName
		for ce.tokenizer.CurrentToken().Literal != ";" {
			name := ce.tokenizer.CurrentToken().Literal
			if ce.tokenizer.CurrentToken().Type != token.IDENTIFIER {
				panic("expected identifier but got " + name)
			}
			ce.subroutineST.Define(name, typ, symboltable.VAR_LOCAL)
			count++
			ce.tokenizer.Advance()

			if ce.tokenizer.CurrentToken().Literal == "," {
				ce.process(",")
			}
		}

		ce.process(";")
	}
	return count
}

func (ce *CompilationEngine) compileStatements() {
	statementPrefix := []token.Keyword{token.LET, token.IF, token.WHILE, token.DO, token.RETURN}
	for slices.Contains(statementPrefix, token.Keyword(ce.tokenizer.CurrentToken().Literal)) {
		switch token.Keyword(ce.tokenizer.CurrentToken().Literal) {
		case token.LET:
			ce.compileLet()
		case token.IF:
			ce.compileIf()
		case token.WHILE:
			ce.compileWhile()
		case token.DO:
			ce.compileDo()
		case token.RETURN:
			ce.compileReturn()
		}
	}
}

func (ce *CompilationEngine) compileLet() {
	ce.process("let")

	// varName
	name := ce.tokenizer.CurrentToken().Literal
	if ce.tokenizer.CurrentToken().Type != token.IDENTIFIER {
		panic("expected identifier but got " + name)
	}
	ce.tokenizer.Advance()

	if ce.tokenizer.CurrentToken().Literal == "[" {
		ce.process("[")
		ce.compileExpression()
		ce.process("]")
		ce.writePushVariable(name)
		ce.vmwriter.WriteArithmetic(vmwriter.ADD)
		ce.process("=")
		ce.compileExpression()
		ce.process(";")
		ce.vmwriter.WritePop(vmwriter.TEMP, 0)
		ce.vmwriter.WritePop(vmwriter.POINTER, 1)
		ce.vmwriter.WritePush(vmwriter.TEMP, 0)
		ce.vmwriter.WritePop(vmwriter.THAT, 0)
	} else {
		ce.process("=")
		ce.compileExpression()
		ce.process(";")
		ce.writePopVariable(name)
	}
}

func (ce *CompilationEngine) compileIf() {
	firstLabelName := ce.className + "_" + strconv.Itoa(ce.labelCount)
	secondLabelName := ce.className + "_" + strconv.Itoa(ce.labelCount+1)
	ce.labelCount += 2

	ce.process("if")
	ce.process("(")
	ce.compileExpression()
	ce.process(")")
	ce.vmwriter.WriteArithmetic(vmwriter.NOT)
	ce.vmwriter.WriteIf(secondLabelName)
	ce.process("{")
	ce.compileStatements()
	ce.process("}")
	ce.vmwriter.WriteGoto(firstLabelName)
	ce.vmwriter.WriteLabel(secondLabelName)
	if ce.tokenizer.CurrentToken().Literal == "else" {
		ce.process("else")
		ce.process("{")
		ce.compileStatements()
		ce.process("}")
	}
	ce.vmwriter.WriteLabel(firstLabelName)
}

func (ce *CompilationEngine) compileWhile() {
	firstLabelName := ce.className + "_" + strconv.Itoa(ce.labelCount)
	secondLabelName := ce.className + "_" + strconv.Itoa(ce.labelCount+1)
	ce.labelCount += 2

	ce.process("while")
	ce.process("(")
	ce.vmwriter.WriteLabel(firstLabelName)
	ce.compileExpression()
	ce.vmwriter.WriteArithmetic(vmwriter.NOT)
	ce.vmwriter.WriteIf(secondLabelName)
	ce.process(")")
	ce.process("{")
	ce.compileStatements()
	ce.vmwriter.WriteGoto(firstLabelName)
	ce.vmwriter.WriteLabel(secondLabelName)
	ce.process("}")
}

func (ce *CompilationEngine) compileDo() {
	args := 0
	ce.process("do")

	// subroutineCall
	name := ce.tokenizer.CurrentToken().Literal
	if ce.tokenizer.CurrentToken().Type != token.IDENTIFIER {
		panic("expected identifier but got " + name)
	}
	if ce.subroutineST.IndexOf(name) != -1 {
		ce.vmwriter.WritePush(kindSegmentMap[ce.subroutineST.KindOf(name)], ce.subroutineST.IndexOf(name))
		name = ce.subroutineST.TypeOf(name)
		args++
	} else if ce.classST.IndexOf(name) != -1 {
		ce.vmwriter.WritePush(kindSegmentMap[ce.classST.KindOf(name)], ce.classST.IndexOf(name))
		name = ce.classST.TypeOf(name)
		args++
	}
	ce.tokenizer.Advance()
	if ce.tokenizer.CurrentToken().Literal == "." {
		ce.process(".")
		n := ce.tokenizer.CurrentToken().Literal
		if ce.tokenizer.CurrentToken().Type != token.IDENTIFIER {
			panic("expected identifier but got " + n)
		}
		name += "." + n
		ce.tokenizer.Advance()
	} else {
		name = ce.className + "." + name
		ce.vmwriter.WritePush(vmwriter.POINTER, 0)
		args++
	}

	ce.process("(")
	args += ce.compileExpressionList()
	ce.process(")")
	ce.process(";")

	ce.vmwriter.WriteCall(name, args)
	ce.vmwriter.WritePop(vmwriter.TEMP, 0)
}

func (ce *CompilationEngine) compileReturn() {
	ce.process("return")
	if ce.tokenizer.CurrentToken().Literal == ";" {
		ce.vmwriter.WritePush(vmwriter.CONSTANT, 0)
	} else {
		ce.compileExpression()
	}
	ce.vmwriter.WriteReturn()
	ce.process(";")
}

func (ce *CompilationEngine) compileExpression() {
	ce.compileTerm()

	operand := []token.Symbol{token.PLUS, token.MINUS, token.ASTERISK, token.SLASH, token.AND, token.PIPE, token.LESS_THAN, token.GREATER_THAN, token.EQUAL}
	for slices.Contains(operand, token.Symbol(ce.tokenizer.CurrentToken().Literal)) {
		op := ce.tokenizer.CurrentToken().Literal
		ce.process(op)
		ce.compileTerm()
		switch token.Symbol(op) {
		case token.PLUS:
			ce.vmwriter.WriteArithmetic(vmwriter.ADD)
		case token.MINUS:
			ce.vmwriter.WriteArithmetic(vmwriter.SUB)
		case token.ASTERISK:
			ce.vmwriter.WriteCall("Math.multiply", 2)
		case token.SLASH:
			ce.vmwriter.WriteCall("Math.divide", 2)
		case token.AND:
			ce.vmwriter.WriteArithmetic(vmwriter.AND)
		case token.PIPE:
			ce.vmwriter.WriteArithmetic(vmwriter.OR)
		case token.LESS_THAN:
			ce.vmwriter.WriteArithmetic(vmwriter.LT)
		case token.GREATER_THAN:
			ce.vmwriter.WriteArithmetic(vmwriter.GT)
		case token.EQUAL:
			ce.vmwriter.WriteArithmetic(vmwriter.EQ)
		}
	}
}

func (ce *CompilationEngine) compileTerm() {
	switch ce.tokenizer.CurrentToken().Type {
	case token.INT_CONST:
		value, err := strconv.Atoi(ce.tokenizer.CurrentToken().Literal)
		if err != nil {
			panic("expected integer constant but got " + ce.tokenizer.CurrentToken().Literal)
		}
		ce.vmwriter.WritePush(vmwriter.CONSTANT, value)
		ce.tokenizer.Advance()
		return
	case token.STRING_CONST:
		ce.vmwriter.WritePush(vmwriter.CONSTANT, len(ce.tokenizer.CurrentToken().Literal))
		ce.vmwriter.WriteCall("String.new", 1)
		for _, c := range ce.tokenizer.CurrentToken().Literal {
			ce.vmwriter.WritePush(vmwriter.CONSTANT, int(c))
			ce.vmwriter.WriteCall("String.appendChar", 2)
		}
		ce.tokenizer.Advance()
		return
	}

	switch ce.tokenizer.CurrentToken().Literal {
	case string(token.TRUE):
		ce.vmwriter.WritePush(vmwriter.CONSTANT, 1)
		ce.vmwriter.WriteArithmetic(vmwriter.NEG)
		ce.tokenizer.Advance()
		return
	case string(token.FALSE), string(token.NULL):
		ce.vmwriter.WritePush(vmwriter.CONSTANT, 0)
		ce.tokenizer.Advance()
		return
	case string(token.THIS):
		ce.vmwriter.WritePush(vmwriter.POINTER, 0)
		ce.tokenizer.Advance()
		return
	case "(":
		ce.process("(")
		ce.compileExpression()
		ce.process(")")
		return
	case "-":
		ce.process("-")
		ce.compileTerm()
		ce.vmwriter.WriteArithmetic(vmwriter.NEG)
		return
	case "~":
		ce.process("~")
		ce.compileTerm()
		ce.vmwriter.WriteArithmetic(vmwriter.NOT)
		return
	}

	name := ce.tokenizer.CurrentToken().Literal
	if ce.tokenizer.CurrentToken().Type != token.IDENTIFIER {
		panic("expected identifier but got " + name)
	}
	ce.tokenizer.Advance()

	switch ce.tokenizer.CurrentToken().Literal {
	case "[":
		ce.process("[")
		ce.compileExpression()
		ce.process("]")
		ce.writePushVariable(name)
		ce.vmwriter.WriteArithmetic(vmwriter.ADD)
		ce.vmwriter.WritePop(vmwriter.POINTER, 1)
		ce.vmwriter.WritePush(vmwriter.THAT, 0)
	case "(":
		ce.process("(")
		ce.compileExpressionList()
		ce.process(")")
	case ".":
		args := 0
		if ce.subroutineST.IndexOf(name) != -1 {
			ce.vmwriter.WritePush(kindSegmentMap[ce.subroutineST.KindOf(name)], ce.subroutineST.IndexOf(name))
			name = ce.subroutineST.TypeOf(name)
			args++
		} else if ce.classST.IndexOf(name) != -1 {
			ce.vmwriter.WritePush(kindSegmentMap[ce.classST.KindOf(name)], ce.classST.IndexOf(name))
			name = ce.classST.TypeOf(name)
			args++
		}
		ce.process(".")
		subroutineName := ce.tokenizer.CurrentToken().Literal
		if ce.tokenizer.CurrentToken().Type != token.IDENTIFIER {
			panic("expected identifier but got " + subroutineName)
		}
		ce.tokenizer.Advance()
		ce.process("(")
		args += ce.compileExpressionList()
		ce.process(")")
		ce.vmwriter.WriteCall(name+"."+subroutineName, args)
	default:
		ce.writePushVariable(name)
	}
}

func (ce *CompilationEngine) compileExpressionList() int {
	count := 0

	for ce.tokenizer.CurrentToken().Literal != ")" {
		ce.compileExpression()
		count++
		if ce.tokenizer.CurrentToken().Literal == "," {
			ce.process(",")
		}
	}
	return count
}

func (ce *CompilationEngine) process(str string) string {
	if ce.tokenizer.CurrentToken().Literal != str {
		panic("expected " + str + " but got " + ce.tokenizer.CurrentToken().Literal)
	}
	ce.tokenizer.Advance()
	return str
}

func (ce *CompilationEngine) processType() string {
	types := []token.Keyword{token.INT, token.CHAR, token.BOOLEAN}
	t := ce.tokenizer.CurrentToken().Literal
	if ce.tokenizer.CurrentToken().Type != token.IDENTIFIER && !slices.Contains(types, token.Keyword(t)) {
		panic("expected type but got " + ce.tokenizer.CurrentToken().Literal)
	}
	ce.tokenizer.Advance()
	return t
}

func (ce *CompilationEngine) writePushVariable(name string) {
	if ce.subroutineST.IndexOf(name) != -1 {
		ce.vmwriter.WritePush(kindSegmentMap[ce.subroutineST.KindOf(name)], ce.subroutineST.IndexOf(name))
	} else if ce.classST.IndexOf(name) != -1 {
		ce.vmwriter.WritePush(kindSegmentMap[ce.classST.KindOf(name)], ce.classST.IndexOf(name))
	} else {
		panic("undefined variable " + name)
	}
}

func (ce *CompilationEngine) writePopVariable(name string) {
	if ce.subroutineST.IndexOf(name) != -1 {
		ce.vmwriter.WritePop(kindSegmentMap[ce.subroutineST.KindOf(name)], ce.subroutineST.IndexOf(name))
	} else if ce.classST.IndexOf(name) != -1 {
		ce.vmwriter.WritePop(kindSegmentMap[ce.classST.KindOf(name)], ce.classST.IndexOf(name))
	} else {
		panic("undefined variable " + name)
	}
}
