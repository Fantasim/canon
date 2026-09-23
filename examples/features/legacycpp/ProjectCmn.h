// Fixture: a minimal stand-in for the server's hand-written ItemProp
// (Engine/Core/Project/ProjectCmn.h), for the legacycpp example (IMPLEMENTATION-PLAN.md M6).
// The five members legacycpp.canon maps keep their real names and types; szIcon is a member
// the example does not map, so generated code must leave it value-initialised.
#ifndef PROJECTCMN_H
#define PROJECTCMN_H

#include <cstdint>

struct ItemProp {
    std::uint32_t dwID;
    std::uint32_t dwItemKind1;
    std::uint32_t dwItemLV;
    std::uint32_t dwPackMax;
    bool bPermanence;
    char szIcon[64];
};

#endif  // PROJECTCMN_H
