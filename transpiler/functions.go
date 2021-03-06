// This file contains functions for declaring function prototypes, expressions
// that call functions, returning from function and the coordination of
// processing the function bodies.

package transpiler

import (
	"errors"
	"fmt"
	"strings"

	"github.com/elliotchance/c2go/ast"
	"github.com/elliotchance/c2go/program"
	"github.com/elliotchance/c2go/types"

	goast "go/ast"
)

// transpileCallExpr transpiles expressions that calls a function, for example:
//
//     foo("bar")
//
// It returns three arguments; the Go AST expression, the C type (that is
// returned by the function) and any error. If there is an error returned you
// can assume the first two arguments will not contain any useful information.
func transpileCallExpr(n *ast.CallExpr, p *program.Program) (*goast.CallExpr, string, error) {
	// The first child will always contain the name of the function being
	// called.
	firstChild := n.Children[0].(*ast.ImplicitCastExpr).Children[0]
	functionName := firstChild.(*ast.DeclRefExpr).Name

	// Get the function definition from it's name. The case where it is not
	// defined is handled below (we haven't seen the prototype yet).
	functionDef := program.GetFunctionDefinition(functionName)

	if functionDef == nil {
		errorMessage := fmt.Sprintf("unknown function: %s", functionName)
		return nil, "", errors.New(errorMessage)
	}

	if functionDef.Substitution != "" {
		parts := strings.Split(functionDef.Substitution, ".")
		importName := strings.Join(parts[:len(parts)-1], ".")
		p.AddImport(importName)

		parts2 := strings.Split(functionDef.Substitution, "/")
		functionName = parts2[len(parts2)-1]
	}

	args := []goast.Expr{}
	i := 0
	for _, arg := range n.Children[1:] {
		e, eType, err := transpileToExpr(arg, p)
		if err != nil {
			return nil, "unknown2", err
		}

		if i > len(functionDef.ArgumentTypes)-1 {
			// This means the argument is one of the varargs so we don't know
			// what type it needs to be cast to.
			args = append(args, e)
		} else {
			args = append(args, types.CastExpr(p, e, eType, functionDef.ArgumentTypes[i]))
		}

		i++
	}

	return &goast.CallExpr{
		Fun:  goast.NewIdent(functionName),
		Args: args,
	}, functionDef.ReturnType, nil
}

// transpileFunctionDecl transpiles the function prototype.
//
// The function prototype may also have a body. If it does have a body the whole
// function will be transpiled into Go.
//
// If there is no function body we register the function interally (actually
// either way the function is registered internally) but we do not do anything
// becuase Go does not use or have any use for forward declarations of
// functions.
func transpileFunctionDecl(n *ast.FunctionDecl, p *program.Program) error {
	var body *goast.BlockStmt

	// This is set at the start of the function declaration so when the
	// ReturnStmt comes alone it will know what the current function is, and
	// therefore be able to lookup what the real return type should be. I'm sure
	// there is a much better way of doing this.
	p.FunctionName = n.Name

	// Always register the new function. Only from this point onwards will
	// we be allowed to refer to the function.
	if program.GetFunctionDefinition(n.Name) == nil {
		program.AddFunctionDefinition(program.FunctionDefinition{
			Name:          n.Name,
			ReturnType:    getFunctionReturnType(n.Type),
			ArgumentTypes: getFunctionArgumentTypes(n),
			Substitution:  "",
		})
	}

	// If the function has a direct substitute in Go we do not want to
	// output the C definition of it.
	f := program.GetFunctionDefinition(n.Name)
	if f != nil && f.Substitution != "" {
		return nil
	}

	// Test if the function has a body. This is identified by a child node that
	// is a CompoundStmt (since it is not valid to have a function body without
	// curly brackets).
	//
	// It's possible that the last node is the CompoundStmt (after all the
	// parameter declarations) - but I don't know this for certain so we will
	// look at all the children for now.
	hasBody := false
	for _, c := range n.Children {
		if b, ok := c.(*ast.CompoundStmt); ok {
			var err error
			body, err = transpileToBlockStmt(b, p)
			if err != nil {
				return err
			}

			hasBody = true
			break
		}
	}

	// These functions cause us trouble for whatever reason. Some of them might
	// even work now.
	//
	// TODO: Some functions are ignored because they are too much trouble
	// https://github.com/elliotchance/c2go/issues/78
	if n.Name == "__istype" ||
		n.Name == "__isctype" ||
		n.Name == "__wcwidth" ||
		n.Name == "__sputc" ||
		n.Name == "__inline_signbitf" ||
		n.Name == "__inline_signbitd" ||
		n.Name == "__inline_signbitl" {
		return nil
	}

	if hasBody {
		fieldList, err := getFieldList(n, p)
		if err != nil {
			return err
		}

		returnTypes := []*goast.Field{
			&goast.Field{
				Type: goast.NewIdent(types.ResolveType(p, f.ReturnType)),
			},
		}

		// main() function does not have a return type.
		if p.FunctionName == "main" {
			returnTypes = []*goast.Field{}
		}

		p.File.Decls = append(p.File.Decls, &goast.FuncDecl{
			Doc:  nil,
			Recv: nil,
			Name: goast.NewIdent(n.Name),
			Type: &goast.FuncType{
				Params: fieldList,
				Results: &goast.FieldList{
					List: returnTypes,
				},
			},
			Body: body,
		})
	}

	return nil
}

