package ir

import (
	"slices"
	"strings"
)

// cppMacroNames are the names platform headers may define as macros, W8006 (CODEGEN.md §3.5, DECISIONS 388).
var cppMacroNames = append(strings.Fields(`min max near far IN OUT OPTIONAL ERROR DELETE TRUE FALSE VOID CONST
interface small TEXT ABSOLUTE RELATIVE TRANSPARENT OPAQUE CALLBACK WINAPI PASCAL CDECL EXPORT DOMAIN OVERFLOW
UNDERFLOW NO_ERROR IGNORE INFINITE BOOL BYTE WORD DWORD INT UINT LONG major minor unix linux i386
GetObject GetMessage GetClassName GetCommandLine GetCurrentTime GetUserName GetFileAttributes GetProp SetProp
CreateWindow CreateFile DeleteFile CopyFile MoveFile LoadImage LoadString DrawText SendMessage PostMessage
RegisterClass`), strings.Fields(`CHAR_BIT SCHAR_MIN SCHAR_MAX UCHAR_MAX CHAR_MIN CHAR_MAX MB_LEN_MAX SHRT_MIN SHRT_MAX
USHRT_MAX INT_MIN INT_MAX UINT_MAX LONG_MIN LONG_MAX ULONG_MAX LLONG_MIN LLONG_MAX ULLONG_MAX NAME_MAX PATH_MAX
M_E M_LOG2E M_LOG10E M_LN2 M_LN10 M_PI M_PI_2 M_PI_4 M_1_PI M_2_PI M_2_SQRTPI M_SQRT2 M_SQRT1_2`)...)

// cppStdMacroTail, sorted: the last macros of cppStdMacroNames, kept apart so constants.go stays within its size (CODEGEN.md §3.4).
var cppStdMacroTail = strings.Fields(`
LDBL_MIN_EXP LDBL_TRUE_MIN L_tmpnam MATH_ERREXCEPT MATH_ERRNO MB_CUR_MAX NAN NULL PTRDIFF_MAX
PTRDIFF_MIN RAND_MAX SEEK_CUR SEEK_END SEEK_SET SIG_ATOMIC_MAX SIG_ATOMIC_MIN SIZE_MAX TMP_MAX
UINT16_C UINT16_MAX UINT32_C UINT32_MAX UINT64_C UINT64_MAX UINT8_C UINT8_MAX UINTMAX_C UINTMAX_MAX
UINTPTR_MAX UINT_FAST16_MAX UINT_FAST32_MAX UINT_FAST64_MAX UINT_FAST8_MAX UINT_LEAST16_MAX
UINT_LEAST32_MAX UINT_LEAST64_MAX UINT_LEAST8_MAX WCHAR_MAX WCHAR_MIN WINT_MAX WINT_MIN _IOFBF
_IOLBF _IONBF errno math_errhandling offsetof stderr stdin stdout`)

// cppStdMacroNames, sorted: cppStdMacroHead then cppStdMacroTail (CODEGEN.md §3.4).
var cppStdMacroNames = append(slices.Clone(cppStdMacroHead), cppStdMacroTail...)

// cppMacros is cppMacroNames as a set, built once.
var cppMacros = nameSet(cppMacroNames...)

// cppStdMacros is cppStdMacroNames as a set, built once.
var cppStdMacros = nameSet(cppStdMacroNames...)
