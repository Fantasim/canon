// The baked dependent bulk fixture (DECISIONS 392): lists of 8 dependent values, dependent define
// refs and define refs, each filled from arrays, read back.
#include <cstdio>

#include "demo.gen.h"

int main() {
    const demo::Event& e = demo::GetEvent();
    const auto& many = e.GetMany();
    const auto& multis = e.GetMultis();
    const auto& values = e.GetMonstersValues();
    std::printf("many=%zu:%d,%d multis=%zu:%s=%lld,%s=%lld monsters=%zu:%lld,%lld\n", many.size(),
                static_cast<int>(*many[0].AsTrue()), static_cast<int>(*many[1].AsTrue()), multis.size(),
                multis[0].AsB()->c_str(), static_cast<long long>(*multis[0].AsBValue()), multis[1].AsB()->c_str(),
                static_cast<long long>(*multis[1].AsBValue()), e.GetMonstersKeys().size(), static_cast<long long>(values[0]),
                static_cast<long long>(values[1]));
    return 0;
}
