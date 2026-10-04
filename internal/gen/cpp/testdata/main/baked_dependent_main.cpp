// The baked dependent fixture: each dependent value in the branch its discriminant selects (CODEGEN.md §5.6).
#include <cstdio>

#include "demo.gen.h"

int main() {
    const demo::Event& e = demo::GetEvent();
    const demo::Multi* m = e.GetMulti();
    std::printf("payload=%d:%d multi=%d:%s=%lld deep=%d:%d many=%d,%d:%d,%d\n",
                static_cast<int>(e.GetPayload().GetBranch()), static_cast<int>(*e.GetPayload().AsTrue()),
                static_cast<int>(m->GetBranch()), m->AsB()->c_str(), static_cast<long long>(*m->AsBValue()),
                static_cast<int>(e.GetDeep().GetBranch()), *e.GetDeep().AsA() ? 1 : 0,
                static_cast<int>(e.GetMany()[0].GetBranch()), static_cast<int>(e.GetMany()[1].GetBranch()),
                static_cast<int>(*e.GetMany()[0].AsTrue()), static_cast<int>(*e.GetMany()[1].AsTrue()));
    return 0;
}
