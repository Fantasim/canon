// A translation unit of meet_main.cpp's binary holding b/, a copy identical to a/ (DECISIONS 436).
#include "b/canon_runtime.h"

canon::EvalErrorHandler SwapSame(canon::EvalErrorHandler handler) {
    return canon::SetEvalErrorHandler(handler);
}
