// A translation unit of meet_main.cpp's binary holding other/, a copy of another stamp (DECISIONS 436).
#include "other/canon_runtime.h"

canon::EvalErrorHandler SwapOther(canon::EvalErrorHandler handler) {
    return canon::SetEvalErrorHandler(handler);
}
