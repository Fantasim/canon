// Package fnsize measures the functions of generated Go, C++ and TypeScript text: for each
// function its name, the lines of its body and its top-level statements.
//
// Go goes through go/parser. C++ and TypeScript go through small tokenizers that skip comments,
// strings, raw strings, template literals and preprocessor lines, then find function bodies by
// the shape of the text before each brace. A function nested in another is part of the outer one.
//
// Not seen: a TypeScript arrow with an expression body (no braces), a function a macro declares.
// An error, never a skipped function: a source the tokenizer cannot read, and a C++ header with
// an elaborated return type, extern "C" before a declaration, alignas or a function-try-block.
package fnsize