// getFieldList returns the paramaters of a C function as a Go AST FieldList.
func getFieldList(f *ast.FunctionDecl, p *program.Program) (*goast.FieldList, error) {
	// The main() function does not have arguments or a return value.
	if f.Name == "main" {
		return &goast.FieldList{}, nil
	}

	r := []*goast.Field{}
	for _, n := range f.Children {
		if v, ok := n.(*ast.ParmVarDecl); ok {
			r = append(r, &goast.Field{
				Names: []*goast.Ident{goast.NewIdent(v.Name)},
				Type:  goast.NewIdent(types.ResolveType(p, v.Type)),
			})
		}
	}

	return &goast.FieldList{
		List: r,
	}, nil
}

func transpileReturnStmt(n *ast.ReturnStmt, p *program.Program) (goast.Stmt, error) {
	e, eType, err := transpileToExpr(n.Children[0], p)
	if err != nil {
		return nil, err
	}

	f := program.GetFunctionDefinition(p.FunctionName)

	results := []goast.Expr{types.CastExpr(p, e, eType, f.ReturnType)}

	// main() function is not allowed to return a result. Use os.Exit if non-zero
	if p.FunctionName == "main" {
		litExpr, isLiteral := e.(*goast.BasicLit)
		if !isLiteral || (isLiteral && litExpr.Value != "0") {
			p.AddImport("os")
			return &goast.ExprStmt{
				X: &goast.CallExpr{
					Fun:  goast.NewIdent("os.Exit"),
					Args: results,
				},
			}, nil
		}
		results = []goast.Expr{}
	}

	return &goast.ReturnStmt{
		Results: results,
	}, nil
}

func getFunctionReturnType(f string) string {
	// The C type of the function will be the complete prototype, like:
	//
	//     __inline_isfinitef(float) int
	//
	// will have a C type of:
	//
	//     int (float)
	//
	// The arguments will handle themselves, we only care about the return type
	// ('int' in this case)
	returnType := strings.TrimSpace(strings.Split(f, "(")[0])

	if returnType == "" {
		panic(fmt.Sprintf("unable to extract the return type from: %s", f))
	}

	return returnType
}

// getFunctionArgumentTypes returns the C types of the arguments in a function.
func getFunctionArgumentTypes(f *ast.FunctionDecl) []string {
	r := []string{}
	for _, n := range f.Children {
		if v, ok := n.(*ast.ParmVarDecl); ok {
			r = append(r, v.Type)
		}
	}

	return r
}
