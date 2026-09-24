// Calls a lookup method with a value that is no member of its enum: the getter aborts
// (log-2026-09-24, gen/cpp review calls), never reads outside its table.
#include "shop.gen.h"

int main() {
    demo::shop::Item item;
    return static_cast<int>(item.Rank(static_cast<demo::shop::Tone>(9), false));
}
