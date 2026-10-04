// Calls Get with an id outside StatusId: the container aborts (CODEGEN.md §5.9, DECISIONS 296).
#include "board.gen.h"

int main() {
    return static_cast<int>(demo::board::GetStatuses().Get(static_cast<demo::board::StatusId>(7)).GetWeight());
}
