package ir

import "strings"

// cppMacroNames are the names platform headers may define as macros, W8006 (CODEGEN.md §3.5, DECISIONS 388).
var cppMacroNames = append(strings.Fields(`min max near far IN OUT OPTIONAL ERROR DELETE TRUE FALSE VOID CONST
interface small TEXT ABSOLUTE RELATIVE TRANSPARENT OPAQUE CALLBACK WINAPI PASCAL CDECL EXPORT DOMAIN OVERFLOW
UNDERFLOW NO_ERROR IGNORE INFINITE BOOL BYTE WORD DWORD INT UINT LONG major minor unix linux i386
GetObject GetMessage GetClassName GetCommandLine GetCurrentTime GetUserName GetFileAttributes GetProp SetProp
CreateWindow CreateFile DeleteFile CopyFile MoveFile LoadImage LoadString DrawText SendMessage PostMessage
RegisterClass`), strings.Fields(`CHAR_BIT SCHAR_MIN SCHAR_MAX UCHAR_MAX CHAR_MIN CHAR_MAX MB_LEN_MAX SHRT_MIN SHRT_MAX
USHRT_MAX INT_MIN INT_MAX UINT_MAX LONG_MIN LONG_MAX ULONG_MAX LLONG_MIN LLONG_MAX ULLONG_MAX NAME_MAX PATH_MAX
M_E M_LOG2E M_LOG10E M_LN2 M_LN10 M_PI M_PI_2 M_PI_4 M_1_PI M_2_PI M_2_SQRTPI M_SQRT2 M_SQRT1_2`)...)

// cppMacros is cppMacroNames as a set, built once.
var cppMacros = nameSet(cppMacroNames...)

// cppStdMacros is cppStdMacroNames as a set, built once.
var cppStdMacros = nameSet(cppStdMacroNames...)
