package program

import (
	"regexp"
	"strings"
)

// FunctionDefinition contains the prototype definition for a function.
type FunctionDefinition struct {
	// The name of the function, like "printf".
	Name string

	// The C return type, like "int".
	ReturnType string

	// The C argument types, like ["bool", "int"]. There is currently no way
	// to represent a varargs.
	ArgumentTypes []string

	// If this is not empty then this function name should be used instead
	// of the Name. Many low level functions have an exact match with a Go
	// function. For example, "sin()".
	Substitution string
}

var functionDefinitions map[string]FunctionDefinition

var builtInFunctionDefinitionsHaveBeenLoaded = false

// Each of the predefined function have a syntax that allows them to be easy to
// read (and maintain). For example:
//
//     double __builtin_fabs(double) -> darwin.Fabs
//
// Declares the prototype of __builtin_fabs (a low level function implemented
// only on Mac) with a specific substitution provided. This means that it should
// replace any instance of __builtin_fabs with:
//
//     github.com/elliotchance/c2go/darwin.Fabs
//
// THe substitution is optional.
var builtInFunctionDefinitions = []string{
	// darwin/assert.h
	"int __builtin_expect(int, int) -> darwin.BuiltinExpect",
	"bool __assert_rtn(const char*, const char*, int, const char*) -> darwin.AssertRtn",

	// darwin/ctype.h
	"uint32 __istype(__darwin_ct_rune_t, uint32) -> darwin.IsType",
	"__darwin_ct_rune_t __isctype(__darwin_ct_rune_t, uint32) -> darwin.IsCType",
	"__darwin_ct_rune_t __tolower(__darwin_ct_rune_t) -> darwin.ToLower",
	"__darwin_ct_rune_t __toupper(__darwin_ct_rune_t) -> darwin.ToUpper",
	"uint32 __maskrune(__darwin_ct_rune_t, uint32) -> darwin.MaskRune",

	// linux/ctype.h
	"const unsigned short int** __ctype_b_loc() -> linux.CtypeLoc",
	"int tolower(int) -> linux.ToLower",
	"int toupper(int) -> linux.ToUpper",

	// darwin/math.h
	"double __builtin_fabs(double) -> darwin.Fabs",
	"float __builtin_fabsf(float) -> darwin.Fabsf",
	"double __builtin_fabsl(double) -> darwin.Fabsl",
	"double __builtin_inf() -> darwin.Inf",
	"float __builtin_inff() -> darwin.Inff",
	"double __builtin_infl() -> darwin.Infl",
	"Double2 __sincospi_stret(double) -> darwin.SincospiStret",
	"Float2 __sincospif_stret(float) -> darwin.SincospifStret",
	"Double2 __sincos_stret(double) -> darwin.SincosStret",
	"Float2 __sincosf_stret(float) -> darwin.SincosfStret",

	// linux/assert.h
	"bool __assert_fail(const char*, const char*, unsigned int, const char*) -> linux.AssertFail",

	// math.h
	"double acos(double) -> math.Acos",
	"double asin(double) -> math.Asin",
	"double atan(double) -> math.Atan",
	"double atan2(double) -> math.Atan2",
	"double ceil(double) -> math.Ceil",
	"double cos(double) -> math.Cos",
	"double cosh(double) -> math.Cosh",
	"double exp(double) -> math.Exp",
	"double fabs(double) -> math.Abs",
	"double floor(double) -> math.Floor",
	"double fmod(double) -> math.Mod",
	"double ldexp(double) -> math.Ldexp",
	"double log(double) -> math.Log",
	"double log10(double) -> math.Log10",
	"double pow(double) -> math.Pow",
	"double sin(double) -> math.Sin",
	"double sinh(double) -> math.Sinh",
	"double sqrt(double) -> math.Sqrt",
	"double tan(double) -> math.Tan",
	"double tanh(double) -> math.Tanh",

	// stdio.h
	"int printf() -> fmt.Printf",
	"int scanf() -> fmt.Scanf",
	"int putchar(int) -> darwin.Putchar",
	"int puts(const char *) -> fmt.Println",
	"FILE* fopen(const char *, const char *) -> noarch.Fopen",
	"int fclose(int) -> noarch.Fclose",

	// stdlib.h
	"int atoi(const char*) -> noarch.Atoi",
	"long strtol(const char *, char **, int) -> noarch.Strtol",

	// I'm not sure which header file these comes from?
	"uint32 __builtin_bswap32(uint32) -> darwin.BSwap32",
	"uint64 __builtin_bswap64(uint64) -> darwin.BSwap64",
}

// getFunctionDefinition will return nil if the function does not exist (is not
// registered).
func GetFunctionDefinition(functionName string) *FunctionDefinition {
	loadFunctionDefinitions()

	if f, ok := functionDefinitions[functionName]; ok {
		return &f
	}

	return nil
}

// addFunctionDefinition registers a function definition. If the definition
// already exists it will be replaced.
func AddFunctionDefinition(f FunctionDefinition) {
	loadFunctionDefinitions()

	functionDefinitions[f.Name] = f
}

func loadFunctionDefinitions() {
	if builtInFunctionDefinitionsHaveBeenLoaded {
		return
	}

	functionDefinitions = map[string]FunctionDefinition{}
	builtInFunctionDefinitionsHaveBeenLoaded = true

	for _, f := range builtInFunctionDefinitions {
		match := regexp.MustCompile(`^(.+) (.+)\((.*)\)( -> .*)?$`).
			FindStringSubmatch(f)

		// Unpack argument types.
		argumentTypes := strings.Split(match[3], ",")
		for i := range argumentTypes {
			argumentTypes[i] = strings.TrimSpace(argumentTypes[i])
		}
		if len(argumentTypes) == 1 && argumentTypes[0] == "" {
			argumentTypes = []string{}
		}

		// Substitution rules.
		substitution := match[4]
		if substitution != "" {
			substitution = strings.TrimLeft(substitution, " ->")
		}
		if strings.HasPrefix(substitution, "darwin.") ||
			strings.HasPrefix(substitution, "linux.") ||
			strings.HasPrefix(substitution, "noarch.") {
			substitution = "github.com/elliotchance/c2go/" + substitution
		}

		AddFunctionDefinition(FunctionDefinition{
			Name:          match[2],
			ReturnType:    match[1],
			ArgumentTypes: argumentTypes,
			Substitution:  substitution,
		})
	}
}
